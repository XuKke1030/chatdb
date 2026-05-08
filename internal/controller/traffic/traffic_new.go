package traffic

import (
	"ai-chat-sql/api/traffic"
)

type ControllerV1 struct{}

func NewV1() traffic.ITrafficV1 {
	return &ControllerV1{}
}
