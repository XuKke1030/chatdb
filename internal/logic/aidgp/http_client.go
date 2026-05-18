package aidgp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"ai-chat-sql/internal/logic/integration"
)

type HTTPClient struct {
	cfg        Config
	jsonClient *integration.JSONClient
	mu         sync.Mutex
	token      string
	expiresAt  time.Time
}

type tokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken string `json:"accessToken"`
		TokenType   string `json:"tokenType"`
		ExpiresIn   int    `json:"expiresIn"`
	} `json:"data"`
}

type pageEnvelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"requestId"`
	Data      struct {
		List     []map[string]any `json:"list"`
		Page     int              `json:"page"`
		PageSize int              `json:"pageSize"`
		Total    int              `json:"total"`
		HasMore  bool             `json:"hasMore"`
	} `json:"data"`
}

func NewHTTPClient(cfg Config) *HTTPClient {
	cfg = withDefaults(cfg)
	client := integration.NewJSONClient(cfg.BaseUrl, cfg.TimeoutSeconds)
	client.ClientId = cfg.AppKey
	client.Secret = cfg.AppSecret
	client.SignRequests = true
	return &HTTPClient{cfg: cfg, jsonClient: client}
}

func (c *HTTPClient) SyncKnowledgeBases(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return c.syncList(ctx, SyncKnowledgeBases, c.cfg.KnowledgeBasesPath, map[string]any{
		"requestId": newRequestId(),
		"page":      1,
		"pageSize":  500,
	})
}

func (c *HTTPClient) TestConnection(ctx context.Context) error {
	_, err := c.GetToken(ctx)
	return err
}

func (c *HTTPClient) SyncDocuments(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return c.syncList(ctx, SyncDocuments, c.cfg.DocumentsPath, map[string]any{
		"requestId":     newRequestId(),
		"knowledgeCode": strings.TrimSpace(scope.KnowledgeCode),
		"page":          1,
		"pageSize":      500,
	})
}

func (c *HTTPClient) SyncPermissions(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return c.syncList(ctx, SyncPermissions, c.cfg.KnowledgePermissionsPath, map[string]any{
		"requestId":     newRequestId(),
		"knowledgeCode": strings.TrimSpace(scope.KnowledgeCode),
		"page":          1,
		"pageSize":      500,
	})
}

func (c *HTTPClient) SyncGridData(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return c.syncList(ctx, SyncGridData, c.cfg.GridQueryPath, map[string]any{
		"requestId": newRequestId(),
		"page":      1,
		"pageSize":  500,
	})
}

func (c *HTTPClient) SyncTrafficData(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return c.syncList(ctx, SyncTrafficData, c.cfg.TrafficQueryPath, map[string]any{
		"requestId": newRequestId(),
		"page":      1,
		"pageSize":  500,
	})
}

func (c *HTTPClient) SyncPopulationData(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return c.syncList(ctx, SyncPopulationData, c.cfg.PopulationQueryPath, map[string]any{
		"requestId": newRequestId(),
		"page":      1,
		"pageSize":  500,
	})
}

func (c *HTTPClient) syncList(ctx context.Context, syncType string, path string, req map[string]any) (SyncResult, error) {
	token, err := c.GetToken(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	logs := make([]SyncLog, 0)
	successCount := 0
	failureCount := 0
	skippedCount := 0
	page := intFieldAny(req, "page")
	if page <= 0 {
		page = 1
	}
	for {
		req["page"] = page
		var resp pageEnvelope
		if err = c.postWithRetry(ctx, path, req, &resp, token); err != nil {
			return SyncResult{}, err
		}
		if resp.Code != 0 {
			return SyncResult{}, fmt.Errorf("aidgp %s failed: code=%d message=%s", syncType, resp.Code, resp.Message)
		}
		for i, item := range resp.Data.List {
			log := persistRecord(ctx, syncType, item, (page-1)*len(resp.Data.List)+i+1)
			logs = append(logs, log)
			switch log.Status {
			case "success":
				successCount++
			case "skipped":
				skippedCount++
			default:
				failureCount++
			}
		}
		if !resp.Data.HasMore || len(resp.Data.List) == 0 {
			break
		}
		page++
	}
	if syncType == SyncTrafficData {
		log := refreshTrafficAggregates(ctx)
		logs = append(logs, log)
		if log.Status == "success" {
			successCount++
		} else {
			failureCount++
		}
	}
	if syncType == SyncGridData {
		log := refreshGridMetrics(ctx)
		logs = append(logs, log)
		if log.Status == "success" {
			successCount++
		} else {
			failureCount++
		}
	}
	return SyncResult{
		Message:      fmt.Sprintf("AIDGP %s synced: success=%d failure=%d skipped=%d", syncType, successCount, failureCount, skippedCount),
		SuccessCount: successCount,
		FailureCount: failureCount,
		SkippedCount: skippedCount,
		Logs:         logs,
	}, nil
}

func (c *HTTPClient) GetToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiresAt) {
		return c.token, nil
	}
	req := map[string]any{
		"appKey":    c.cfg.AppKey,
		"appSecret": c.cfg.AppSecret,
		"grantType": "client_credentials",
	}
	var resp tokenResponse
	if err := c.jsonClient.PostJSON(ctx, c.cfg.TokenPath, req, &resp, ""); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("aidgp token failed: code=%d message=%s", resp.Code, resp.Message)
	}
	if strings.TrimSpace(resp.Data.AccessToken) == "" {
		return "", fmt.Errorf("aidgp token response missing accessToken")
	}
	skew := c.cfg.TokenExpireSkewSeconds
	if skew <= 0 {
		skew = 120
	}
	expiresIn := resp.Data.ExpiresIn
	if expiresIn <= skew {
		expiresIn = skew + 60
	}
	c.token = resp.Data.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(expiresIn-skew) * time.Second)
	return c.token, nil
}

func (c *HTTPClient) postWithRetry(ctx context.Context, path string, req any, resp any, token string) error {
	retries := c.cfg.RetryTimes
	if retries < 0 {
		retries = 0
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		lastErr = c.jsonClient.PostJSON(ctx, path, req, resp, token)
		if lastErr == nil {
			return nil
		}
		if attempt < retries {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}
	}
	return lastErr
}

func withDefaults(cfg Config) Config {
	if cfg.TokenPath == "" {
		cfg.TokenPath = "/openapi/oauth/token"
	}
	if cfg.TrafficQueryPath == "" {
		cfg.TrafficQueryPath = "/openapi/chatdb/traffic/query"
	}
	if cfg.PopulationQueryPath == "" {
		cfg.PopulationQueryPath = "/openapi/chatdb/population/query"
	}
	if cfg.GridQueryPath == "" {
		cfg.GridQueryPath = "/openapi/chatdb/grid/query"
	}
	if cfg.KnowledgeBasesPath == "" {
		cfg.KnowledgeBasesPath = "/openapi/chatdb/knowledge-bases"
	}
	if cfg.DocumentsPath == "" {
		cfg.DocumentsPath = "/openapi/chatdb/documents"
	}
	if cfg.DocumentSegmentsPath == "" {
		cfg.DocumentSegmentsPath = "/openapi/chatdb/document-segments"
	}
	if cfg.KnowledgePermissionsPath == "" {
		cfg.KnowledgePermissionsPath = "/openapi/chatdb/knowledge-permissions"
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 10
	}
	if cfg.RetryTimes < 0 {
		cfg.RetryTimes = 0
	}
	return cfg
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func newRequestId() string {
	id, err := integration.RandomHex(16)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return id
}
