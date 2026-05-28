package cmd

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/controller/admin"
	"ai-chat-sql/internal/controller/ai_chat"
	// "ai-chat-sql/internal/controller/qa" // 问答模块暂时注释
	"ai-chat-sql/internal/controller/traffic"
	"ai-chat-sql/internal/controller/user"
	"ai-chat-sql/internal/logic/precipitate"
	"ai-chat-sql/internal/logic/sync"
	"ai-chat-sql/internal/logic/uiap"
	"ai-chat-sql/internal/packed"
	"ai-chat-sql/internal/service"
	"context"
	"net/url"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/os/gcmd"
)

// corsMiddleware returns a middleware that applies CORS based on the
// configured allowOrigins list. If the list is empty (dev mode), all
// origins are permitted. Otherwise only whitelisted origins are allowed.
func corsMiddleware(r *ghttp.Request) {
	allowOrigins := g.Cfg().MustGet(r.Context(), "cors.allowOrigins").Strings()
	origin := r.Header.Get("Origin")

	var allowOrigin string
	if len(allowOrigins) == 0 {
		// Dev mode: mirror the request origin (same as GoFrame default)
		if origin != "" {
			allowOrigin = origin
		} else {
			allowOrigin = "*"
		}
	} else {
		// Production: only allow whitelisted origins
		for _, o := range allowOrigins {
			if o == origin {
				allowOrigin = origin
				break
			}
			// Support wildcard subdomain matching: *.example.com
			if len(o) > 2 && o[:2] == "*." {
				if u, err := url.Parse(origin); err == nil {
					if u.Hostname() == o[2:] || len(u.Hostname()) > len(o[2:]) && u.Hostname()[len(u.Hostname())-len(o[2:])-1:] == "."+o[2:] {
						allowOrigin = origin
						break
					}
				}
			}
		}
	}

	if allowOrigin == "" {
		r.Middleware.Next()
		return
	}

	r.Response.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	r.Response.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
	r.Response.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
	r.Response.Header().Set("Access-Control-Allow-Credentials", "true")
	r.Response.Header().Set("Access-Control-Max-Age", "86400")

	if r.Method == "OPTIONS" {
		r.Response.WriteStatus(204)
		return
	}

	r.Middleware.Next()
}

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			if err = packed.CheckDatabase(ctx); err != nil {
				return err
			}
			if err = service.SystemInit().InitDB(ctx); err != nil {
				return err
			}
			go service.Population().RefreshAggregates(ctx, "2020-01-01", gtime.Now().AddDate(1, 0, 0).Format("Y-m-d"))
			service.Traffic().StartMqttSubscriber(ctx)
			uiap.StartPermissionPoller(ctx)
	sync.StartScheduler(ctx)
			go precipitate.StartCandidateScanner(ctx, 10*time.Minute, 3)
			s := g.Server()
			s.Group("/api/v1", func(group *ghttp.RouterGroup) {
				group.Middleware(service.Middleware().RequestMetrics, service.Middleware().HandlerResponse, corsMiddleware)

				{
					authGroup := group.Clone()
					authGroup.Middleware(service.Middleware().JwtAuth(consts.JwtSubjectUser)).
						Bind(ai_chat.NewV1(), user.NewV1(), /* qa.NewV1(), */ traffic.NewV1())
				}
				// Admin routes: AdminJwtAuth skips /admin/login automatically
				{
					adminGroup := group.Clone()
					adminGroup.Middleware(service.Middleware().AdminJwtAuth).
						Bind(admin.NewV1())
				}
			})
			s.Run()
			return nil
		},
	}
)
