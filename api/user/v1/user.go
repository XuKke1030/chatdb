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
	Authenticated     bool                      `json:"authenticated"`
	UserId            int64                     `json:"userId,omitempty"`
	Username          string                    `json:"username,omitempty"`
	RuleLevel         int                       `json:"ruleLevel"`
	PermissionVersion int                       `json:"permissionVersion"`
	Permissions       []model.TopicItem         `json:"permissions"`
	QaPermissions     []UserKnowledgePermission `json:"qaPermissions"`
}

type UserBootstrapReq struct {
	g.Meta `path:"/user/bootstrap" method:"get" tags:"V1/鐢ㄦ埛" sm:"棣栧睆Bootstrap" dc:"鑾峰彇棣栧睆鎵€闇€鐨勭敤鎴枫€佹潈闄愩€佺儹闂ㄩ棶棰樺拰鍛婅鎽樿"`
}

type UserBootstrapRes struct {
	Authenticated     bool                      `json:"authenticated"`
	User              *model.User               `json:"user,omitempty"`
	RuleLevel         int                       `json:"ruleLevel"`
	PermissionVersion int                       `json:"permissionVersion"`
	TopicPermissions  []model.TopicItem         `json:"topicPermissions"`
	Topics            []model.TopicItem         `json:"topics"`
	QaPermissions     []UserKnowledgePermission `json:"qaPermissions"`
	KnowledgeBases    []UserKnowledgePermission `json:"knowledgeBases"`
	PopularQuestions  []UserPopularQuestionItem `json:"popularQuestions"`
	AlertSummary      UserAlertSummary          `json:"alertSummary"`
	WelcomeMessage    string                    `json:"welcomeMessage"`
	WelcomeSubtext    string                    `json:"welcomeSubtext"`
}

type UserAlertSummary struct {
	Total    int               `json:"total"`
	Critical int               `json:"critical"`
	Warning  int               `json:"warning"`
	ByTopic  map[string]int    `json:"byTopic"`
	Latest   []model.AlertItem `json:"latest"`
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
