package user

import (
	"ai-chat-sql/internal/consts"
	"context"
	"fmt"
	"strings"
	"time"

	v1 "ai-chat-sql/api/user/v1"
	"ai-chat-sql/internal/logic/uiap"
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

func (c *ControllerV1) UiapCallback(ctx context.Context, req *v1.UiapCallbackReq) (res *v1.UiapCallbackRes, err error) {
	cfg, ok := uiap.EnabledConfigFromSystem()
	if !ok {
		return nil, fmt.Errorf("uiap is not enabled or config is incomplete")
	}
	client := uiap.NewClient(cfg)
	token, err := client.ExchangeCode(ctx, req.Code, req.RedirectUri)
	if err != nil {
		return nil, err
	}
	info, err := client.UserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	permission, err := client.Permissions(ctx, token.AccessToken, uiap.PermissionQuery{
		UserId:                      info.UserId,
		Username:                    info.Username,
		IncludeTopicPermissions:     true,
		IncludeKnowledgePermissions: true,
		IncludeDocumentPermissions:  true,
	})
	if err != nil {
		return nil, err
	}
	userId, err := uiap.ApplyUserPermissions(ctx, info, permission)
	if err != nil {
		return nil, err
	}
	out, err := service.User().GenJwtTokenByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}
	user, err := service.User().GetUserInfoById(ctx, userId)
	if err != nil {
		return nil, err
	}
	res = &v1.UiapCallbackRes{}
	res.JWTGenTokenOutput = *out
	if err = gconv.Scan(user, &res.User); err != nil {
		return nil, err
	}
	res.RuleLevel = effectiveRuleLevel(ctx, userId, user.RuleLevel)
	now := int(gtime.Timestamp())
	_, _ = g.DB("master").Model("user").Ctx(ctx).Where("user_id = ?", userId).Data(g.Map{
		"last_login_tme": now,
		"update_time":    now,
	}).Update()
	_, _ = g.DB("master").Model("admin_operation_log").Ctx(ctx).Data(g.Map{
		"log_type":    "system",
		"username":    user.Username,
		"action_type": "UIAP登录",
		"content":     "UIAP统一身份认证登录问数平台",
		"result":      "success",
		"create_time": now,
	}).Insert()
	return res, nil
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
		return &v1.UserPermissionsRes{Authenticated: false, RuleLevel: 0, Permissions: topicItemsFromRuleLevel(0, false), QaPermissions: knowledgePermissionsForUser(ctx, 0, false)}, nil
	}

	user, err := service.User().GetUserInfoById(ctx, userId)
	if err != nil || user == nil {
		return &v1.UserPermissionsRes{Authenticated: false, RuleLevel: 0, Permissions: topicItemsFromRuleLevel(0, false), QaPermissions: knowledgePermissionsForUser(ctx, 0, false)}, nil
	}
	snapshot := loadUserPermissionSnapshot(ctx, userId, user.RuleLevel)
	return &v1.UserPermissionsRes{
		Authenticated:     true,
		UserId:            userId,
		Username:          user.Username,
		RuleLevel:         snapshot.RuleLevel,
		PermissionVersion: snapshot.PermissionVersion,
		Permissions:       topicItemsFromRuleLevel(snapshot.RuleLevel, false),
		QaPermissions:     knowledgePermissionsForUser(ctx, userId, false),
	}, nil
}

