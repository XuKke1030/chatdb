package admin

import "ai-chat-sql/api/admin"

type ControllerV1 struct{}

func NewV1() admin.IAdminV1 {
	return &ControllerV1{}
}
