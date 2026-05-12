package v1

import (
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

type KnowledgeBaseItem struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Enabled        bool   `json:"enabled"`
	SourceProvider string `json:"sourceProvider"`
	ExternalId     string `json:"externalId,omitempty"`
	UpdateTime     int    `json:"updateTime"`
}

type KnowledgeBasesReq struct {
	g.Meta `path:"/qa/knowledge-bases" method:"get" tags:"V1/问答" sm:"问答知识库列表" dc:"获取当前用户有权限访问的问答知识库"`
}

type KnowledgeBasesRes struct {
	List []KnowledgeBaseItem `json:"list"`
}

type RetrieveReq struct {
	g.Meta        `path:"/qa/retrieve" method:"post" tags:"V1/问答" sm:"RAG检索" dc:"按问题和知识库范围检索相关文档片段"`
	Question      string `json:"question" v:"required#问题不能为空"`
	KnowledgeCode string `json:"knowledgeCode" dc:"知识库编码，不传则检索当前用户可访问的全部知识库"`
	TopK          int    `json:"topK" dc:"返回条数，默认5，最大20"`
}

type RetrieveItem struct {
	KnowledgeCode string  `json:"knowledgeCode"`
	DocumentId    int64   `json:"documentId"`
	DocumentTitle string  `json:"documentTitle"`
	FileName      string  `json:"fileName"`
	SegmentId     int64   `json:"segmentId"`
	Content       string  `json:"content"`
	Page          int     `json:"page"`
	Anchor        string  `json:"anchor"`
	Score         float64 `json:"score"`
}

type RetrieveRes struct {
	List []RetrieveItem `json:"list"`
}

type ChatReq struct {
	g.Meta        `path:"/qa/chats" method:"post" tags:"V1/问答" sm:"问答流式输出" dc:"基于知识库召回结果进行问答并通过SSE流式返回"`
	Message       string                  `json:"message" v:"required#问题不能为空"`
	KnowledgeCode string                  `json:"knowledgeCode" dc:"知识库编码，不传则在当前用户可访问范围内检索"`
	SessionId     string                  `json:"sessionId" dc:"会话ID，不传则自动创建"`
	History       []model.ChatHistoryItem `json:"history" dc:"前端附带的临时上下文，服务端会合并已存储历史"`
	TopK          int                     `json:"topK" dc:"召回条数，默认5，最大20"`
	Ai            string                  `json:"ai" dc:"AI供应商，默认deepseek"`
	Model         string                  `json:"model" dc:"模型名称，默认deepseek-chat"`
	DeepThinking  bool                    `json:"deepThinking" dc:"是否展示深度思考过程"`
	WebSearch     bool                    `json:"webSearch" dc:"是否启用联网搜索补充"`
}

type ChatRes struct {
}

type CitationDetailReq struct {
	g.Meta     `path:"/qa/citations/{citationId}" method:"get" tags:"V1/问答" sm:"引用详情" dc:"获取引用来源详情并返回可定位的文档段落信息"`
	CitationId int64 `json:"citationId" in:"path" v:"required#引用ID不能为空"`
}

type CitationDetailRes struct {
	CitationId    int64  `json:"citationId"`
	SessionId     string `json:"sessionId"`
	MessageId     int64  `json:"messageId"`
	DocumentId    int64  `json:"documentId"`
	DocumentTitle string `json:"documentTitle"`
	FileName      string `json:"fileName"`
	FileType      string `json:"fileType"`
	KnowledgeCode string `json:"knowledgeCode"`
	SegmentId     int64  `json:"segmentId"`
	SegmentIndex  int    `json:"segmentIndex"`
	Page          int    `json:"page"`
	Anchor        string `json:"anchor"`
	PreviewText   string `json:"previewText"`
	Content       string `json:"content"`
}

type DocumentSegmentItem struct {
	SegmentId    int64  `json:"segmentId"`
	SegmentIndex int    `json:"segmentIndex"`
	Content      string `json:"content"`
	Page         int    `json:"page"`
	Anchor       string `json:"anchor"`
}

type DocumentViewReq struct {
	g.Meta     `path:"/qa/documents/{id}/view" method:"get" tags:"V1/问答" sm:"文档查看" dc:"查看问答端文档内容，返回段落锚点供前端定位"`
	DocumentId int64 `json:"id" in:"path" v:"required#文档ID不能为空"`
}

type DocumentViewRes struct {
	DocumentId     int64                 `json:"documentId"`
	KnowledgeCode  string                `json:"knowledgeCode"`
	Title          string                `json:"title"`
	FileName       string                `json:"fileName"`
	FileType       string                `json:"fileType"`
	SourceProvider string                `json:"sourceProvider"`
	ExternalId     string                `json:"externalId,omitempty"`
	Status         string                `json:"status"`
	Segments       []DocumentSegmentItem `json:"segments"`
}

type PopularQuestionsReq struct {
	g.Meta        `path:"/qa/popular-questions" method:"get" tags:"V1/问答" sm:"热门问题" dc:"按用户和知识库返回高频推荐问题"`
	KnowledgeCode string `json:"knowledgeCode" dc:"知识库编码，不传则统计当前用户可访问范围"`
	Limit         int    `json:"limit" dc:"返回条数，默认3，最大20"`
	Days          int    `json:"days" dc:"统计最近天数，默认30，最大365"`
}

