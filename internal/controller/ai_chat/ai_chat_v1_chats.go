package ai_chat

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "ai-chat-sql/api/ai_chat/v1"
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
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
	totalStart := time.Now()
	stageStart := totalStart
	logStage := func(stage string) {
		consts.Logger.Infof(ctx, "perf ask_number stage=%s topic=%s sessionId=%s costMs=%d totalMs=%d", stage, req.Topic, req.SessionId, time.Since(stageStart).Milliseconds(), time.Since(totalStart).Milliseconds())
		stageStart = time.Now()
	}
	defer func() {
		consts.Logger.Infof(ctx, "perf ask_number stage=total topic=%s sessionId=%s costMs=%d", req.Topic, req.SessionId, time.Since(totalStart).Milliseconds())
	}()

	// 获取用户ID和权限
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}
	username := usernameByUserId(ctx, userId)
	logStage("identity")

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
	logStage("permission")
	insertSystemQueryLog(ctx, username, req.Topic, req.Message, "success")
	logStage("operation_log")

	sessionId, err := ensureAskNumberSession(ctx, userId, req.SessionId, req.Topic, req.DatabaseId, req.Message)
	if err != nil {
		return nil, err
	}
	req.SessionId = sessionId
	logStage("session")
	storedHistory, err := loadAskNumberSessionHistory(ctx, userId, sessionId, 12)
	if err != nil {
		return nil, err
	}
	req.History = mergeChatHistory(storedHistory, req.History)
	logStage("history")
	if err = appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "user", req.Message); err != nil {
		return nil, err
	}
	logStage("save_user_message")

	// Fast path: bypass AI SQL generation for high-frequency questions.
	if intent, ok := NewFastPathRouter().Match(req); ok {
		return c.streamFastPathIntentAnswer(ctx, intent, req, userId, sessionId)
	}

	if req.Topic == "grid" && isMajorCaseQuestion(req.Message) {
		return c.streamGridMajorCaseAnswer(ctx, req, userId, sessionId)
	}

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
	logStage("first_sse_ready")

	var jsonData []byte
	var assistantBuilder strings.Builder
	firstTokenLogged := false
	for v := range respChan {
		switch item := v.(type) {
		case string:
			r.Response.Writef("%s\n\n", item)
		case error:
			r.Response.Writef("data: %s\n\n", gjson.MustEncodeString(g.Map{"error": fmt.Sprintf("%s", item)}))
		default:
			if out, ok := item.(model.ChatOutDataItem); ok {
				if out.Event == "message" && out.Content != "" && (out.Role == "" || strings.EqualFold(out.Role, "assistant")) {
					if !firstTokenLogged {
						firstTokenLogged = true
						consts.Logger.Infof(ctx, "perf ask_number stage=first_token topic=%s sessionId=%s totalMs=%d", req.Topic, sessionId, time.Since(totalStart).Milliseconds())
					}
					assistantBuilder.WriteString(out.Content)
				}
				if out.Event == "end" && assistantBuilder.Len() > 0 {
					if saveErr := appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "assistant", assistantBuilder.String()); saveErr != nil {
						consts.Logger.Errorf(ctx, "保存问数会话回复失败: %s", saveErr.Error())
					}
				}
			}
			jsonData, err = json.Marshal(item)
			if err != nil {
				return
			}
			r.Response.Writef("data: %s\n\n", jsonData)
		}
		r.Response.Flush()
	}
	return
}

func (c *ControllerV1) ChatSessionReset(ctx context.Context, req *v1.ChatSessionResetReq) (res *v1.ChatSessionResetRes, err error) {
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}
	if userId <= 0 {
		return nil, gerror.New("用户未登录")
	}
	count, err := g.DB("master").Model("ask_number_session").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gerror.New("会话不存在或无权操作")
	}
	now := int(gtime.Timestamp())
	if _, err = g.DB("master").Model("ask_number_message").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Delete(); err != nil {
		return nil, err
	}
	if _, err = g.DB("master").Model("ask_number_session").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", req.SessionId, userId).
		Data(g.Map{"status": "reset", "update_time": now}).
		Update(); err != nil {
		return nil, err
	}
	return &v1.ChatSessionResetRes{SessionId: req.SessionId}, nil
}

func (c *ControllerV1) GridMajorCaseAnalysis(ctx context.Context, req *v1.GridMajorCaseAnalysisReq) (res *v1.GridMajorCaseAnalysisRes, err error) {
	userIdVal := ctx.Value(model.UserGroup{})
	userId := 0
	if userIdVal != nil {
		userId = gconv.Int(userIdVal)
	}
	if userId > 0 {
		user, userErr := service.User().GetUserInfoById(ctx, int64(userId))
		if userErr != nil {
			return nil, userErr
		}
		ruleLevel := effectiveTopicRuleLevel(ctx, userId, user.RuleLevel)
		if !checkTopicPermission(ruleLevel, "grid") {
			return nil, gerror.New("没有网格主题访问权限")
		}
	}
	return analyzeGridMajorCase(ctx, req.Metric, req.Region)
}

func (c *ControllerV1) streamGridMajorCaseAnswer(ctx context.Context, req *v1.ChatReq, userId int, sessionId string) (*v1.ChatRes, error) {
	analysis, err := analyzeGridMajorCase(ctx, majorCaseMetricFromQuestion(req.Message), "")
	if err != nil {
		return nil, err
	}
	answer := formatMajorCaseAnswer(analysis)
	r := g.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "start", Data: g.Map{"sessionId": sessionId}})
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "major_case", Data: analysis})
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "message", Role: "assistant", Content: answer})
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "end", Data: g.Map{"sessionId": sessionId}})
	if err = appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "assistant", answer); err != nil {
		consts.Logger.Errorf(ctx, "保存重大案件问数回复失败: %s", err.Error())
	}
	return &v1.ChatRes{}, nil
}

func writeChatSSE(ctx context.Context, r *ghttp.Request, item model.ChatOutDataItem) {
	data, err := json.Marshal(model.GenChatOutDataItem(ctx, item))
	if err != nil {
		return
	}
	r.Response.Writef("data: %s\n\n", data)
	r.Response.Flush()
}

func ensureAskNumberSession(ctx context.Context, userId int, sessionId string, topic string, databaseId int, firstMessage string) (string, error) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		sessionId = newAskNumberSessionId()
	}
	db := g.DB("master")
	count, err := db.Model("ask_number_session").Ctx(ctx).
		Where("session_id = ?", sessionId).
		Count()
	if err != nil {
		return "", err
	}
	now := int(gtime.Timestamp())
	title := strings.TrimSpace(firstMessage)
	if len([]rune(title)) > 30 {
		title = string([]rune(title)[:30])
	}
	if count == 0 {
		_, err = db.Model("ask_number_session").Ctx(ctx).Data(g.Map{
			"session_id":  sessionId,
			"user_id":     userId,
			"topic":       topic,
			"database_id": databaseId,
			"title":       title,
			"status":      "active",
			"create_time": now,
			"update_time": now,
		}).Insert()
		return sessionId, err
	}
	record, err := db.Model("ask_number_session").Ctx(ctx).
		Fields("user_id").
		Where("session_id = ?", sessionId).
		One()
	if err != nil {
		return "", err
	}
	if record != nil && record["user_id"].Int() != userId {
		return "", gerror.New("会话不存在或无权访问")
	}
	_, err = db.Model("ask_number_session").Ctx(ctx).
		Where("session_id = ?", sessionId).
		Data(g.Map{
			"topic":       topic,
			"database_id": databaseId,
			"status":      "active",
			"update_time": now,
		}).
		Update()
	return sessionId, err
}

