package model

import "context"

type JWTGenTokenInput struct {
	Subject string `json:"subject"`
	Id      int64  `json:"id"`
}

type JWTGenTokenOutput struct {
	Token  string `json:"token,omitempty" dc:"token"`
	Expire int64  `json:"expire,omitempty" dc:"过期时间"`
}

type JWTVerifyTokenInput struct {
	Subject string `json:"subject"`
	Token   string `json:"token"`
}

type JWTVerifyTokenOutput struct {
	Id     int    `json:"id" dc:"包含的唯一ID"`
	Expire int    `json:"exp" dc:"过期时间"`
	JTI    string `json:"jti" dc:"随机值"`
}

type JwtOption struct {
	Secret  string `json:"secret" dc:"密钥"`
	Expire  int    `json:"expire" dc:"过期时间 (小时)"`
	Issuer  string `json:"issuer" dc:"签发者"`
	Subject string `json:"subject" dc:"主题"`
}

type userGroupKey struct{}

func UserIdFromContext(ctx context.Context) int {
	v, _ := ctx.Value(userGroupKey{}).(int)
	return v
}

func ContextWithUserId(ctx context.Context, id int) context.Context {
	return context.WithValue(ctx, userGroupKey{}, id)
}
