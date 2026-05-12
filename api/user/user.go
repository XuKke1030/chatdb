// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package user

import (
	"context"

	"ai-chat-sql/api/user/v1"
)

type IUserV1 interface {
	UserLogin(ctx context.Context, req *v1.UserLoginReq) (res *v1.UserLoginRes, err error)
	UserRegister(ctx context.Context, req *v1.UserRegisterReq) (res *v1.UserRegisterRes, err error)
	UserPermissions(ctx context.Context, req *v1.UserPermissionsReq) (res *v1.UserPermissionsRes, err error)
	UserBootstrap(ctx context.Context, req *v1.UserBootstrapReq) (res *v1.UserBootstrapRes, err error)
	UserTopics(ctx context.Context, req *v1.UserTopicsReq) (res *v1.UserTopicsRes, err error)
	UserKnowledgeBases(ctx context.Context, req *v1.UserKnowledgeBasesReq) (res *v1.UserKnowledgeBasesRes, err error)
	UserPopularQuestions(ctx context.Context, req *v1.UserPopularQuestionsReq) (res *v1.UserPopularQuestionsRes, err error)
}