func loadAskNumberSessionHistory(ctx context.Context, userId int, sessionId string, limit int) ([]model.ChatHistoryItem, error) {
	if limit <= 0 {
		limit = 12
	}
	records, err := g.DB("master").Model("ask_number_message").Ctx(ctx).
		Fields("role, content").
		Where("session_id = ? AND user_id = ?", sessionId, userId).
		OrderDesc("id").
		Limit(limit).
		All()
	if err != nil {
		return nil, err
	}
	history := make([]model.ChatHistoryItem, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		role := records[i]["role"].String()
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(records[i]["content"].String())
		if content == "" {
			continue
		}
		history = append(history, model.ChatHistoryItem{Role: role, Content: content})
	}
	return history, nil
}

func appendAskNumberMessage(ctx context.Context, userId int, sessionId string, topic string, role string, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	now := int(gtime.Timestamp())
	_, err := g.DB("master").Model("ask_number_message").Ctx(ctx).Data(g.Map{
		"session_id":  sessionId,
		"user_id":     userId,
		"topic":       topic,
		"role":        role,
		"content":     content,
		"create_time": now,
	}).Insert()
	if err != nil {
		return err
	}
	_, err = g.DB("master").Model("ask_number_session").Ctx(ctx).
		Where("session_id = ? AND user_id = ?", sessionId, userId).
		Data(g.Map{"update_time": now}).
		Update()
	return err
}

func mergeChatHistory(stored []model.ChatHistoryItem, client []model.ChatHistoryItem) []model.ChatHistoryItem {
	merged := make([]model.ChatHistoryItem, 0, len(stored)+len(client))
	seen := make(map[string]bool)
	appendItems := func(items []model.ChatHistoryItem) {
		for _, item := range items {
			role := strings.TrimSpace(item.Role)
			content := strings.TrimSpace(item.Content)
			if content == "" || (role != "user" && role != "assistant") {
				continue
			}
			key := role + "\x00" + content
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, model.ChatHistoryItem{Role: role, Content: content})
		}
	}
	appendItems(stored)
	appendItems(client)
	if len(merged) > 20 {
		merged = merged[len(merged)-20:]
	}
	return merged
}

func CreateAskNumberSessionTables(ctx context.Context, db gdb.DB) error {
	sessionSQL := `CREATE TABLE IF NOT EXISTS ask_number_session (
session_id VARCHAR(64) PRIMARY KEY,
user_id INT NOT NULL,
topic VARCHAR(32),
database_id INT NOT NULL DEFAULT 0,
title VARCHAR(128),
status VARCHAR(32) NOT NULL DEFAULT 'active',
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_user_update (user_id, update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	messageSQL := `CREATE TABLE IF NOT EXISTS ask_number_message (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
session_id VARCHAR(64) NOT NULL,
user_id INT NOT NULL,
topic VARCHAR(32),
role VARCHAR(32) NOT NULL,
content LONGTEXT NOT NULL,
create_time INT NOT NULL,
INDEX idx_session_id (session_id),
INDEX idx_user_session (user_id, session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	if _, err := db.Exec(ctx, sessionSQL); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, messageSQL); err != nil {
		return err
	}
	return nil
}

func newAskNumberSessionId() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ask_%d", gtime.TimestampNano())
	}
	return fmt.Sprintf("ask_%x", buf)
}

func analyzeGridMajorCase(ctx context.Context, metric string, region string) (*v1.GridMajorCaseAnalysisRes, error) {
	metric = normalizeMajorCaseMetric(metric)
	query := g.DB("master").Model("case_list").Ctx(ctx)
	if strings.TrimSpace(region) != "" {
		query = query.WhereLike("region", "%"+strings.TrimSpace(region)+"%")
	}
	records, err := query.OrderDesc("id").All()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, gerror.New("暂无网格案件数据，请先在后台导入网格案件")
	}
	cases := make([]v1.MajorCaseItem, 0, len(records))
	for _, record := range records {
		item := majorCaseFromRecord(record)
		scoreMajorCase(&item)
		cases = append(cases, item)
	}
	sort.SliceStable(cases, func(i, j int) bool {
		switch metric {
		case "impact":
			if cases[i].ImpactScore != cases[j].ImpactScore {
				return cases[i].ImpactScore > cases[j].ImpactScore
			}
		case "difficulty":
			if cases[i].DifficultyScore != cases[j].DifficultyScore {
				return cases[i].DifficultyScore > cases[j].DifficultyScore
			}
		default:
			if cases[i].TotalScore != cases[j].TotalScore {
				return cases[i].TotalScore > cases[j].TotalScore
			}
		}
		return cases[i].Id > cases[j].Id
	})
	top := cases[0]
	basis := []string{
		fmt.Sprintf("影响度评分：%d，处置难度评分：%d，综合评分：%d。", top.ImpactScore, top.DifficultyScore, top.TotalScore),
	}
	basis = append(basis, top.Reasons...)
	return &v1.GridMajorCaseAnalysisRes{
		Metric: metric,
		Case:   top,
		Basis:  basis,
	}, nil
}

func majorCaseFromRecord(record gdb.Record) v1.MajorCaseItem {
	item := v1.MajorCaseItem{
		Id:                 record["id"].Int64(),
		CaseNumber:         record["case_number"].String(),
		CaseSource:         record["case_source"].String(),
		ReportTime:         record["report_time"].String(),
		PendingStep:        record["pending_step"].String(),
		CaseType:           record["case_type"].String(),
		Region:             record["region"].String(),
		ResponsibilityUnit: record["responsibility_unit"].String(),
		CaseLocation:       record["case_location"].String(),
		Description:        record["description"].String(),
	}
	item.CaseName = buildMajorCaseName(item)
	return item
}

func buildMajorCaseName(item v1.MajorCaseItem) string {
	parts := make([]string, 0, 3)
	if item.Region != "" {
		parts = append(parts, item.Region)
	}
	if item.CaseType != "" {
		parts = append(parts, item.CaseType)
	}
	if item.CaseLocation != "" {
		parts = append(parts, item.CaseLocation)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "-")
	}
	if item.CaseNumber != "" {
		return "案件" + item.CaseNumber
	}
	return "未命名网格案件"
}

