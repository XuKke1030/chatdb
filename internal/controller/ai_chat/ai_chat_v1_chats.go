package ai_chat

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "ai-chat-sql/api/ai_chat/v1"
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gogf/gf/v2/util/grand"
)

// topicPermissionMap 主题权限位掩码映射
var topicPermissionMap = map[string]int{
	"grid":       1, // bit 0
	"population": 2, // bit 1
	"traffic":    4, // bit 2
}

// checkTopicPermission 检查用户是否有指定主题的权限
func checkTopicPermission(ruleLevel int, topic string) bool {
	if topic == "" {
		return true // 未指定主题时不限制
	}
	bit, ok := topicPermissionMap[topic]
	if !ok {
		return false // 未知主题
	}
	return ruleLevel&bit != 0
}

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// 获取用户ID和权限
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}
	username := usernameByUserId(ctx, userId)

	// 校验主题权限
	if req.Topic != "" && userId > 0 {
		// 查询用户权限等级
		user, err := service.User().GetUserInfoById(ctx, int64(userId))
		if err != nil {
			return nil, err
		}
		ruleLevel := effectiveTopicRuleLevel(ctx, userId, user.RuleLevel)
		if !checkTopicPermission(ruleLevel, req.Topic) {
			insertSystemQueryLog(ctx, username, req.Topic, req.Message, "failed")
			r := g.RequestFromCtx(ctx)
			r.Response.WriteStatus(403)
			r.Response.WriteJson(g.Map{"code": 403, "message": "没有该主题的访问权限"})
			return nil, nil
		}
	}
	if req.Topic == "" && req.KnowledgeCode != "" {
		if !checkKnowledgePermission(ctx, req.KnowledgeCode) {
			insertSystemQueryLog(ctx, username, req.Topic, req.Message, "failed")
			r := g.RequestFromCtx(ctx)
			r.Response.WriteStatus(403)
			r.Response.WriteJson(g.Map{"code": 403, "message": "该知识库不可用"})
			return nil, nil
		}
	}
	insertSystemQueryLog(ctx, username, req.Topic, req.Message, "success")

	respChan := make(chan any)
	g.Go(ctx, func(ctx context.Context) {
		service.AiChat().Chat(ctx, req.ChatInput, respChan)
	}, func(ctx context.Context, exception error) {
		close(respChan)
		consts.Logger.Error(ctx, exception)
	})
	r := g.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")

	var jsonData []byte
	for v := range respChan {
		switch v.(type) {
		case string:
			r.Response.Writef("%s\n\n", v)
		case error:
			r.Response.Writef("data: %s\n\n", gjson.MustEncodeString(g.Map{"error": fmt.Sprintf("%s", v)}))
		default:
			jsonData, err = json.Marshal(v)
			if err != nil {
				return
			}
			r.Response.Writef("data: %s\n\n", jsonData)
		}
		r.Response.Flush()
	}
	return
}

func effectiveTopicRuleLevel(ctx context.Context, userId int, fallback int) int {
	record, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).One()
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

func checkKnowledgePermission(ctx context.Context, knowledgeCode string) bool {
	if knowledgeCode == "" {
		return false
	}
	base, err := g.DB("master").Model("admin_knowledge_base").Ctx(ctx).
		Where("code = ? AND enabled = ?", knowledgeCode, 1).
		One()
	return err == nil && base != nil
}

func usernameByUserId(ctx context.Context, userId int) string {
	if userId <= 0 {
		return "anonymous"
	}
	record, err := g.DB("master").Model("user").Ctx(ctx).Fields("username").Where("user_id = ?", userId).One()
	if err != nil || record == nil {
		return fmt.Sprintf("user_%d", userId)
	}
	return record["username"].String()
}

func insertSystemQueryLog(ctx context.Context, username string, topic string, message string, result string) {
	actionType := "问答查询"
	if topic != "" {
		actionType = "问数查询"
	}
	_, _ = g.DB("master").Model("admin_operation_log").Ctx(ctx).Data(g.Map{
		"log_type":    "system",
		"username":    username,
		"action_type": actionType,
		"content":     message,
		"result":      result,
		"create_time": int(gtime.Timestamp()),
	}).Insert()
}

var topicDefaultSuggestions = map[string]struct {
	questions   []string
	placeholder string
}{
	"grid": {
		questions:   []string{"本月高新增区案终结率是多少？", "本月各网格案件分布情况", "近一周新增案件趋势如何？"},
		placeholder: "输入网格相关问题...",
	},
	"population": {
		questions:   []string{"过去一周人流进出趋势如何？", "今日各区域人流量对比", "人流高峰时段分布"},
		placeholder: "输入人流相关问题...",
	},
	"traffic": {
		questions:   []string{"今日港珠澳车辆占比是多少？", "各卡口车流量排名", "本周车流变化趋势"},
		placeholder: "输入车流相关问题...",
	},
}

func (c *ControllerV1) ChatSessionCreate(ctx context.Context, req *v1.ChatSessionCreateReq) (res *v1.ChatSessionCreateRes, err error) {
	userId := gconv.Int(ctx.Value(model.UserGroup{}))
	now := int(gtime.Timestamp())
	sessionId := fmt.Sprintf("sess_%d_%d", now, grand.N(100000, 999999))

	suggestions := topicDefaultSuggestions[req.Topic]
	suggestedQuestions := suggestions.questions
	if len(suggestedQuestions) == 0 {
		suggestedQuestions = []string{}
	}
	inputPlaceholder := suggestions.placeholder
	if inputPlaceholder == "" {
		inputPlaceholder = "输入您的问题..."
	}

	_, err = g.DB("master").Model("chat_session").Ctx(ctx).Data(g.Map{
		"session_id":          sessionId,
		"user_id":             userId,
		"topic":               req.Topic,
		"source":              req.Source,
		"alert_id":            req.AlertId,
		"suggested_questions": gjson.MustEncodeString(suggestedQuestions),
		"input_placeholder":   inputPlaceholder,
		"create_time":         now,
		"update_time":         now,
	}).Insert()
	if err != nil {
		return
	}

	return &v1.ChatSessionCreateRes{
		SessionId:          sessionId,
		SuggestedQuestions: suggestedQuestions,
		InputPlaceholder:   inputPlaceholder,
	}, nil
}

func (c *ControllerV1) ChatSessionReset(ctx context.Context, req *v1.ChatSessionResetReq) (res *v1.ChatSessionResetRes, err error) {
	now := int(gtime.Timestamp())
	_, err = g.DB("master").Model("chat_session").Ctx(ctx).
		Where("session_id = ?", req.Id).
		Data(g.Map{"update_time": now}).
		Update()
	if err != nil {
		return
	}
	return &v1.ChatSessionResetRes{}, nil
}
