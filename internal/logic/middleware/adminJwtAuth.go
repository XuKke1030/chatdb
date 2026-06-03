package middleware

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// AdminJwtAuth rejects requests without a valid admin JWT.
// /admin/login is exempted (it has noAuth:"true" in API definition).
func (s *sMiddleware) AdminJwtAuth(r *ghttp.Request) {
	// Skip auth for login endpoint (exact match to prevent path-traversal bypass)
	if r.URL.Path == "/api/v1/admin/login" {
		r.Middleware.Next()
		return
	}

	ctx := r.GetCtx()
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))

	if token == "" {
		r.Response.WriteStatus(401, `{"code":401,"message":"未登录或登录已过期"}`)
		r.Exit()
		return
	}

	out, valid, err := service.Jwt().VerifyToken(ctx, &model.JWTVerifyTokenInput{
		Token:   token,
		Subject: consts.JwtSubjectAdmin,
	})
	if err != nil || !valid || out == nil {
		r.Response.WriteStatus(401, `{"code":401,"message":"登录已过期，请重新登录"}`)
		r.Exit()
		return
	}

	r.SetCtx(model.ContextWithUserId(ctx, out.Id))
	r.Middleware.Next()
}