func scoreMajorCase(item *v1.MajorCaseItem) {
	impact := 30
	difficulty := 30
	reasons := make([]string, 0, 8)
	text := strings.Join([]string{item.CaseSource, item.PendingStep, item.CaseType, item.Region, item.CaseLocation, item.Description}, " ")

	addImpact := func(score int, reason string) {
		impact += score
		reasons = append(reasons, fmt.Sprintf("影响度：%s（+%d）", reason, score))
	}
	addDifficulty := func(score int, reason string) {
		difficulty += score
		reasons = append(reasons, fmt.Sprintf("难度：%s（+%d）", reason, score))
	}

	if containsAny(text, []string{"媒体", "舆情", "督办", "上级", "领导"}) {
		addImpact(18, "来源或描述包含媒体/督办/上级关注")
	} else if containsAny(text, []string{"12345", "热线", "信访", "群众投诉", "投诉人", "投诉件"}) {
		addImpact(12, "来源或描述包含热线投诉、信访等公众反馈")
	}
	if containsAny(text, []string{"地基下沉", "路面塌陷", "塌陷", "沉降", "裂缝", "危房", "坍塌", "结构安全"}) {
		addImpact(36, "涉及地基、塌陷、裂缝或建筑结构安全等高危隐患")
	} else if containsAny(text, []string{"安全", "消防", "燃气", "漏电", "污染", "群体"}) {
		addImpact(18, "涉及公共安全、民生或群体性影响关键词")
	} else if containsAny(text, []string{"道路", "垃圾", "噪音", "占道", "违建", "市容", "积水"}) {
		addImpact(10, "涉及高频民生治理问题")
	}
	if containsAny(text, []string{"小区", "居民", "住宅", "楼栋", "物业", "建筑物", "房屋"}) {
		addImpact(14, "涉及住宅小区、居民或建筑物，影响人群和安全面更广")
	}
	if item.Region != "" {
		addImpact(4, "案件已定位到具体所属区域，具备网格处置影响面")
	}

	if containsAny(text, []string{"重复", "反复", "多次", "持续"}) {
		addDifficulty(18, "存在重复、反复或持续性表述")
	}
	if containsAny(text, []string{"协调", "多部门", "权属", "历史遗留", "疑难", "无法", "困难", "拒不整改"}) {
		addDifficulty(18, "涉及协调、权属、历史遗留或整改阻力")
	}
	if containsAny(item.PendingStep, []string{"待办", "处置", "派遣", "处理中", "未处理"}) {
		addDifficulty(10, "当前仍处于待办/处置链路")
	} else if containsAny(item.PendingStep, []string{"结案", "办结"}) {
		difficulty -= 6
		reasons = append(reasons, "难度：案件已办结，处置难度扣减（-6）")
	}
	if days := majorCaseAgeDays(item.ReportTime); days >= 30 {
		addDifficulty(16, fmt.Sprintf("上报已超过%d天", days))
	} else if days >= 7 {
		addDifficulty(8, fmt.Sprintf("上报已超过%d天", days))
	}
	if len([]rune(item.Description)) >= 60 {
		addDifficulty(6, "问题描述较长，可能涉及更复杂的现场情况")
	}

	item.ImpactScore = clampScore(impact)
	item.DifficultyScore = clampScore(difficulty)
	item.TotalScore = clampScore(item.ImpactScore*55/100 + item.DifficultyScore*45/100)
	item.Reasons = reasons
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func majorCaseAgeDays(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"2006/01/02",
	}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			days := int(time.Since(parsed).Hours() / 24)
			if days > 0 {
				return days
			}
			return 0
		}
	}
	return 0
}

func normalizeMajorCaseMetric(metric string) string {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "impact", "influence", "影响", "影响度":
		return "impact"
	case "difficulty", "hard", "难度", "处置难度":
		return "difficulty"
	default:
		return "combined"
	}
}

func majorCaseMetricFromQuestion(question string) string {
	if containsAny(question, []string{"影响最大", "影响度最大", "影响最高"}) {
		return "impact"
	}
	if containsAny(question, []string{"难度最大", "最难", "处置难度最大", "处置最难"}) {
		return "difficulty"
	}
	return "combined"
}

func isMajorCaseQuestion(question string) bool {
	question = strings.TrimSpace(question)
	if question == "" {
		return false
	}
	return containsAny(question, []string{"重大案件", "重点案件", "影响最大", "难度最大", "处置最难", "最难处理"})
}

func formatMajorCaseAnswer(res *v1.GridMajorCaseAnalysisRes) string {
	if res == nil {
		return "暂无可分析的网格案件。"
	}
	item := res.Case
	lines := []string{
		fmt.Sprintf("重大案件：%s", item.CaseName),
		"",
		fmt.Sprintf("案件编号：%s", emptyText(item.CaseNumber, "暂无")),
		fmt.Sprintf("所属区域：%s", emptyText(item.Region, "暂无")),
		fmt.Sprintf("案件类型：%s", emptyText(item.CaseType, "暂无")),
		fmt.Sprintf("当前环节：%s", emptyText(item.PendingStep, "暂无")),
		fmt.Sprintf("综合评分：%d（影响度%d，处置难度%d）", item.TotalScore, item.ImpactScore, item.DifficultyScore),
		"",
		"判断依据：",
	}
	for _, reason := range res.Basis {
		lines = append(lines, "- "+reason)
	}
	if item.Description != "" {
		lines = append(lines, "", "问题描述："+item.Description)
	}
	return strings.Join(lines, "\n")
}

