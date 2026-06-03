package middleware

import (
	"ai-chat-sql/internal/consts"
	"fmt"
	"runtime/debug"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *sMiddleware) RecoverPanic(r *ghttp.Request) {
	defer func() {
		if err := recover(); err != nil {
			consts.Logger.Errorf(r.Context(), "panic recovered: %v\n%s", err, debug.Stack())
			r.Response.WriteStatus(500, DefaultHandlerResponse{
				Code:    gcode.CodeInternalError.Code(),
				Message: "internal server error",
			})
			r.SetError(fmt.Errorf("panic: %v", err))
		}
	}()
	r.Middleware.Next()
}
