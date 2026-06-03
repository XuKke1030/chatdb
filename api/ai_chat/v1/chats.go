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
	SessionId string `json:"sessionId" in:"path" v:"required|length:1,64#会话ID不能为空|会话ID长度不合法" dc:"问数会话ID"`
}

type ChatSessionResetRes struct {
	SessionId string `json:"sessionId" dc:"问数会话ID"`
}

type ChatSessionCreateReq struct {
	g.Meta  `path:"/chats/sessions" method:"post" tags:"V1/聊天" sm:"创建问数会话" dc:"按主题创建或初始化问数会话"`
	Topic   string `json:"topic" v:"required|in:grid,population,traffic#主题不能为空|主题须为grid/population/traffic" dc:"主题：grid|population|traffic"`
	Source  string `json:"source" dc:"来源：topic_entry|topic_alert"`
	AlertId int    `json:"alertId" dc:"告警ID"`
}

type ChatSessionCreateRes struct {
	SessionId          string   `json:"sessionId" dc:"问数会话ID"`
	Topic              string   `json:"topic" dc:"主题"`
	Title              string   `json:"title" dc:"会话标题"`
	SuggestedQuestions []string `json:"suggestedQuestions" dc:"推荐问题"`
	InputPlaceholder   string   `json:"inputPlaceholder" dc:"输入框占位提示"`
}

type ExampleQuestionsReq struct {
	g.Meta `path:"/chats/example-questions" method:"get" tags:"V1/聊天" sm:"获取示例问题" dc:"按主题获取用户可用的示例问题列表"`
	Topic  string `json:"topic" p:"topic" dc:"主题筛选：grid|population|traffic，为空则返回全部"`
}

type ExampleQuestionsRes struct {
	Items []ExampleQuestionItem `json:"items" dc:"示例问题列表"`
}

type ExampleQuestionItem struct {
	Id          int    `json:"id" dc:"记录ID"`
	Topic       string `json:"topic" dc:"主题"`
	Question    string `json:"question" dc:"问题内容"`
	Description string `json:"description,omitempty" dc:"描述"`
}

type GridMajorCaseAnalysisReq struct {
	g.Meta `path:"/grid/major-case-analysis" method:"get" tags:"V1/问数" sm:"重大案件分析" dc:"按影响度/难度对网格案件排序，返回Top1案件信息及评分依据"`
	Metric string `json:"metric" in:"query" v:"in:combined,impact,difficulty#排序口径须为combined/impact/difficulty" dc:"排序口径：combined|impact|difficulty，默认combined"`
	Region string `json:"region" in:"query" dc:"所属区域筛选"`
}

type GridMajorCaseAnalysisRes struct {
	Metric     string        `json:"metric" dc:"排序口径"`
	Case       MajorCaseItem `json:"case" dc:"重大案件详情"`
	Basis      []string      `json:"basis" dc:"评分依据"`
	Suggestion []string      `json:"suggestion" dc:"处置建议"`
}

type MajorCaseItem struct {
	Id                 int64    `json:"id" dc:"记录ID"`
	CaseName           string   `json:"caseName" dc:"案件名称"`
	CaseNumber         string   `json:"caseNumber" dc:"案件编号"`
	CaseSource         string   `json:"caseSource" dc:"案件来源"`
	ReportTime         string   `json:"reportTime" dc:"上报时间"`
	PendingStep        string   `json:"pendingStep" dc:"待办环节"`
	CaseType           string   `json:"caseType" dc:"案件类别"`
	Region             string   `json:"region" dc:"所属区域"`
	ResponsibilityUnit string   `json:"responsibilityUnit" dc:"责任单位"`
	CaseLocation       string   `json:"caseLocation" dc:"案件位置"`
	Description        string   `json:"description" dc:"问题描述"`
	ImpactScore        int      `json:"impactScore" dc:"影响度评分"`
	DifficultyScore    int      `json:"difficultyScore" dc:"难度评分"`
	TimeRiskScore      int      `json:"timeRiskScore" dc:"时效风险评分"`
	TotalScore         int      `json:"totalScore" dc:"综合评分"`
	Level              string   `json:"level" dc:"案件等级"`
	Reasons            []string `json:"reasons" dc:"评分原因"`
	Suggestion         []string `json:"suggestion" dc:"处置建议"`
}