func emptyText(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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

// ---- Fast path for high-frequency ask_number questions ----

type fastPathKind int

const (
	fpTrafficToday          fastPathKind = iota // 今日车流汇总
	fpTrafficTopGateToday                       // 今日车流最大卡口
	fpTrafficWeek                               // 近七天车流趋势
	fpTrafficHkMacau                            // 港澳车占比
	fpTrafficGateRank                           // 卡口排名
	fpTrafficWeekendCompare                     // 周末和平日对比
	fpTrafficHolidayCompare                     // 节假日和平日对比
	fpTrafficForeignOrigin                      // 外地车来源排名
	fpPopWeekTrend                              // 近七天进出趋势
	fpPopHolidayCompare                         // 节假日对比
	fpPopRegionRank                             // 区域人流排名
	fpGridCaseCount                             // 案件数量
	fpGridCloseRate                             // 结案率
	fpGridRegionRank                            // 区域排名
)

type fastPath struct {
	kind  fastPathKind
	topic string
}

func matchFastPath(topic, question string) *fastPath {
	q := strings.TrimSpace(question)
	if q == "" {
		return nil
	}

	switch topic {
	case "traffic":
		if containsAny(q, []string{"哪个地方车流大", "哪里车流大", "哪个卡口车最多", "今天哪个卡口车最多", "车流最大", "车最多"}) {
			return &fastPath{kind: fpTrafficTopGateToday, topic: topic}
		}
		if containsAny(q, []string{"今日车流", "今天车流", "今日车辆", "今天车流量", "当前车流"}) {
			return &fastPath{kind: fpTrafficToday, topic: topic}
		}
		if containsAny(q, []string{"近七", "近7", "最近七天", "最近7天", "一周车流", "7天车流", "车流趋势"}) {
			return &fastPath{kind: fpTrafficWeek, topic: topic}
		}
		if containsAny(q, []string{"港澳", "港车", "澳车", "港澳车占比", "港澳车比例"}) {
			return &fastPath{kind: fpTrafficHkMacau, topic: topic}
		}
		if containsAny(q, []string{"卡口排名", "卡口排行", "卡口最多", "卡口最忙", "车流排名", "车流量排名", "排名前十"}) {
			return &fastPath{kind: fpTrafficGateRank, topic: topic}
		}
		if containsAny(q, []string{"节假日", "节日", "假日", "节假日和平日", "节假日车流", "假期车流"}) {
			return &fastPath{kind: fpTrafficHolidayCompare, topic: topic}
		}
		if containsAny(q, []string{"周末", "平日", "工作日", "周末和平日", "周末车流", "工作日车流"}) {
			return &fastPath{kind: fpTrafficWeekendCompare, topic: topic}
		}
		if containsAny(q, []string{"外地车", "外地来源", "外地车来源", "外省车", "来源排名", "来源地"}) {
			return &fastPath{kind: fpTrafficForeignOrigin, topic: topic}
		}
	case "population":
		if containsAny(q, []string{"近七", "近7", "最近七天", "最近7天", "一周人流", "7天人流", "进出趋势", "人流趋势"}) {
			return &fastPath{kind: fpPopWeekTrend, topic: topic}
		}
		if containsAny(q, []string{"节假日", "节日", "假期对比", "假期人流", "假日"}) {
			return &fastPath{kind: fpPopHolidayCompare, topic: topic}
		}
		if containsAny(q, []string{"区域排名", "区域人流", "排行", "最多人", "人流最多"}) {
			return &fastPath{kind: fpPopRegionRank, topic: topic}
		}
	case "grid":
		if containsAny(q, []string{"案件数量", "有多少案件", "案件数", "案件总数"}) {
			return &fastPath{kind: fpGridCaseCount, topic: topic}
		}
		if containsAny(q, []string{"结案率", "办结率", "结案比例"}) {
			return &fastPath{kind: fpGridCloseRate, topic: topic}
		}
		if containsAny(q, []string{"区域排名", "区域案件", "排行", "最多案件"}) {
			return &fastPath{kind: fpGridRegionRank, topic: topic}
		}
	}
	return nil
}

func executeFastPath(ctx context.Context, fp *fastPath) (answer string, chartData string, err error) {
	switch fp.kind {
	case fpTrafficToday:
		return fastTrafficToday(ctx)
	case fpTrafficTopGateToday:
		return fastTrafficTopGateToday(ctx)
	case fpTrafficWeek:
		return fastTrafficWeek(ctx)
	case fpTrafficHkMacau:
		return fastTrafficHkMacau(ctx)
	case fpTrafficGateRank:
		return fastTrafficGateRank(ctx)
	case fpTrafficWeekendCompare:
		return fastTrafficWeekendCompare(ctx)
	case fpTrafficHolidayCompare:
		return fastTrafficHolidayCompare(ctx)
	case fpTrafficForeignOrigin:
		return fastTrafficForeignOrigin(ctx)
	case fpPopWeekTrend, fpPopHolidayCompare, fpPopRegionRank:
		return fastPopQuery(ctx, fp.kind)
	case fpGridCaseCount, fpGridCloseRate, fpGridRegionRank:
		return fastGridQuery(ctx, fp.kind)
	}
	return "暂不支持该问题的快速查询。", "", nil
}

func formatFastPathAnswer(fp *fastPath, conclusion string, chartData string) string {
	conclusion = strings.TrimSpace(conclusion)
	if conclusion == "" {
		conclusion = "当前未查询到符合条件的数据。"
	}
	parts := []string{
		"## 精准结论",
		conclusion,
		"",
		"## 特征洞察",
		fastPathFeatureText(fp, chartData != ""),
		"",
	}
	if strings.TrimSpace(chartData) != "" {
		parts = append(parts,
			"## 可视化",
			strings.TrimSpace(chartData),
			"",
		)
	}
	parts = append(parts,
		"## 洞察分析",
		"### 关键 / 异常点",
		fastPathKeyPointText(fp),
		"",
		"### 业务影响",
		fastPathImpactText(fp),
		"",
		"### 优化建议",
		fastPathSuggestionText(fp),
	)
	return strings.Join(parts, "\n")
}

func fastPathFeatureText(fp *fastPath, hasChart bool) string {
	switch fp.kind {
	case fpTrafficToday:
		return "统计口径为当天卡口通行记录，按进入、离开、港澳车等业务维度汇总。"
	case fpTrafficTopGateToday, fpTrafficGateRank:
		return "统计口径为当天卡口通行记录，按卡口车流量进行排名汇总。"
	case fpTrafficWeek:
		return "统计口径为近七天卡口通行记录，按日期进行汇总对比。"
	case fpTrafficHkMacau:
		return "统计口径为近七天卡口通行记录，按港澳车与其他车辆构成进行汇总。"
	case fpTrafficWeekendCompare:
		return "统计口径为最近一周周末与工作日的卡口通行记录，计算日均车流进行对比。"
	case fpTrafficHolidayCompare:
		return "统计口径为最近一个中国法定节假日假期与相邻工作日的卡口通行记录，计算日均车流进行对比。"
	case fpTrafficForeignOrigin:
		return "统计口径为近七天卡口通行记录，按车牌归属地类型进行排名汇总。"
	case fpPopWeekTrend, fpPopRegionRank:
		return "统计口径为近七天人流记录，按日期或区域维度汇总。"
	case fpPopHolidayCompare:
		return "统计口径为当前可识别周期与上一可比周期的人流记录对比；如需严格法定节假日口径，应接入节假日日历数据。"
	case fpGridCaseCount, fpGridCloseRate, fpGridRegionRank:
		return "统计口径为当前网格案件数据，按案件数量、办结状态或区域维度汇总。"
	default:
		if hasChart {
			return "本次结果基于当前可查询数据汇总，并提供图表辅助对比。"
		}
		return "本次结果基于当前可查询数据汇总。"
	}
}

func fastPathKeyPointText(fp *fastPath) string {
	switch fp.topic {
	case "traffic":
		return "- 重点关注车流总量、进出方向、卡口排名和港澳车占比变化。"
	case "population":
		return "- 重点关注人流总量、进出变化、区域差异和周期波动。"
	case "grid":
		return "- 重点关注案件规模、办结情况、区域分布和治理压力。"
	default:
		return "- 当前结果反映了所选口径下的主要业务变化。"
	}
}

func fastPathImpactText(fp *fastPath) string {
	switch fp.topic {
	case "traffic":
		return "- 可为交通疏导、口岸保障、卡口运行和设备运维提供参考。"
	case "population":
		return "- 可为公共服务保障、人员流动研判和重点区域调度提供参考。"
	case "grid":
		return "- 可为基层治理、案件督办和资源配置提供参考。"
	default:
		return "- 可辅助业务管理人员快速掌握当前运行态势。"
	}
}

func fastPathSuggestionText(fp *fastPath) string {
	switch fp.topic {
	case "traffic":
		return "- 建议持续关注高峰卡口和异常波动点位，必要时加强现场疏导和设备巡检。"
	case "population":
		return "- 建议关注人流集中的区域和时段，提前做好服务保障和秩序维护。"
	case "grid":
		return "- 建议对案件高发区域和未办结事项加强跟踪督办，复盘高发原因。"
	default:
		return "- 建议结合后续数据变化持续跟踪，并对异常项开展复核。"
	}
}

// ---- Traffic fast paths ----

func fastTrafficToday(ctx context.Context) (string, string, error) {
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: gtime.Now().Format("Y-m-d"),
		DateTo:   gtime.Now().Format("Y-m-d") + " 23:59:59",
		GroupBy:  "day",
	})
	if err != nil {
		return "", "", err
	}
	s := result.Summary
	answer := fmt.Sprintf("今日车流总计 %d 辆，其中进入 %d 辆，离开 %d 辆。港澳车 %d 辆，占比 %.1f%%。",
		s.Total, s.InCount, s.OutCount, s.HkMacauCount, s.HkMacauRatio*100)
	return answer, "", nil
}

