package middleware

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// JwtAuth 在开发阶段仍放行所有接口；如果请求带 JWT，则解析用户身份写入上下文，
// 供主题权限、告警过滤等接口按当前账号返回数据。
func (s *sMiddleware) JwtAuth(subject string) func(r *ghttp.Request) {
	return func(r *ghttp.Request) {
		ctx := r.GetCtx()
		userId := 0

		token := strings.TrimSpace(r.Header.Get("Authorization"))
		token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		if token != "" {
			if out, valid, err := service.Jwt().VerifyToken(ctx, &model.JWTVerifyTokenInput{
				Token:   token,
				Subject: subject,
			}); err == nil && valid && out != nil {
				userId = out.Id
			} else if err != nil {
				consts.Logger.Warningf(ctx, "JWT 解析失败，按未登录用户处理: %s", err.Error())
			}
		}

		r.SetCtx(context.WithValue(ctx, model.UserGroup{}, userId))
		r.Middleware.Next()
	}
}
