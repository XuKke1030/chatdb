package ai_chat

import (
	"context"

	v1 "ai-chat-sql/api/ai_chat/v1"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
)

func (c *ControllerV1) AlertList(ctx context.Context, req *v1.AlertListReq) (res *v1.AlertListRes, err error) {
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = userIdVal.(int)
	}

	list, err := service.Alert().GetAlertList(ctx, userId, req.Topic)
	if err != nil {
		return nil, err
	}
	return &v1.AlertListRes{List: list}, nil
}

func (c *ControllerV1) AlertDismiss(ctx context.Context, req *v1.AlertDismissReq) (res *v1.AlertDismissRes, err error) {
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = userIdVal.(int)
	}

	err = service.Alert().DismissAlert(ctx, userId, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AlertDismissRes{}, nil
}