func fastTrafficTopGateToday(ctx context.Context) (string, string, error) {
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: gtime.Now().Format("Y-m-d"),
		DateTo:   gtime.Now().Format("Y-m-d") + " 23:59:59",
		GroupBy:  "gate",
	})
	if err != nil {
		return "", "", err
	}
	if len(result.Series) == 0 {
		return "今日暂无卡口车流数据。", "", nil
	}
	xLabels, totalData, tableRows := trafficGateRankChartData(result.Series, 10)
	chart := buildChart("bar", "今日卡口车流排名（Top10）", xLabels,
		[]chartSeries{{Name: "车流量", Data: totalData}},
		[]string{"卡口", "合计", "进入", "离开"}, tableRows)
	answer := fmt.Sprintf("今日车流量最大的卡口为「%s」，共 %d 辆。", result.Series[0].Name, result.Series[0].Total)
	return answer, chart, nil
}

func fastTrafficWeek(ctx context.Context) (string, string, error) {
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "day",
	})
	if err != nil {
		return "", "", err
	}
	if len(result.Series) == 0 {
		return "近七天暂无车流数据。", "", nil
	}
	xLabels := make([]string, 0, len(result.Series))
	inData := make([]int, 0, len(result.Series))
	outData := make([]int, 0, len(result.Series))
	tableRows := make([][]any, 0, len(result.Series))
	for _, item := range result.Series {
		xLabels = append(xLabels, item.Name)
		inData = append(inData, item.InCount)
		outData = append(outData, item.OutCount)
		tableRows = append(tableRows, []any{item.Name, item.InCount, item.OutCount, item.Total})
	}
	s := result.Summary
	chart := buildChart("line", "近七天车流进出趋势", xLabels,
		[]chartSeries{{Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
		[]string{"日期", "进入", "离开", "合计"}, tableRows)
	answer := fmt.Sprintf("近七天车流总计 %d 辆，日均 %.0f 辆。港澳车占比 %.1f%%。",
		s.Total, float64(s.Total)/7, s.HkMacauRatio*100)
	return answer, chart, nil
}

func fastTrafficHkMacau(ctx context.Context) (string, string, error) {
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: gtime.Now().AddDate(0, 0, -6).Format("Y-m-d"),
		DateTo:   gtime.Now().Format("Y-m-d") + " 23:59:59",
		GroupBy:  "day",
	})
	if err != nil {
		return "", "", err
	}
	s := result.Summary
	answer := fmt.Sprintf("近七天车流中，港澳车 %d 辆，占总车流 %.1f%%；内地车 %d 辆，占比 %.1f%%。",
		s.HkMacauCount, s.HkMacauRatio*100, s.MainlandCount, (1-s.HkMacauRatio)*100)
	return answer, "", nil
}

func fastTrafficGateRank(ctx context.Context) (string, string, error) {
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: gtime.Now().Format("Y-m-d"),
		DateTo:   gtime.Now().Format("Y-m-d") + " 23:59:59",
		GroupBy:  "gate",
	})
	if err != nil {
		return "", "", err
	}
	if len(result.Series) == 0 {
		return "今日暂无卡口数据。", "", nil
	}
	xLabels, totalData, tableRows := trafficGateRankChartData(result.Series, 10)
	chart := buildChart("bar", "今日卡口车流排名（Top10）", xLabels,
		[]chartSeries{{Name: "车流量", Data: totalData}},
		[]string{"卡口", "合计", "进入", "离开"}, tableRows)
	answer := fmt.Sprintf("今日车流量最大的卡口为「%s」，共 %d 辆。", result.Series[0].Name, result.Series[0].Total)
	return answer, chart, nil
}

func fastTrafficWeekendCompare(ctx context.Context) (string, string, error) {
	weekendDates, workdayDates := recentWeekendAndWorkdayDates(time.Now(), 14)
	weekendSummary, err := trafficSummaryForDates(ctx, weekendDates)
	if err != nil {
		return "", "", err
	}
	workdaySummary, err := trafficSummaryForDates(ctx, workdayDates)
	if err != nil {
		return "", "", err
	}
	var weekendDaily, workdayDaily float64
	if len(weekendDates) > 0 {
		weekendDaily = float64(weekendSummary.Total) / float64(len(weekendDates))
	}
	if len(workdayDates) > 0 {
		workdayDaily = float64(workdaySummary.Total) / float64(len(workdayDates))
	}

	pct := 0.0
	direction := "相当"
	if workdayDaily > 0 {
		pct = (weekendDaily - workdayDaily) / workdayDaily * 100
		if pct > 1 {
			direction = "高"
		} else if pct < -1 {
			direction = "低"
			pct = -pct
		} else {
			pct = 0
		}
	}

	xLabels := []string{"周末日均", "平日日均"}
	barData := []int{int(weekendDaily), int(workdayDaily)}
	chart := buildChart("bar", "周末 vs 平日日均车流对比", xLabels,
		[]chartSeries{{Name: "日均车流", Data: barData}},
		[]string{"类型", "统计天数", "总车流", "日均车流"}, [][]any{
			{"周末", len(weekendDates), weekendSummary.Total, int(weekendDaily)},
			{"平日", len(workdayDates), workdaySummary.Total, int(workdayDaily)},
		})

	answer := fmt.Sprintf("近14天内，周末共统计%d天、日均车流%d辆；平日共统计%d天、日均车流%d辆，周末日均比平日%s %.1f%%。",
		len(weekendDates), int(weekendDaily), len(workdayDates), int(workdayDaily), direction, pct)
	return answer, chart, nil
}

