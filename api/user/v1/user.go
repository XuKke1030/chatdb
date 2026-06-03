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

type UiapCallbackReq struct {
	g.Meta      `path:"/user/uiap/callback" method:"post" tags:"V1/User" sm:"UIAP login callback" dc:"Exchange UIAP auth code and login" noAuth:"true"`
	Code        string `json:"code" v:"required#请填写UIAP授权码"`
	RedirectUri string `json:"redirectUri"`
	State       string `json:"state"`
}

type UiapCallbackRes struct {
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
	Authenticated     bool                      `json:"authenticated" dc:"是否已认证"`
	UserId            int64                     `json:"userId,omitempty" dc:"用户ID"`
	Username          string                    `json:"username,omitempty" dc:"用户名"`
	RuleLevel         int                       `json:"ruleLevel" dc:"权限等级"`
	PermissionVersion int                       `json:"permissionVersion" dc:"权限版本"`
	Permissions       []model.TopicItem         `json:"permissions" dc:"主题权限列表"`
	QaPermissions     []UserKnowledgePermission `json:"qaPermissions" dc:"问答知识库权限"`
}

type UserBootstrapReq struct {
	g.Meta `path:"/user/bootstrap" method:"get" tags:"V1/鐢ㄦ埛" sm:"棣栧睆Bootstrap" dc:"鑾峰彇棣栧睆鎵€闇€鐨勭敤鎴枫€佹潈闄愩€佺儹闂ㄩ棶棰樺拰鍛婅鎽樿"`
}

type UserBootstrapRes struct {
	Authenticated     bool                      `json:"authenticated" dc:"是否已认证"`
	User              *model.User               `json:"user,omitempty" dc:"用户信息"`
	RuleLevel         int                       `json:"ruleLevel" dc:"权限等级"`
	PermissionVersion int                       `json:"permissionVersion" dc:"权限版本"`
	TopicPermissions  []model.TopicItem         `json:"topicPermissions" dc:"主题权限列表"`
	Topics            []model.TopicItem         `json:"topics" dc:"可用主题列表"`
	QaPermissions     []UserKnowledgePermission `json:"qaPermissions" dc:"问答知识库权限"`
	KnowledgeBases    []UserKnowledgePermission `json:"knowledgeBases" dc:"可用知识库列表"`
	PopularQuestions  []UserPopularQuestionItem `json:"popularQuestions" dc:"热门问题列表"`
	AlertSummary      UserAlertSummary          `json:"alertSummary" dc:"告警摘要"`
	WelcomeMessage    string                    `json:"welcomeMessage" dc:"欢迎语"`
	WelcomeSubtext    string                    `json:"welcomeSubtext" dc:"欢迎副标题"`
}

type UserAlertSummary struct {
	Total    int               `json:"total" dc:"告警总数"`
	Critical int               `json:"critical" dc:"严重告警数"`
	Warning  int               `json:"warning" dc:"警告数"`
	ByTopic  map[string]int    `json:"byTopic" dc:"按主题统计"`
	Latest   []model.AlertItem `json:"latest" dc:"最新告警"`
}

type UserKnowledgePermission struct {
	Code    string `json:"code" dc:"知识库编码"`
	Name    string `json:"name" dc:"知识库名称"`
	Enabled bool   `json:"enabled" dc:"是否启用"`
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
