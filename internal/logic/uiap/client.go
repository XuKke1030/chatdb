package uiap

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"ai-chat-sql/internal/logic/integration"
)

type Config struct {
	Enabled                bool
	BaseUrl                string
	ClientId               string
	ClientSecret           string
	TokenPath              string
	UserInfoPath           string
	PermissionPath         string
	BatchPermissionPath    string
	TimeoutSeconds         int
	TokenExpireSkewSeconds int
}

type Client struct {
	cfg        Config
	jsonClient *integration.JSONClient
	mu         sync.Mutex
	token      string
	expiresAt  time.Time
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

type UserInfo struct {
	UserId      string   `json:"sub"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	OrgCode     string   `json:"orgCode"`
	OrgName     string   `json:"orgName"`
	Roles       []string `json:"roles"`
	Enabled     bool     `json:"enabled"`
	UpdateTime  string   `json:"updateTime"`
}

type PermissionQuery struct {
	RequestId                   string `json:"requestId"`
	UserId                      string `json:"userId"`
	Username                    string `json:"username"`
	IncludeTopicPermissions     bool   `json:"includeTopicPermissions"`
	IncludeKnowledgePermissions bool   `json:"includeKnowledgePermissions"`
	IncludeDocumentPermissions  bool   `json:"includeDocumentPermissions"`
}

type PermissionResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	RequestId string         `json:"requestId"`
	Data      PermissionData `json:"data"`
}

type PermissionData struct {
	UserId               string                `json:"userId"`
	Enabled              bool                  `json:"enabled"`
	TopicPermissions     []TopicPermission     `json:"topicPermissions"`
	KnowledgePermissions []KnowledgePermission `json:"knowledgePermissions"`
	DocumentPermissions  []DocumentPermission  `json:"documentPermissions"`
	PermissionVersion    int                   `json:"permissionVersion"`
	PermissionHash       string                `json:"permissionHash"`
	UpdateTime           string                `json:"updateTime"`
}

type TopicPermission struct {
	Topic   string `json:"topic"`
	Enabled bool   `json:"enabled"`
}

type KnowledgePermission struct {
	KnowledgeCode string `json:"knowledgeCode"`
	Enabled       bool   `json:"enabled"`
}

type DocumentPermission struct {
	DocumentId string `json:"documentId"`
	Enabled    bool   `json:"enabled"`
}

type BatchPermissionQuery struct {
	RequestId    string `json:"requestId"`
	UpdatedAfter string `json:"updatedAfter"`
	Page         int    `json:"page"`
	PageSize     int    `json:"pageSize"`
}

type BatchPermissionResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"requestId"`
	Data      struct {
		List     []PermissionData `json:"list"`
		Page     int              `json:"page"`
		PageSize int              `json:"pageSize"`
		Total    int              `json:"total"`
		HasMore  bool             `json:"hasMore"`
	} `json:"data"`
}

func NewClient(cfg Config) *Client {
	cfg = withDefaults(cfg)
	httpClient := integration.NewJSONClient(cfg.BaseUrl, cfg.TimeoutSeconds)
	httpClient.ClientId = cfg.ClientId
	httpClient.Secret = cfg.ClientSecret
	httpClient.SignRequests = true
	return &Client{cfg: cfg, jsonClient: httpClient}
}

func (c *Client) ExchangeCode(ctx context.Context, code string, redirectURI string) (*TokenResponse, error) {
	req := map[string]any{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  redirectURI,
		"client_id":     c.cfg.ClientId,
		"client_secret": c.cfg.ClientSecret,
	}
	var resp TokenResponse
	if err := c.jsonClient.PostJSON(ctx, c.cfg.TokenPath, req, &resp, ""); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return nil, fmt.Errorf("uiap token response missing access_token")
	}
	return &resp, nil
}

func (c *Client) UserInfo(ctx context.Context, token string) (*UserInfo, error) {
	var resp UserInfo
	if err := c.jsonClient.GetJSON(ctx, c.cfg.UserInfoPath, &resp, token); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.UserId) == "" && strings.TrimSpace(resp.Username) == "" {
		return nil, fmt.Errorf("uiap userinfo response missing user id")
	}
	return &resp, nil
}

func (c *Client) Permissions(ctx context.Context, token string, query PermissionQuery) (*PermissionData, error) {
	if query.RequestId == "" {
		query.RequestId = newRequestId()
	}
	var resp PermissionResponse
	if err := c.jsonClient.PostJSON(ctx, c.cfg.PermissionPath, query, &resp, token); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("uiap permission failed: code=%d message=%s", resp.Code, resp.Message)
	}
	return &resp.Data, nil
}

func (c *Client) BatchPermissions(ctx context.Context, updatedAfter string, page int, pageSize int) (*BatchPermissionResponse, error) {
	token, err := c.ServiceToken(ctx)
	if err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 500
	}
	req := BatchPermissionQuery{
		RequestId:    newRequestId(),
		UpdatedAfter: updatedAfter,
		Page:         page,
		PageSize:     pageSize,
	}
	var resp BatchPermissionResponse
	if err = c.jsonClient.PostJSON(ctx, c.cfg.BatchPermissionPath, req, &resp, token); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("uiap batch permission failed: code=%d message=%s", resp.Code, resp.Message)
	}
	return &resp, nil
}

func (c *Client) ServiceToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiresAt) {
		return c.token, nil
	}
	req := map[string]any{
		"grant_type":    "client_credentials",
		"client_id":     c.cfg.ClientId,
		"client_secret": c.cfg.ClientSecret,
	}
	var resp TokenResponse
	if err := c.jsonClient.PostJSON(ctx, c.cfg.TokenPath, req, &resp, ""); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return "", fmt.Errorf("uiap service token response missing access_token")
	}
	skew := c.cfg.TokenExpireSkewSeconds
	if skew <= 0 {
		skew = 120
	}
	expiresIn := resp.ExpiresIn
	if expiresIn <= skew {
		expiresIn = skew + 60
	}
	c.token = resp.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(expiresIn-skew) * time.Second)
	return c.token, nil
}

func withDefaults(cfg Config) Config {
	if cfg.TokenPath == "" {
		cfg.TokenPath = "/oauth/token"
	}
	if cfg.UserInfoPath == "" {
		cfg.UserInfoPath = "/oauth/userinfo"
	}
	if cfg.PermissionPath == "" {
		cfg.PermissionPath = "/openapi/chatdb/permissions/query"
	}
	if cfg.BatchPermissionPath == "" {
		cfg.BatchPermissionPath = "/openapi/chatdb/permissions/batch-query"
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 5
	}
	return cfg
}

func newRequestId() string {
	id, err := integration.RandomHex(16)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return id
}