func fastTrafficHolidayCompare(ctx context.Context) (string, string, error) {
	period, ok := latestChinaHolidayPeriod(time.Now())
	if !ok {
		return "当前未配置可用的中国法定节假日日历，无法进行节假日和平日对比。", "", nil
	}
	holidayDates := datesBetween(period.Start, period.End)
	workdayDates := adjacentWorkdays(period.End.AddDate(0, 0, 1), len(holidayDates))
	if len(workdayDates) < len(holidayDates) {
		workdayDates = adjacentWorkdaysBefore(period.Start.AddDate(0, 0, -1), len(holidayDates))
	}
	holidaySummary, err := trafficSummaryForDates(ctx, holidayDates)
	if err != nil {
		return "", "", err
	}
	workdaySummary, err := trafficSummaryForDates(ctx, workdayDates)
	if err != nil {
		return "", "", err
	}

	holidayDaily := 0.0
	workdayDaily := 0.0
	if len(holidayDates) > 0 {
		holidayDaily = float64(holidaySummary.Total) / float64(len(holidayDates))
	}
	if len(workdayDates) > 0 {
		workdayDaily = float64(workdaySummary.Total) / float64(len(workdayDates))
	}
	pct := 0.0
	direction := "持平"
	if workdayDaily > 0 {
		pct = (holidayDaily - workdayDaily) / workdayDaily * 100
		if pct > 1 {
			direction = "增长"
		} else if pct < -1 {
			direction = "下降"
			pct = -pct
		} else {
			pct = 0
		}
	}

	chart := buildChart("bar", period.Name+"节假日 vs 平日日均车流对比",
		[]string{"节假日日均", "平日日均"},
		[]chartSeries{{Name: "日均车流", Data: []int{int(holidayDaily), int(workdayDaily)}}},
		[]string{"类型", "日期范围", "统计天数", "总车流", "日均车流"},
		[][]any{
			{"节假日", formatDateListRange(holidayDates), len(holidayDates), holidaySummary.Total, int(holidayDaily)},
			{"平日", formatDateListRange(workdayDates), len(workdayDates), workdaySummary.Total, int(workdayDaily)},
		})
	answer := fmt.Sprintf("%s期间（%s），节假日日均车流%d辆；相邻平日（%s）日均车流%d辆，节假日日均较平日%s %.1f%%。",
		period.Name, formatDateListRange(holidayDates), int(holidayDaily), formatDateListRange(workdayDates), int(workdayDaily), direction, pct)
	return answer, chart, nil
}

func fastTrafficForeignOrigin(ctx context.Context) (string, string, error) {
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: gtime.Now().AddDate(0, 0, -6).Format("Y-m-d"),
		DateTo:   gtime.Now().Format("Y-m-d") + " 23:59:59",
		GroupBy:  "plateregion",
	})
	if err != nil {
		return "", "", err
	}
	if len(result.Series) == 0 {
		return "近七天暂无外地车来源数据。", "", nil
	}
	xLabels := make([]string, 0, len(result.Series))
	totalData := make([]int, 0, len(result.Series))
	tableRows := make([][]any, 0, len(result.Series))
	for i, item := range result.Series {
		if i >= 10 {
			break
		}
		xLabels = append(xLabels, item.Name)
		totalData = append(totalData, item.Total)
		tableRows = append(tableRows, []any{fmt.Sprintf("%d", i+1), item.Name, item.Total})
	}
	chart := buildChart("bar", "近七天外地车来源排名（Top10）", xLabels,
		[]chartSeries{{Name: "车流量", Data: totalData}},
		[]string{"排名", "来源地", "车流量"}, tableRows)
	answer := fmt.Sprintf("近七天外地车主要来自「%s」，共 %d 辆。", result.Series[0].Name, result.Series[0].Total)
	return answer, chart, nil
}

// ---- Population fast paths (via direct SQL) ----

func fastPopQuery(ctx context.Context, kind fastPathKind) (string, string, error) {
	db := g.DB("master")
	switch kind {
	case fpPopWeekTrend:
		return fastPopWeekTrendQuery(ctx, db)
	case fpPopHolidayCompare:
		return fastPopHolidayCompareQuery(ctx, db)
	case fpPopRegionRank:
		return fastPopRegionRankQuery(ctx, db)
	}
	return "暂不支持该问题的快速查询。", "", nil
}

func fastPopWeekTrendQuery(ctx context.Context, db gdb.DB) (string, string, error) {
	// Discover the population record table dynamically
	tableName := discoverPopTable(ctx, db)
	if tableName == "" {
		return "当前未接入人流数据，无法直接查询，请联系管理员开通人流数据接入。", "", nil
	}
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	records, err := db.Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT DATE(snapshot_time) AS day,
		       SUM(CASE WHEN direction = 'in' OR in_dir = 0 THEN 1 ELSE 0 END) AS in_count,
		       SUM(CASE WHEN direction = 'out' OR in_dir = 1 THEN 1 ELSE 0 END) AS out_count,
		       COUNT(*) AS total
		FROM %s
		WHERE snapshot_time >= ? AND snapshot_time <= ?
		GROUP BY day ORDER BY day`, tableName), from, to).All()
	if err != nil || len(records) == 0 {
		return "近七天暂无人流数据。", "", nil
	}
	xLabels := make([]string, 0, len(records))
	inData := make([]int, 0, len(records))
	outData := make([]int, 0, len(records))
	tableRows := make([][]any, 0, len(records))
	var totalAll int
	for _, r := range records {
		day := r["day"].String()
		ic := r["in_count"].Int()
		oc := r["out_count"].Int()
		t := r["total"].Int()
		totalAll += t
		xLabels = append(xLabels, day)
		inData = append(inData, ic)
		outData = append(outData, oc)
		tableRows = append(tableRows, []any{day, ic, oc, t})
	}
	chart := buildChart("line", "近七天人流进出趋势", xLabels,
		[]chartSeries{{Name: "进入人数", Data: inData}, {Name: "离开人数", Data: outData}},
		[]string{"日期", "进入", "离开", "合计"}, tableRows)
	answer := fmt.Sprintf("近七天人流总计 %d 人次，日均 %.0f 人次。", totalAll, float64(totalAll)/float64(len(records)))
	return answer, chart, nil
}

func fastPopHolidayCompareQuery(ctx context.Context, db gdb.DB) (string, string, error) {
	tableName := discoverPopTable(ctx, db)
	if tableName == "" {
		return "当前未接入人流数据，无法直接查询节假日对比。", "", nil
	}
	// Use current week vs last week as a simple comparison
	thisFrom := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	thisTo := gtime.Now().Format("Y-m-d") + " 23:59:59"
	lastFrom := gtime.Now().AddDate(0, 0, -13).Format("Y-m-d")
	lastTo := gtime.Now().AddDate(0, 0, -7).Format("Y-m-d") + " 23:59:59"
	thisCount, err := db.Ctx(ctx).Raw(fmt.Sprintf(
		"SELECT COUNT(*) AS c FROM %s WHERE snapshot_time >= ? AND snapshot_time <= ?", tableName), thisFrom, thisTo).Value()
	if err != nil {
		return "", "", err
	}
	lastCount, err := db.Ctx(ctx).Raw(fmt.Sprintf(
		"SELECT COUNT(*) AS c FROM %s WHERE snapshot_time >= ? AND snapshot_time <= ?", tableName), lastFrom, lastTo).Value()
	if err != nil {
		return "", "", err
	}
	tc := thisCount.Int()
	lc := lastCount.Int()
	pct := 0.0
	if lc > 0 {
		pct = float64(tc-lc) / float64(lc) * 100
	}
	direction := "增长"
	if pct < 0 {
		direction = "下降"
		pct = -pct
	}
	answer := fmt.Sprintf("本周人流 %d 人次，上周 %d 人次，环比%s %.1f%%。", tc, lc, direction, pct)
	return answer, "", nil
}

func fastPopRegionRankQuery(ctx context.Context, db gdb.DB) (string, string, error) {
	tableName := discoverPopTable(ctx, db)
	if tableName == "" {
		return "当前未接入人流数据，无法查询区域排名。", "", nil
	}
	records, err := db.Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT COALESCE(region, '未知') AS name, COUNT(*) AS total
		FROM %s
		WHERE snapshot_time >= ? AND snapshot_time <= ?
		GROUP BY name ORDER BY total DESC LIMIT 10`,
		tableName),
		gtime.Now().AddDate(0, 0, -6).Format("Y-m-d"),
		gtime.Now().Format("Y-m-d")+" 23:59:59").All()
	if err != nil || len(records) == 0 {
		return "近七天暂无区域人流数据。", "", nil
	}
	xLabels := make([]string, 0, len(records))
	totalData := make([]int, 0, len(records))
	tableRows := make([][]any, 0, len(records))
	for i, r := range records {
		xLabels = append(xLabels, r["name"].String())
		totalData = append(totalData, r["total"].Int())
		tableRows = append(tableRows, []any{fmt.Sprintf("%d", i+1), r["name"].String(), r["total"].Int()})
	}
	chart := buildChart("bar", "近七天区域人流排名（Top10）", xLabels,
		[]chartSeries{{Name: "人流量", Data: totalData}},
		[]string{"排名", "区域", "人流量"}, tableRows)
	answer := fmt.Sprintf("近七天人流最多的区域为「%s」，共 %d 人次。", records[0]["name"].String(), records[0]["total"].Int())
	return answer, chart, nil
}