type PopularQuestionItem struct {
	Question      string `json:"question"`
	KnowledgeCode string `json:"knowledgeCode,omitempty"`
	HitCount      int    `json:"hitCount"`
	LastAskedAt   int    `json:"lastAskedAt,omitempty"`
	Source        string `json:"source"`
}

type PopularQuestionsRes struct {
	List []PopularQuestionItem `json:"list"`
}

type WebSearchReq struct {
	g.Meta        `path:"/qa/web-search" method:"post" tags:"V1/问答" sm:"联网搜索" dc:"问答端联网搜索补充，当前仅提供配置占位和统一响应结构"`
	Query         string `json:"query" v:"required#搜索问题不能为空"`
	KnowledgeCode string `json:"knowledgeCode" dc:"知识库编码，用于按权限限定搜索上下文"`
	Limit         int    `json:"limit" dc:"返回条数，默认5，最大10"`
}

type WebSearchItem struct {
	Title   string  `json:"title"`
	Url     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Source  string  `json:"source"`
	Score   float64 `json:"score"`
}

type WebSearchRes struct {
	Enabled  bool            `json:"enabled"`
	Provider string          `json:"provider"`
	Message  string          `json:"message"`
	List     []WebSearchItem `json:"list"`
}

type QaSyncReq struct {
	g.Meta `path:"/qa/sync/knowledge-bases" method:"post" tags:"V1/问答同步" sm:"同步知识库" dc:"预留 AIDGP 知识库同步入口"`
}

type QaSyncDocumentsReq struct {
	g.Meta        `path:"/qa/sync/documents" method:"post" tags:"V1/问答同步" sm:"同步文档" dc:"预留 AIDGP 文档同步入口"`
	KnowledgeCode string `json:"knowledgeCode" dc:"知识库编码"`
}

type QaSyncPermissionsReq struct {
	g.Meta        `path:"/qa/sync/permissions" method:"post" tags:"V1/问答同步" sm:"同步权限" dc:"预留 AIDGP 权限同步入口"`
	KnowledgeCode string `json:"knowledgeCode" dc:"知识库编码"`
}

type QaSyncGridDataReq struct {
	g.Meta `path:"/qa/sync/grid-data" method:"post" tags:"V1/同步" sm:"同步网格数据" dc:"触发 AIDGP 网格数据同步任务"`
}

type QaSyncTrafficDataReq struct {
	g.Meta `path:"/qa/sync/traffic-data" method:"post" tags:"V1/同步" sm:"同步车流数据" dc:"触发 AIDGP 车流数据同步任务"`
}

type QaSyncPopulationDataReq struct {
	g.Meta `path:"/qa/sync/population-data" method:"post" tags:"V1/同步" sm:"同步人流数据" dc:"触发 AIDGP 人流数据同步任务；格式未配置时只记录跳过日志"`
}

type QaSyncRes struct {
	TaskId       int64  `json:"taskId"`
	Provider     string `json:"provider"`
	SyncType     string `json:"syncType"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	SuccessCount int    `json:"successCount"`
	FailureCount int    `json:"failureCount"`
	SkippedCount int    `json:"skippedCount"`
}

type QaSyncStatusReq struct {
	g.Meta   `path:"/qa/sync/status" method:"get" tags:"V1/问答同步" sm:"同步状态" dc:"查询问答端同步任务状态"`
	Limit    int    `json:"limit" dc:"返回条数，默认10，最大50"`
	Provider string `json:"provider" dc:"同步供应商"`
	SyncType string `json:"syncType" dc:"同步类型"`
}

type QaSyncTaskItem struct {
	TaskId       int64  `json:"taskId"`
	Provider     string `json:"provider"`
	SyncType     string `json:"syncType"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	SuccessCount int    `json:"successCount"`
	FailureCount int    `json:"failureCount"`
	SkippedCount int    `json:"skippedCount"`
	StartedAt    int    `json:"startedAt"`
	FinishedAt   int    `json:"finishedAt"`
	CreateTime   int    `json:"createTime"`
	UpdateTime   int    `json:"updateTime"`
}

type QaSyncStatusRes struct {
	Provider string           `json:"provider"`
	Enabled  bool             `json:"enabled"`
	List     []QaSyncTaskItem `json:"list"`
}

type QaSyncLogsReq struct {
	g.Meta `path:"/qa/sync/tasks/{taskId}/logs" method:"get" tags:"V1/同步" sm:"同步日志" dc:"查询同步任务明细日志"`
	TaskId int64  `json:"taskId" in:"path" v:"required#任务ID不能为空"`
	Status string `json:"status" dc:"按日志状态过滤：success/skipped/failed"`
	Limit  int    `json:"limit" dc:"返回条数，默认50，最大200"`
}

type QaSyncLogItem struct {
	LogId      int64  `json:"logId"`
	TaskId     int64  `json:"taskId"`
	Provider   string `json:"provider"`
	SyncType   string `json:"syncType"`
	ExternalId string `json:"externalId"`
	LocalId    string `json:"localId"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreateTime int    `json:"createTime"`
}

type QaSyncLogsRes struct {
	TaskId int64           `json:"taskId"`
	List   []QaSyncLogItem `json:"list"`
}

type SessionResetReq struct {
	g.Meta    `path:"/qa/sessions/{sessionId}/reset" method:"post" tags:"V1/问答" sm:"重置问答会话" dc:"清空指定问答会话上下文"`
	SessionId string `json:"sessionId" in:"path" v:"required#会话ID不能为空"`
}

type SessionResetRes struct {
	SessionId string `json:"sessionId"`
}
