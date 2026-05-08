package v1

import (
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

type ChatReq struct {
	g.Meta `path:"/chats" method:"post" tags:"V1/聊天" sm:"聊天" dc:"聊天"`
	model.ChatInput
}

type ChatRes struct {
	g.Meta `mime:"text/event-stream"`
}

type ChatSessionResetReq struct {
	g.Meta    `path:"/chats/sessions/{sessionId}/reset" method:"post" tags:"V1/聊天" sm:"重置问数会话" dc:"清空指定问数会话上下文"`
	SessionId string `json:"sessionId" in:"path" v:"required#会话ID不能为空" dc:"问数会话ID"`
}

type ChatSessionResetRes struct {
	SessionId string `json:"sessionId"`
}

type GridMajorCaseAnalysisReq struct {
	g.Meta `path:"/grid/major-case-analysis" method:"get" tags:"V1/问数" sm:"重大案件分析" dc:"按影响度/难度对网格案件排序，返回Top1案件信息及评分依据"`
	Metric string `json:"metric" in:"query" dc:"排序口径：combined|impact|difficulty，默认combined"`
	Region string `json:"region" in:"query" dc:"所属区域筛选"`
}

type GridMajorCaseAnalysisRes struct {
	Metric string        `json:"metric"`
	Case   MajorCaseItem `json:"case"`
	Basis  []string      `json:"basis"`
}

type MajorCaseItem struct {
	Id                 int64    `json:"id"`
	CaseName           string   `json:"caseName"`
	CaseNumber         string   `json:"caseNumber"`
	CaseSource         string   `json:"caseSource"`
	ReportTime         string   `json:"reportTime"`
	PendingStep        string   `json:"pendingStep"`
	CaseType           string   `json:"caseType"`
	Region             string   `json:"region"`
	ResponsibilityUnit string   `json:"responsibilityUnit"`
	CaseLocation       string   `json:"caseLocation"`
	Description        string   `json:"description"`
	ImpactScore        int      `json:"impactScore"`
	DifficultyScore    int      `json:"difficultyScore"`
	TotalScore         int      `json:"totalScore"`
	Reasons            []string `json:"reasons"`
}
