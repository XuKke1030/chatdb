package aidgp

import (
	"context"
	"fmt"
	"strings"
)

const (
	ProviderMock  = "mock"
	ProviderAidgp = "aidgp"

	SyncKnowledgeBases = "knowledge_bases"
	SyncDocuments      = "documents"
	SyncPermissions    = "permissions"
	SyncGridData       = "grid_data"
	SyncTrafficData    = "traffic_data"
	SyncPopulationData = "population_data"
)

type SyncScope struct {
	KnowledgeCode string
}

type SyncLog struct {
	ExternalId string
	LocalId    string
	Action     string
	Status     string
	Message    string
}

type SyncResult struct {
	Message      string
	SuccessCount int
	FailureCount int
	SkippedCount int
	Logs         []SyncLog
}

type Client interface {
	SyncKnowledgeBases(ctx context.Context, scope SyncScope) (SyncResult, error)
	SyncDocuments(ctx context.Context, scope SyncScope) (SyncResult, error)
	SyncPermissions(ctx context.Context, scope SyncScope) (SyncResult, error)
	SyncGridData(ctx context.Context, scope SyncScope) (SyncResult, error)
	SyncTrafficData(ctx context.Context, scope SyncScope) (SyncResult, error)
	SyncPopulationData(ctx context.Context, scope SyncScope) (SyncResult, error)
}

type Config struct {
	Provider  string
	BaseUrl   string
	AppKey    string
	AppSecret string
}

func NewClient(cfg Config) Client {
	// Until the real AIDGP contract is available, the concrete AIDGP client
	// intentionally falls back to deterministic mock data. The controller and
	// task/log persistence are already real, so swapping this implementation
	// later will not change the upper layers.
	return &MockClient{Provider: normalizeProvider(cfg.Provider)}
}

func normalizeProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return ProviderMock
	}
	return provider
}

type MockClient struct {
	Provider string
}

func (c *MockClient) SyncKnowledgeBases(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return SyncResult{
		Message:      "AIDGP Mock 知识库同步完成，当前仅写入任务日志，不修改正式知识库数据。",
		SuccessCount: 2,
		Logs: []SyncLog{
			{ExternalId: "aidgp-kb-policy", LocalId: "policy", Action: "update", Status: "success", Message: "模拟同步政策制度库"},
			{ExternalId: "aidgp-kb-manual", LocalId: "manual", Action: "update", Status: "success", Message: "模拟同步业务手册库"},
		},
	}, nil
}

func (c *MockClient) SyncDocuments(ctx context.Context, scope SyncScope) (SyncResult, error) {
	code := strings.TrimSpace(scope.KnowledgeCode)
	if code == "" {
		code = "all"
	}
	return SyncResult{
		Message:      fmt.Sprintf("AIDGP Mock 文档同步完成，范围：%s。", code),
		SuccessCount: 2,
		SkippedCount: 1,
		Logs: []SyncLog{
			{ExternalId: "aidgp-doc-001", LocalId: code + ":doc-001", Action: "update", Status: "success", Message: "模拟同步文档元数据"},
			{ExternalId: "aidgp-doc-002", LocalId: code + ":doc-002", Action: "update", Status: "success", Message: "模拟同步文档正文"},
			{ExternalId: "aidgp-doc-003", LocalId: code + ":doc-003", Action: "skip", Status: "skipped", Message: "模拟跳过未变化文档"},
		},
	}, nil
}

func (c *MockClient) SyncPermissions(ctx context.Context, scope SyncScope) (SyncResult, error) {
	code := strings.TrimSpace(scope.KnowledgeCode)
	if code == "" {
		code = "all"
	}
	return SyncResult{
		Message:      fmt.Sprintf("AIDGP Mock 权限同步完成，范围：%s。", code),
		SuccessCount: 1,
		Logs: []SyncLog{
			{ExternalId: "aidgp-perm-public", LocalId: code + ":public", Action: "update", Status: "success", Message: "模拟同步默认可见权限"},
		},
	}, nil
}

func (c *MockClient) SyncGridData(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return SyncResult{
		Message:      "AIDGP Mock 网格数据同步完成，真实字段映射待 AIDGP 接口确认。",
		SuccessCount: 1,
		Logs: []SyncLog{
			{ExternalId: "aidgp-grid-batch-demo", LocalId: "grid:mock-batch", Action: "update", Status: "success", Message: "模拟同步网格批次"},
		},
	}, nil
}

func (c *MockClient) SyncTrafficData(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return SyncResult{
		Message:      "AIDGP Mock 车流数据同步完成，真实接入后将转换到本地车流聚合模型。",
		SuccessCount: 1,
		Logs: []SyncLog{
			{ExternalId: "aidgp-traffic-batch-demo", LocalId: "traffic:mock-batch", Action: "update", Status: "success", Message: "模拟同步车流批次"},
		},
	}, nil
}

func (c *MockClient) SyncPopulationData(ctx context.Context, scope SyncScope) (SyncResult, error) {
	return SyncResult{
		Message:      "人流数据格式尚未配置，AIDGP Mock 人流同步已跳过，不生成任何人流数字。",
		SkippedCount: 1,
		Logs: []SyncLog{
			{ExternalId: "aidgp-population-contract", LocalId: "population:pending-contract", Action: "skip", Status: "skipped", Message: "等待人流字段格式与同步口径"},
		},
	}, nil
}