func (c *ControllerV1) UserBootstrap(ctx context.Context, req *v1.UserBootstrapReq) (res *v1.UserBootstrapRes, err error) {
	userId := currentUserId(ctx)
	if userId <= 0 {
		return bootstrapGuest(ctx), nil
	}
	cacheKey := fmt.Sprintf("user_bootstrap:%d", userId)
	cached, err := consts.Cache.GetOrSetFunc(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return buildUserBootstrap(ctx, userId)
	}, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if out, ok := cached.Interface().(*v1.UserBootstrapRes); ok {
		return out, nil
	}
	return buildUserBootstrap(ctx, userId)
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
	return &v1.UserKnowledgeBasesRes{List: knowledgePermissionsForUser(ctx, currentUserId(ctx), true)}, nil
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

func bootstrapGuest(ctx context.Context) *v1.UserBootstrapRes {
	topicPermissions := topicItemsFromRuleLevel(0, false)
	return &v1.UserBootstrapRes{
		Authenticated:     false,
		RuleLevel:         0,
		PermissionVersion: 0,
		TopicPermissions:  topicPermissions,
		Topics:            []model.TopicItem{},
		QaPermissions:     knowledgePermissionsForUser(ctx, 0, false),
		KnowledgeBases:    []v1.UserKnowledgePermission{},
		PopularQuestions:  []v1.UserPopularQuestionItem{},
		AlertSummary:      emptyAlertSummary(),
	}
}

func buildUserBootstrap(ctx context.Context, userId int64) (*v1.UserBootstrapRes, error) {
	user, err := service.User().GetUserInfoById(ctx, userId)
	if err != nil || user == nil {
		return bootstrapGuest(ctx), nil
	}
	var outUser model.User
	if err = gconv.Scan(user, &outUser); err != nil {
		return nil, err
	}
	snapshot := loadUserPermissionSnapshot(ctx, userId, user.RuleLevel)
	topicPermissions := topicItemsFromRuleLevel(snapshot.RuleLevel, false)
	topics := enabledTopics(topicPermissions)
	qaPermissions := knowledgePermissionsForUser(ctx, userId, false)
	knowledgeBases := enabledKnowledgeBases(qaPermissions)
	popularQuestions := popularQuestionsForUser(ctx, userId, 3)
	alertSummary := alertSummaryForUser(ctx, int(userId), 5)
	return &v1.UserBootstrapRes{
		Authenticated:     true,
		User:              &outUser,
		RuleLevel:         snapshot.RuleLevel,
		PermissionVersion: snapshot.PermissionVersion,
		TopicPermissions:  topicPermissions,
		Topics:            topics,
		QaPermissions:     qaPermissions,
		KnowledgeBases:    knowledgeBases,
		PopularQuestions:  popularQuestions,
		AlertSummary:      alertSummary,
	}, nil
}

func enabledTopics(items []model.TopicItem) []model.TopicItem {
	list := make([]model.TopicItem, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			list = append(list, item)
		}
	}
	return list
}

func enabledKnowledgeBases(items []v1.UserKnowledgePermission) []v1.UserKnowledgePermission {
	list := make([]v1.UserKnowledgePermission, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			list = append(list, item)
		}
	}
	return list
}

func popularQuestionsForUser(ctx context.Context, userId int64, limit int) []v1.UserPopularQuestionItem {
	if limit <= 0 {
		limit = 3
	}
	username := usernameById(ctx, userId)
	if username == "" {
		return []v1.UserPopularQuestionItem{}
	}
	records, err := g.DB("master").Model("admin_operation_log").Ctx(ctx).
		Fields("content, COUNT(1) AS count").
		Where("username = ? AND action_type = ? AND result = ? AND content <> ?", username, "闂瓟鏌ヨ", "success", "").
		Group("content").
		OrderDesc("count").
		Limit(limit).
		All()
	if err != nil {
		return []v1.UserPopularQuestionItem{}
	}
	list := make([]v1.UserPopularQuestionItem, 0, len(records))
	for _, record := range records {
		question := record["content"].String()
		if question == "" {
			continue
		}
		list = append(list, v1.UserPopularQuestionItem{Question: question, Count: record["count"].Int()})
	}
	return list
}

func alertSummaryForUser(ctx context.Context, userId int, limit int) v1.UserAlertSummary {
	if userId <= 0 {
		return emptyAlertSummary()
	}
	alerts, err := service.Alert().GetAlertList(ctx, userId, "")
	if err != nil {
		return emptyAlertSummary()
	}
	summary := emptyAlertSummary()
	if limit <= 0 {
		limit = 5
	}
	for _, item := range alerts {
		summary.Total++
		summary.ByTopic[item.Topic]++
		switch strings.ToLower(item.Level) {
		case "critical":
			summary.Critical++
		default:
			summary.Warning++
		}
		if len(summary.Latest) < limit {
			summary.Latest = append(summary.Latest, item)
		}
	}
	return summary
}