// ---- Grid fast paths (via direct SQL on case_list) ----

func fastGridQuery(ctx context.Context, kind fastPathKind) (string, string, error) {
	db := g.DB("master")
	switch kind {
	case fpGridCaseCount:
		count, err := db.Model("case_list").Ctx(ctx).Count()
		if err != nil {
			return "", "", err
		}
		return fmt.Sprintf("当前案件总数为 %d 件。", count), "", nil
	case fpGridCloseRate:
		total, _ := db.Model("case_list").Ctx(ctx).Count()
		closed, _ := db.Model("case_list").Ctx(ctx).Where("pending_step", "结案").Count()
		rate := 0.0
		if total > 0 {
			rate = float64(closed) / float64(total) * 100
		}
		return fmt.Sprintf("案件总数 %d 件，已结案 %d 件，结案率 %.1f%%。", total, closed, rate), "", nil
	case fpGridRegionRank:
		records, err := db.Ctx(ctx).Raw(`
			SELECT COALESCE(region, '未知') AS name, COUNT(*) AS total
			FROM case_list
			GROUP BY name ORDER BY total DESC LIMIT 10`).All()
		if err != nil || len(records) == 0 {
			return "暂无区域案件数据。", "", nil
		}
		xLabels := make([]string, 0, len(records))
		totalData := make([]int, 0, len(records))
		tableRows := make([][]any, 0, len(records))
		for i, r := range records {
			xLabels = append(xLabels, r["name"].String())
			totalData = append(totalData, r["total"].Int())
			tableRows = append(tableRows, []any{fmt.Sprintf("%d", i+1), r["name"].String(), r["total"].Int()})
		}
		chart := buildChart("bar", "区域案件数量排名（Top10）", xLabels,
			[]chartSeries{{Name: "案件数", Data: totalData}},
			[]string{"排名", "区域", "案件数"}, tableRows)
		answer := fmt.Sprintf("案件最多的区域为「%s」，共 %d 件。", records[0]["name"].String(), records[0]["total"].Int())
		return answer, chart, nil
	}
	return "暂不支持该问题的快速查询。", "", nil
}

// ---- Helpers ----

type chinaHolidayPeriod struct {
	Name  string
	Start time.Time
	End   time.Time
}

type calendarDayType string

const (
	calendarDayHoliday         calendarDayType = "holiday"
	calendarDayWeekend         calendarDayType = "weekend"
	calendarDayWorkday         calendarDayType = "workday"
	calendarDayAdjustedWorkday calendarDayType = "adjusted_workday"
)

var chinaHolidayDates2026 = map[string]string{
	"2026-01-01": "元旦",
	"2026-01-02": "元旦",
	"2026-01-03": "元旦",
	"2026-02-15": "春节",
	"2026-02-16": "春节",
	"2026-02-17": "春节",
	"2026-02-18": "春节",
	"2026-02-19": "春节",
	"2026-02-20": "春节",
	"2026-02-21": "春节",
	"2026-02-22": "春节",
	"2026-02-23": "春节",
	"2026-04-04": "清明节",
	"2026-04-05": "清明节",
	"2026-04-06": "清明节",
	"2026-05-01": "劳动节",
	"2026-05-02": "劳动节",
	"2026-05-03": "劳动节",
	"2026-05-04": "劳动节",
	"2026-05-05": "劳动节",
	"2026-06-19": "端午节",
	"2026-06-20": "端午节",
	"2026-06-21": "端午节",
	"2026-09-25": "中秋节",
	"2026-09-26": "中秋节",
	"2026-09-27": "中秋节",
	"2026-10-01": "国庆节",
	"2026-10-02": "国庆节",
	"2026-10-03": "国庆节",
	"2026-10-04": "国庆节",
	"2026-10-05": "国庆节",
	"2026-10-06": "国庆节",
	"2026-10-07": "国庆节",
}

var chinaAdjustedWorkdays2026 = map[string]string{
	"2026-01-04": "元旦调休工作日",
	"2026-02-14": "春节调休工作日",
	"2026-02-28": "春节调休工作日",
	"2026-05-09": "劳动节调休工作日",
	"2026-09-20": "国庆节调休工作日",
	"2026-10-10": "国庆节调休工作日",
}

func latestChinaHolidayPeriod(now time.Time) (chinaHolidayPeriod, bool) {
	today := normalizeDate(now)
	years := []int{today.Year(), today.Year() - 1}
	var latest chinaHolidayPeriod
	found := false
	for _, year := range years {
		for _, period := range chinaHolidayPeriods(year, today.Location()) {
			if (today.Equal(period.Start) || today.After(period.Start)) && (today.Equal(period.End) || today.Before(period.End)) {
				return period, true
			}
			if period.End.Before(today) && (!found || period.End.After(latest.End)) {
				latest = period
				found = true
			}
		}
	}
	return latest, found
}

