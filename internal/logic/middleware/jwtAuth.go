package middleware

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"os"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// jwtDevBypass controls whether missing/invalid JWT is allowed through.
// In production this MUST be "false" (or unset).
var jwtDevBypass = strings.EqualFold(os.Getenv("CHATDB_JWT_DEV_BYPASS"), "true")

// noAuthPaths are routes that do not require a valid JWT.
var noAuthPaths = map[string]bool{
	"/api/v1/user/login":       true,
	"/api/v1/user/register":    true,
	"/api/v1/user/uiap/callback": true,
}

// JwtAuth validates user JWT tokens. In production mode (default), requests
// without a valid token receive 401 — except routes in noAuthPaths.
// When CHATDB_JWT_DEV_BYPASS=true, invalid tokens are logged and the
// request continues as an unauthenticated user.
func (s *sMiddleware) JwtAuth(subject string) func(r *ghttp.Request) {
	return func(r *ghttp.Request) {
		ctx := r.GetCtx()
		userId := 0

		// Skip auth for public routes
		if noAuthPaths[r.URL.Path] {
			r.SetCtx(context.WithValue(ctx, model.UserGroup{}, 0))
			r.Middleware.Next()
			return
		}

		token := strings.TrimSpace(r.Header.Get("Authorization"))
		token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		if token != "" {
			if out, valid, err := service.Jwt().VerifyToken(ctx, &model.JWTVerifyTokenInput{
				Token:   token,
				Subject: subject,
			}); err == nil && valid && out != nil {
				userId = out.Id
			} else if err != nil {
				consts.Logger.Warningf(ctx, "JWT 解析失败: %s", err.Error())
			}
		}

		if userId <= 0 && !jwtDevBypass {
			r.Response.WriteStatus(401)
			r.Response.WriteJson(map[string]any{"code": 401, "message": "未登录或令牌已过期"})
			return
		}

		r.SetCtx(context.WithValue(ctx, model.UserGroup{}, userId))
		r.Middleware.Next()
	}
}
