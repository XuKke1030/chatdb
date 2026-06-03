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

type ChatSessionCreateReq struct {
	g.Meta  `path:"/chats/sessions" method:"post" tags:"V1/聊天" sm:"创建会话" dc:"创建对话会话"`
	Topic   string `json:"topic" dc:"主题：grid|population|traffic"`
	Source  string `json:"source" d:"topic_entry" dc:"来源：topic_entry|topic_alert"`
	AlertId int    `json:"alertId" d:"0" dc:"告警ID"`
}

type ChatSessionCreateRes struct {
	SessionId          string   `json:"sessionId"`
	SuggestedQuestions []string `json:"suggestedQuestions"`
	InputPlaceholder   string   `json:"inputPlaceholder"`
}

type ChatSessionResetReq struct {
	g.Meta `path:"/chats/sessions/{id}/reset" method:"post" tags:"V1/聊天" sm:"重置会话" dc:"重置对话会话"`
	Id     string `p:"id" in:"path" dc:"会话ID"`
}

type ChatSessionResetRes struct{}
