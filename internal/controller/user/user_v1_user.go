package user

import (
	"context"

	v1 "ai-chat-sql/api/user/v1"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
)

func (c *ControllerV1) UserLogin(ctx context.Context, req *v1.UserLoginReq) (res *v1.UserLoginRes, err error) {
	user, out, err := service.User().Login(ctx, req.UserLoginInput)
	if err != nil {
		return
	}
	res = &v1.UserLoginRes{}
	res.JWTGenTokenOutput = *out
	err = gconv.Scan(user, &res.User)
	res.RuleLevel = effectiveRuleLevel(ctx, int64(res.UserId), user.RuleLevel)
	now := int(gtime.Timestamp())
	_, _ = g.DB("master").Model("user").Ctx(ctx).Where("user_id = ?", res.UserId).Data(g.Map{
		"last_login_tme": now,
		"update_time":    now,
	}).Update()
	_, _ = g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", res.UserId).Data(g.Map{
		"last_login_at": now,
		"update_time":   now,
	}).Update()
	_, _ = g.DB("master").Model("admin_operation_log").Ctx(ctx).Data(g.Map{
		"log_type":    "system",
		"username":    user.Username,
		"action_type": "用户登录",
		"content":     "用户登录问答问数平台",
		"result":      "success",
		"create_time": now,
	}).Insert()
	return
}

func (c *ControllerV1) UserRegister(ctx context.Context, req *v1.UserRegisterReq) (res *v1.UserRegisterRes, err error) {
	res = &v1.UserRegisterRes{}

	userId, err := service.User().Register(ctx, true, req.UserRegisterInput)
	if err != nil {
		return
	}
	out, err := service.User().GenJwtTokenByUserId(ctx, userId)
	if err != nil {
		return
	}

	res.JWTGenTokenOutput = *out
	return
}

func (c *ControllerV1) UserPermissions(ctx context.Context, req *v1.UserPermissionsReq) (res *v1.UserPermissionsRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return &v1.UserPermissionsRes{RuleLevel: 0, Permissions: topicItemsFromRuleLevel(0, false), QaPermissions: knowledgePermissionsForUser(ctx, 0, false)}, nil
	}

	user, err := service.User().GetUserInfoById(ctx, userId)
	if err != nil || user == nil {
		return &v1.UserPermissionsRes{RuleLevel: 0, Permissions: topicItemsFromRuleLevel(0, false), QaPermissions: knowledgePermissionsForUser(ctx, 0, false)}, nil
	}
	ruleLevel := effectiveRuleLevel(ctx, userId, user.RuleLevel)
	return &v1.UserPermissionsRes{
		RuleLevel:     ruleLevel,
		Permissions:   topicItemsFromRuleLevel(ruleLevel, false),
		QaPermissions: knowledgePermissionsForUser(ctx, userId, false),
	}, nil
}

func (c *ControllerV1) UserTopics(ctx context.Context, req *v1.UserTopicsReq) (res *v1.UserTopicsRes, err error) {
	userId := currentUserId(ctx)

	ruleLevel := 0
	if userId > 0 {
		user, userErr := service.User().GetUserInfoById(ctx, userId)
		if userErr == nil && user != nil {
			ruleLevel = effectiveRuleLevel(ctx, userId, user.RuleLevel)
		}
	}

	allTopics := topicItemsFromRuleLevel(ruleLevel, false)

	list := make([]model.TopicItem, 0, len(allTopics))
	for _, topic := range allTopics {
		if topic.Enabled {
			list = append(list, topic)
		}
	}

	return &v1.UserTopicsRes{List: list}, nil
}

func (c *ControllerV1) UserKnowledgeBases(ctx context.Context, req *v1.UserKnowledgeBasesReq) (res *v1.UserKnowledgeBasesRes, err error) {
	return &v1.UserKnowledgeBasesRes{List: knowledgePermissionsForUser(ctx, 0, true)}, nil
}

