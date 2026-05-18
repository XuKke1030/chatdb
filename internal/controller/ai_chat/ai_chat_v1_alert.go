package ai_chat

import (
	"context"
	"strings"

	v1 "ai-chat-sql/api/ai_chat/v1"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/util/gconv"
)

func (c *ControllerV1) AlertList(ctx context.Context, req *v1.AlertListReq) (res *v1.AlertListRes, err error) {
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}

	topics := requestedAlertTopics(req.Topic, req.Topics)
	grouped, err := service.Alert().GetAlertListByTopics(ctx, userId, topics)
	if err != nil {
		return nil, err
	}
	res = &v1.AlertListRes{
		List:   []model.AlertItem{},
		Topics: make([]v1.AlertTopicResult, 0, len(grouped)),
	}
	for _, topic := range []string{"grid", "population", "traffic"} {
		alerts, ok := grouped[topic]
		if !ok {
			continue
		}
		if alerts == nil {
			alerts = []model.AlertItem{}
		}
		res.List = append(res.List, alerts...)
		res.Topics = append(res.Topics, v1.AlertTopicResult{
			Topic:     topic,
			HasAlert:  len(alerts) > 0,
			EmptyText: emptyAlertText(alerts),
			Alerts:    alerts,
		})
	}
	return res, nil
}

func (c *ControllerV1) AlertDismiss(ctx context.Context, req *v1.AlertDismissReq) (res *v1.AlertDismissRes, err error) {
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}

	err = service.Alert().DismissAlert(ctx, userId, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AlertDismissRes{}, nil
}

func requestedAlertTopics(topic string, topics string) []string {
	raw := make([]string, 0, 3)
	if strings.TrimSpace(topics) != "" {
		raw = append(raw, strings.Split(topics, ",")...)
	}
	if strings.TrimSpace(topic) != "" {
		raw = append(raw, topic)
	}
	if len(raw) == 0 {
		return []string{"grid", "population", "traffic"}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func emptyAlertText(alerts []model.AlertItem) string {
	if len(alerts) == 0 {
		return "暂无异常"
	}
	return ""
}
