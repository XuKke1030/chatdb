package v1

import (
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

// AlertListReq 告警列表请求
type AlertListReq struct {
	g.Meta `path:"/alerts" method:"get" tags:"V1/告警" sm:"告警列表" dc:"获取当前用户的告警列表"`
	Topic  string `json:"topic" in:"query" dc:"主题筛选：grid|population|traffic"`
	Topics string `json:"topics" in:"query" dc:"主题筛选，逗号分隔：grid,population,traffic"`
}

type AlertListRes struct {
	List   []model.AlertItem  `json:"list"`
	Topics []AlertTopicResult `json:"topics"`
}

type AlertTopicResult struct {
	Topic     string            `json:"topic"`
	HasAlert  bool              `json:"hasAlert"`
	EmptyText string            `json:"emptyText"`
	Alerts    []model.AlertItem `json:"alerts"`
}

// AlertDismissReq 关闭告警请求
type AlertDismissReq struct {
	g.Meta `path:"/alerts/{id}/dismiss" method:"post" tags:"V1/告警" sm:"关闭告警" dc:"关闭指定告警"`
	Id     int `json:"id" in:"path" v:"required#告警ID不能为空" dc:"告警ID"`
}

type AlertDismissRes struct{}
