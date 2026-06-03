package middleware

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *sMiddleware) RequestMetrics(r *ghttp.Request) {
	start := time.Now()
	r.Middleware.Next()

	status := r.Response.Status
	if status == 0 {
		status = 200
	}
	userId := model.UserIdFromContext(r.GetCtx())
	costMs := time.Since(start).Milliseconds()
	logFormat := "perf request method=%s path=%s status=%d userId=%d costMs=%d"
	args := []any{r.Method, r.URL.Path, status, userId, costMs}
	if costMs >= 3000 {
		consts.Logger.Errorf(r.GetCtx(), logFormat, args...)
		return
	}
	if costMs >= 1000 {
		consts.Logger.Warningf(r.GetCtx(), logFormat, args...)
		return
	}
	consts.Logger.Infof(r.GetCtx(), logFormat, args...)
}
