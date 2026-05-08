package model

import (
	"context"

	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"
)

type ChatInput struct {
	Ai            string            `json:"ai" v:"required#AI类型不能为空" dc:"AI类型"`
	Model         string            `json:"model" v:"required#模型不能为空" dc:"模型"`
	Prompt        string            `json:"prompt" dc:"临时提示词"`
	Message       string            `json:"message" v:"required#消息不能为空" dc:"消息"`
	DatabaseId    int               `json:"databaseId" v:"required#数据库配置ID不能为空" dc:"数据库配置ID"`
	Topic         string            `json:"topic" dc:"主题：grid|population|traffic"`
	KnowledgeCode string            `json:"knowledgeCode" dc:"问答知识库编码"`
	SessionId     string            `json:"sessionId" dc:"问数会话ID，不传时自动创建"`
	History       []ChatHistoryItem `json:"history" dc:"前端补充的历史消息"`
}

type ChatHistoryItem struct {
	Role    string `json:"role" dc:"消息角色：user|assistant"`
	Content string `json:"content" dc:"消息内容"`
}

type ChatOutDataItem struct {
	Event      string `json:"event"`
	TraceId    string `json:"traceId,omitempty"`
	Role       string `json:"role,omitempty"`
	Content    string `json:"content,omitempty"`
	Data       any    `json:"data,omitempty"`
	CreateTime int64  `json:"createTime"`
}

func GenChatOutDataItem(ctx context.Context, in ChatOutDataItem) ChatOutDataItem {
	in.TraceId = gctx.CtxId(ctx)
	in.CreateTime = gtime.TimestampMilli()
	return in
}

func SendChatOutDataItem(ctx context.Context, in ChatOutDataItem, respChan chan any) (err error) {
	item := GenChatOutDataItem(ctx, in)
	respChan <- item
	return
}