func chinaHolidayPeriods(year int, loc *time.Location) []chinaHolidayPeriod {
	switch year {
	case 2026:
		return []chinaHolidayPeriod{
			{Name: "元旦", Start: mustDateInLocation("2026-01-01", loc), End: mustDateInLocation("2026-01-03", loc)},
			{Name: "春节", Start: mustDateInLocation("2026-02-15", loc), End: mustDateInLocation("2026-02-23", loc)},
			{Name: "清明节", Start: mustDateInLocation("2026-04-04", loc), End: mustDateInLocation("2026-04-06", loc)},
			{Name: "劳动节", Start: mustDateInLocation("2026-05-01", loc), End: mustDateInLocation("2026-05-05", loc)},
			{Name: "端午节", Start: mustDateInLocation("2026-06-19", loc), End: mustDateInLocation("2026-06-21", loc)},
			{Name: "中秋节", Start: mustDateInLocation("2026-09-25", loc), End: mustDateInLocation("2026-09-27", loc)},
			{Name: "国庆节", Start: mustDateInLocation("2026-10-01", loc), End: mustDateInLocation("2026-10-07", loc)},
		}
	default:
		return nil
	}
}

func mustDateInLocation(value string, loc *time.Location) time.Time {
	t, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

func normalizeDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dateKey(t time.Time) string {
	return normalizeDate(t).Format("2006-01-02")
}

func chinaCalendarDayType(t time.Time) calendarDayType {
	key := dateKey(t)
	if _, ok := chinaHolidayDates2026[key]; ok {
		return calendarDayHoliday
	}
	if _, ok := chinaAdjustedWorkdays2026[key]; ok {
		return calendarDayAdjustedWorkday
	}
	weekday := normalizeDate(t).Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return calendarDayWeekend
	}
	return calendarDayWorkday
}

func isChinaWorkday(t time.Time) bool {
	dayType := chinaCalendarDayType(t)
	return dayType == calendarDayWorkday || dayType == calendarDayAdjustedWorkday
}

func recentWeekendAndWorkdayDates(now time.Time, days int) ([]time.Time, []time.Time) {
	if days <= 0 {
		return nil, nil
	}
	today := normalizeDate(now)
	start := today.AddDate(0, 0, -days+1)
	weekendDates := make([]time.Time, 0, days/3)
	workdayDates := make([]time.Time, 0, days)
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		switch chinaCalendarDayType(d) {
		case calendarDayWeekend:
			weekendDates = append(weekendDates, d)
		case calendarDayWorkday, calendarDayAdjustedWorkday:
			workdayDates = append(workdayDates, d)
		}
	}
	return weekendDates, workdayDates
}

func datesBetween(start, end time.Time) []time.Time {
	start = normalizeDate(start)
	end = normalizeDate(end)
	if end.Before(start) {
		return nil
	}
	dates := make([]time.Time, 0, int(end.Sub(start).Hours()/24)+1)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	return dates
}

func adjacentWorkdays(start time.Time, count int) []time.Time {
	dates := make([]time.Time, 0, count)
	for d, guard := normalizeDate(start), 0; len(dates) < count && guard < 60; d, guard = d.AddDate(0, 0, 1), guard+1 {
		if isChinaWorkday(d) {
			dates = append(dates, d)
		}
	}
	return dates
}

func adjacentWorkdaysBefore(start time.Time, count int) []time.Time {
	dates := make([]time.Time, 0, count)
	for d, guard := normalizeDate(start), 0; len(dates) < count && guard < 60; d, guard = d.AddDate(0, 0, -1), guard+1 {
		if isChinaWorkday(d) {
			dates = append(dates, d)
		}
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
	return dates
}

func formatDateListRange(dates []time.Time) string {
	if len(dates) == 0 {
		return "-"
	}
	cp := append([]time.Time(nil), dates...)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Before(cp[j])
	})
	if len(cp) == 1 {
		return cp[0].Format("1月2日")
	}
	return fmt.Sprintf("%s至%s", cp[0].Format("1月2日"), cp[len(cp)-1].Format("1月2日"))
}

func trafficSummaryForDates(ctx context.Context, dates []time.Time) (model.TrafficAggregateSummary, error) {
	if len(dates) == 0 {
		return model.TrafficAggregateSummary{}, nil
	}
	conditions := make([]string, 0, len(dates))
	args := make([]any, 0, len(dates)*2)
	for _, d := range dates {
		day := normalizeDate(d)
		conditions = append(conditions, "(snapshot_time >= ? AND snapshot_time <= ?)")
		args = append(args, day.Format("2006-01-02 00:00:00"), day.Format("2006-01-02 23:59:59"))
	}
	sql := fmt.Sprintf(`SELECT
COUNT(*) AS total,
SUM(CASE WHEN in_dir = 0 THEN 1 ELSE 0 END) AS in_count,
SUM(CASE WHEN in_dir = 1 THEN 1 ELSE 0 END) AS out_count,
SUM(CASE WHEN in_dir <> 0 AND in_dir <> 1 THEN 1 ELSE 0 END) AS unknown_dir_count,
SUM(CASE WHEN is_hk_macau = 1 THEN 1 ELSE 0 END) AS hk_macau_count
FROM traffic_gate_record
WHERE %s`, strings.Join(conditions, " OR "))
	record, err := g.DB("master").Ctx(ctx).Raw(sql, args...).One()
	if err != nil {
		return model.TrafficAggregateSummary{}, err
	}
	return trafficSummaryFromRecord(record), nil
}

func trafficSummaryFromRecord(record gdb.Record) model.TrafficAggregateSummary {
	if record == nil {
		return model.TrafficAggregateSummary{}
	}
	total := record["total"].Int()
	hkMacau := record["hk_macau_count"].Int()
	ratio := 0.0
	if total > 0 {
		ratio = float64(hkMacau) / float64(total)
	}
	return model.TrafficAggregateSummary{
		Total:           total,
		InCount:         record["in_count"].Int(),
		OutCount:        record["out_count"].Int(),
		HkMacauCount:    hkMacau,
		HkMacauRatio:    ratio,
		MainlandCount:   total - hkMacau,
		UnknownDirCount: record["unknown_dir_count"].Int(),
	}
}

func trafficGateRankChartData(series []model.TrafficAggregateSeriesItem, limit int) ([]string, []int, [][]any) {
	if limit <= 0 || limit > len(series) {
		limit = len(series)
	}
	xLabels := make([]string, 0, limit)
	totalData := make([]int, 0, limit)
	tableRows := make([][]any, 0, limit)
	for i, item := range series {
		if i >= limit {
			break
		}
		xLabels = append(xLabels, item.Name)
		totalData = append(totalData, item.Total)
		tableRows = append(tableRows, []any{item.Name, item.Total, item.InCount, item.OutCount})
	}
	return xLabels, totalData, tableRows
}

func discoverPopTable(ctx context.Context, db gdb.DB) string {
	// Try common population table names
	for _, t := range []string{"population_record", "population_gate_record", "population_snapshot"} {
		count, err := db.Model(t).Ctx(ctx).Count()
		if err == nil && count >= 0 {
			return t
		}
	}
	return ""
}

type chartSeries struct {
	Name string `json:"name"`
	Data any    `json:"data"`
}

func buildChart(chartType, title string, x []string, series []chartSeries, columns []string, rows [][]any) string {
	chart := g.Map{
		"type":   chartType,
		"title":  title,
		"x":      x,
		"series": series,
		"table": g.Map{
			"columns": columns,
			"rows":    rows,
		},
	}
	b, err := json.Marshal(chart)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("```chatdb-chart\n%s\n```", string(b))
}
