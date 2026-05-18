package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type JSONClient struct {
	BaseURL        string
	ClientId       string
	Secret         string
	HTTPClient     *http.Client
	SignRequests   bool
	DefaultHeaders map[string]string
}

func NewJSONClient(baseURL string, timeoutSeconds int) *JSONClient {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	return &JSONClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		DefaultHeaders: make(map[string]string),
	}
}

func (c *JSONClient) PostJSON(ctx context.Context, path string, request any, response any, bearerToken string) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, body, bearerToken)
	if err != nil {
		return err
	}
	return c.do(req, response)
}

func (c *JSONClient) GetJSON(ctx context.Context, path string, response any, bearerToken string) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil, bearerToken)
	if err != nil {
		return err
	}
	return c.do(req, response)
}

func (c *JSONClient) newRequest(ctx context.Context, method, path string, body []byte, bearerToken string) (*http.Request, error) {
	target, err := joinURL(c.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if c.ClientId != "" {
		req.Header.Set("X-Client-Id", c.ClientId)
		req.Header.Set("X-App-Key", c.ClientId)
	}
	for key, value := range c.DefaultHeaders {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	if c.SignRequests && c.Secret != "" {
		pathWithQuery := req.URL.EscapedPath()
		if req.URL.RawQuery != "" {
			pathWithQuery += "?" + req.URL.RawQuery
		}
		headers, err := NewSignedHeaders(method, pathWithQuery, body, c.Secret)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Request-Id", headers.RequestId)
		req.Header.Set("X-Timestamp", headers.Timestamp)
		req.Header.Set("X-Nonce", headers.Nonce)
		req.Header.Set("X-Signature", headers.Signature)
	}
	return req, nil
}

func (c *JSONClient) do(req *http.Request, response any) error {
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if response == nil || len(data) == 0 {
		return nil
	}
	if err = json.Unmarshal(data, response); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func joinURL(baseURL string, path string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("baseURL is required")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(rel).String(), nil
}