func (c *ControllerV1) UserPopularQuestions(ctx context.Context, req *v1.UserPopularQuestionsReq) (res *v1.UserPopularQuestionsRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return &v1.UserPopularQuestionsRes{List: []v1.UserPopularQuestionItem{}}, nil
	}
	username := usernameById(ctx, userId)
	if username == "" {
		return &v1.UserPopularQuestionsRes{List: []v1.UserPopularQuestionItem{}}, nil
	}

	records, err := g.DB("master").Model("admin_operation_log").Ctx(ctx).
		Fields("content, COUNT(1) AS count").
		Where("username = ? AND action_type = ? AND result = ? AND content <> ?", username, "问答查询", "success", "").
		Group("content").
		OrderDesc("count").
		Limit(3).
		All()
	if err != nil {
		return &v1.UserPopularQuestionsRes{List: []v1.UserPopularQuestionItem{}}, nil
	}

	list := make([]v1.UserPopularQuestionItem, 0, len(records))
	for _, record := range records {
		question := record["content"].String()
		if question == "" {
			continue
		}
		list = append(list, v1.UserPopularQuestionItem{
			Question: question,
			Count:    record["count"].Int(),
		})
	}
	return &v1.UserPopularQuestionsRes{List: list}, nil
}

func (c *ControllerV1) UserBootstrap(ctx context.Context, req *v1.UserBootstrapReq) (res *v1.UserBootstrapRes, err error) {
	userId := currentUserId(ctx)
	authenticated := userId > 0
	welcomeMessage := ""
	welcomeSubtext := ""

	if authenticated {
		username := usernameById(ctx, userId)
		welcomeMessage = "您好，" + username
		welcomeSubtext = "请选择业务主题"
	}

	return &v1.UserBootstrapRes{
		WelcomeMessage: welcomeMessage,
		WelcomeSubtext: welcomeSubtext,
		Authenticated:  authenticated,
	}, nil
}

func currentUserId(ctx context.Context) int64 {
	userIdVal := ctx.Value(model.UserGroup{})
	if userIdVal == nil {
		return 0
	}
	return gconv.Int64(userIdVal)
}

func usernameById(ctx context.Context, userId int64) string {
	record, err := g.DB("master").Model("user").Ctx(ctx).Fields("username").Where("user_id = ?", userId).One()
	if err != nil || record == nil {
		return ""
	}
	return record["username"].String()
}

func topicItemsFromRuleLevel(ruleLevel int, enabledOnly bool) []model.TopicItem {
	allTopics := []model.TopicItem{
		{Label: "网格", Value: "grid", Permission: 1},
		{Label: "人流", Value: "population", Permission: 2},
		{Label: "车流", Value: "traffic", Permission: 4},
	}

	list := make([]model.TopicItem, 0, len(allTopics))
	for _, topic := range allTopics {
		topic.Enabled = ruleLevel&topic.Permission != 0
		if enabledOnly && !topic.Enabled {
			continue
		}
		list = append(list, topic)
	}
	return list
}

func knowledgePermissionsForUser(ctx context.Context, userId int64, enabledOnly bool) []v1.UserKnowledgePermission {
	bases, err := g.DB("master").Model("admin_knowledge_base").Ctx(ctx).Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return []v1.UserKnowledgePermission{}
	}

	enabledByCode := make(map[string]bool, len(bases))
	for _, base := range bases {
		enabledByCode[base["code"].String()] = true
	}

	list := make([]v1.UserKnowledgePermission, 0, len(bases))
	for _, base := range bases {
		code := base["code"].String()
		item := v1.UserKnowledgePermission{
			Code:    code,
			Name:    base["name"].String(),
			Enabled: enabledByCode[code],
		}
		if enabledOnly && !item.Enabled {
			continue
		}
		list = append(list, item)
	}
	return list
}

func effectiveRuleLevel(ctx context.Context, userId int64, fallback int) int {
	record, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).
		Where("user_id = ?", userId).
		One()
	if err != nil || record == nil {
		return fallback
	}
	if record["enabled"].Val() != nil && record["enabled"].Int() == 0 {
		return 0
	}
	if record["rule_level"].Val() != nil {
		return record["rule_level"].Int()
	}
	return fallback
}
