package qa

import "ai-chat-sql/api/qa"

type ControllerV1 struct{}

func NewV1() qa.IQaV1 {
	return &ControllerV1{}
}
