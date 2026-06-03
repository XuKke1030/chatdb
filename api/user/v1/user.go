package v1

import (
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

type UserLoginReq struct {
	g.Meta `path:"/user/login" method:"post" tags:"V1/用户" sm:"登录" dc:"登录" noAuth:"true"`
	model.UserLoginInput
}

type UserLoginRes struct {
	model.JWTGenTokenOutput
	model.User
}

type UserRegisterReq struct {
	g.Meta `path:"/user/register" method:"post" tags:"V1/用户" sm:"注册" dc:"注册" noAuth:"true"`
	model.UserRegisterInput
}

type UserRegisterRes struct {
	model.JWTGenTokenOutput
}

type UserPermissionsReq struct {
	g.Meta `path:"/user/permissions" method:"get" tags:"V1/用户" sm:"用户权限" dc:"获取当前用户的主题权限"`
}

type UserPermissionsRes struct {
	RuleLevel     int                       `json:"ruleLevel"`
	Permissions   []model.TopicItem         `json:"permissions"`
	QaPermissions []UserKnowledgePermission `json:"qaPermissions"`
}

type UserKnowledgePermission struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type UserTopicsReq struct {
	g.Meta `path:"/topics" method:"get" tags:"V1/用户" sm:"主题列表" dc:"获取当前用户可访问的问数主题"`
}

type UserTopicsRes struct {
	List []model.TopicItem `json:"list"`
}

type UserKnowledgeBasesReq struct {
	g.Meta `path:"/knowledge-bases" method:"get" tags:"V1/用户" sm:"问答知识库" dc:"获取当前用户可访问的问答知识库"`
}

type UserKnowledgeBasesRes struct {
	List []UserKnowledgePermission `json:"list"`
}

type UserPopularQuestionsReq struct {
	g.Meta        `path:"/popular-questions" method:"get" tags:"V1/用户" sm:"热门问题" dc:"获取当前用户高频问题Top3"`
	KnowledgeCode string `json:"knowledgeCode" p:"knowledgeCode"`
}

type UserPopularQuestionItem struct {
	Question string `json:"question"`
	Count    int    `json:"count"`
}

type UserPopularQuestionsRes struct {
	List []UserPopularQuestionItem `json:"list"`
}

type UserBootstrapReq struct {
	g.Meta `path:"/user/bootstrap" method:"get" tags:"V1/用户" sm:"用户引导" dc:"获取当前用户欢迎语和认证状态"`
}

type UserBootstrapRes struct {
	WelcomeMessage string `json:"welcomeMessage"`
	WelcomeSubtext string `json:"welcomeSubtext"`
	Authenticated  bool   `json:"authenticated"`
}
