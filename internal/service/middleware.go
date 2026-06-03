// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

type (
	IMiddleware interface {
		// JwtAuth 校验jwt
		JwtAuth(subject string) func(r *ghttp.Request)
		// RequestMetrics 记录请求总耗时
		RequestMetrics(r *ghttp.Request)
		// HandlerResponse 处理 Http 请求返回结果
		HandlerResponse(r *ghttp.Request)
		// AdminJwtAuth 校验管理员jwt，无效则返回401
		AdminJwtAuth(r *ghttp.Request)
		// RecoverPanic 捕获panic并记录日志
		RecoverPanic(r *ghttp.Request)
	}
)

var (
	localMiddleware IMiddleware
)

func Middleware() IMiddleware {
	if localMiddleware == nil {
		panic("implement not found for interface IMiddleware, forgot register?")
	}
	return localMiddleware
}

func RegisterMiddleware(i IMiddleware) {
	localMiddleware = i
}