func emptyAlertSummary() v1.UserAlertSummary {
	return v1.UserAlertSummary{
		ByTopic: map[string]int{},
		Latest:  []model.AlertItem{},
	}
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
		{Label: "网格", Value: "grid", Code: "grid", Name: "网格", Description: "案件总量、结案率、社区排名、重大案件分析", Icon: "grid", Sort: 1, Permission: 1},
		{Label: "人流", Value: "population", Code: "population", Name: "人流", Description: "区域人流、峰值时段、趋势分析、流动人口", Icon: "users", Sort: 2, Permission: 2},
		{Label: "车流", Value: "traffic", Code: "traffic", Name: "车流", Description: "关口车流、港澳车、外地车、停留时长", Icon: "car", Sort: 3, Permission: 4},
	}

	list := make([]model.TopicItem, 0, len(allTopics))
	for _, topic := range allTopics {
		topic.Enabled = ruleLevel&topic.Permission != 0
		if topic.Enabled {
			topic.PermissionStatus = "allowed"
		} else {
			topic.PermissionStatus = "denied"
		}
		if enabledOnly && !topic.Enabled {
			continue
		}
		list = append(list, topic)
	}
	return list
}

func knowledgePermissionsForUser(ctx context.Context, userId int64, enabledOnly bool) []v1.UserKnowledgePermission {
	bases, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return []v1.UserKnowledgePermission{}
	}

	enabledByCode := make(map[string]bool, len(bases))
	if userId > 0 {
		perms, _ := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).All()
		for _, perm := range perms {
			enabledByCode[normalizeQaKnowledgeCode(perm["knowledge_code"].String())] = perm["enabled"].Int() != 0
		}
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

func normalizeQaKnowledgeCode(code string) string {
	switch strings.TrimSpace(code) {
	case "policy_files":
		return "policy"
	case "laws":
		return "manual"
	default:
		return strings.TrimSpace(code)
	}
}

type userPermissionSnapshot struct {
	RuleLevel         int
	PermissionVersion int
}

func loadUserPermissionSnapshot(ctx context.Context, userId int64, fallback int) userPermissionSnapshot {
	if userId <= 0 {
		return userPermissionSnapshot{RuleLevel: 0, PermissionVersion: 0}
	}
	cacheKey := fmt.Sprintf("user_permission:%d", userId)
	cached, err := consts.Cache.GetOrSetFunc(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return queryUserPermissionSnapshot(ctx, userId, fallback), nil
	}, 5*time.Second)
	if err == nil {
		if snapshot, ok := cached.Interface().(userPermissionSnapshot); ok {
			return snapshot
		}
		if snapshot, ok := cached.Interface().(*userPermissionSnapshot); ok && snapshot != nil {
			return *snapshot
		}
	}
	return queryUserPermissionSnapshot(ctx, userId, fallback)
}

func queryUserPermissionSnapshot(ctx context.Context, userId int64, fallback int) userPermissionSnapshot {
	record, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).
		Fields("enabled, rule_level, permission_version").
		Where("user_id = ?", userId).
		One()
	if err != nil || record == nil {
		return userPermissionSnapshot{RuleLevel: fallback, PermissionVersion: 1}
	}
	version := record["permission_version"].Int()
	if version <= 0 {
		version = 1
	}
	if record["enabled"].Val() != nil && record["enabled"].Int() == 0 {
		return userPermissionSnapshot{RuleLevel: 0, PermissionVersion: version}
	}
	ruleLevel := fallback
	if record["rule_level"].Val() != nil {
		ruleLevel = record["rule_level"].Int()
	}
	return userPermissionSnapshot{RuleLevel: ruleLevel, PermissionVersion: version}
}

func effectiveRuleLevel(ctx context.Context, userId int64, fallback int) int {
	return loadUserPermissionSnapshot(ctx, userId, fallback).RuleLevel
}
