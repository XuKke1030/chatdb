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
	"ai-chat-sql/internal/logic/precipitate"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"ai-chat-sql/utility"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
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
	userIdVal := model.UserIdFromContext(ctx)
	userId := 0
	if userIdVal != 0 {
		userId = userIdVal
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
	// Normalize: "context" is an alias for "history" (frontend compat)
	if len(req.Context) > 0 && len(req.History) == 0 {
		req.History = req.Context
	}
	req.History = mergeChatHistory(storedHistory, req.History)
	logStage("history")
	if err = appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "user", req.Message); err != nil {
		return nil, err
	}
	logStage("save_user_message")

	// Record question for auto-precipitation (fire-and-forget)
	g.Go(ctx, func(ctx context.Context) {
		_ = upsertAskNumberQuestionStat(ctx, userId, req.Topic, req.Message)
	}, func(ctx context.Context, exception error) {
		consts.Logger.Errorf(ctx, "upsertAskNumberQuestionStat error: %v", exception)
	})


	if req.Topic == "grid" && isMajorCaseQuestion(req.Message) {
		return c.streamGridMajorCaseAnswer(ctx, req, userId, sessionId)
	}

	respChan := make(chan any, 64)
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
	var isClarification bool
	var collectedTables string
	firstTokenLogged := false
	for {
		select {
		case <-ctx.Done():
			consts.Logger.Infof(ctx, "SSE client disconnected, aborting stream topic=%s sessionId=%s", req.Topic, sessionId)
			return
		case v, ok := <-respChan:
			if !ok {
				return
			}
			switch item := v.(type) {
			case string:
				r.Response.Writef("%s\n\n", item)
			case error:
				outData := model.GenChatOutDataItem(ctx, model.ChatOutDataItem{
					Event: "error",
					Data:  g.Map{"message": askNumberBusinessError(item)},
				})
				if jsonData, err = json.Marshal(outData); err == nil {
					r.Response.Writef("data: %s\n\n", jsonData)
				}
			default:
				if out, ok := item.(model.ChatOutDataItem); ok {
					if out.Event == "message" && out.Content != "" && (out.Role == "" || strings.EqualFold(out.Role, "assistant")) {
						if !firstTokenLogged {
							firstTokenLogged = true
							consts.Logger.Infof(ctx, "perf ask_number stage=first_token topic=%s sessionId=%s totalMs=%d", req.Topic, sessionId, time.Since(totalStart).Milliseconds())
						}
						assistantBuilder.WriteString(out.Content)
					}
					if out.Event == "clarification" {
						isClarification = true
					}
					if out.Event == "tables" && out.Content != "" {
						collectedTables = out.Content
					}
					if out.Event == "end" && assistantBuilder.Len() > 0 && !isClarification {
						saveContent := utility.SanitizeOutput(assistantBuilder.String())
						if collectedTables != "" {
							saveContent += "\n<!-- tables:" + collectedTables + " -->"
						}
						if saveErr := appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "assistant", saveContent); saveErr != nil {
							consts.Logger.Errorf(ctx, "保存问数会话回复失败: %s", saveErr.Error())
						}
					}
				}
				jsonData, err = json.Marshal(item)
				if err != nil {
					return
				}
				r.Response.Writef("data: %s\n\n", jsonData)
				if out, ok := item.(model.ChatOutDataItem); ok && out.Event == "end" {
					r.Response.Writef("data: [DONE]\n\n")
					r.Response.Flush()
					return
				}
			}
			r.Response.Flush()
		}
	}
}

func askNumberBusinessError(err error) string {
	return utility.SafeUserErr(err)
}

func (c *ControllerV1) ChatSessionCreate(ctx context.Context, req *v1.ChatSessionCreateReq) (res *v1.ChatSessionCreateRes, err error) {
	userIdVal := model.UserIdFromContext(ctx)
	userId := 0
	if userIdVal != 0 {
		userId = userIdVal
	}
	topic := strings.ToLower(strings.TrimSpace(req.Topic))
	if !isKnownAskNumberTopic(topic) {
		return nil, gerror.New("unknown topic")
	}
	if userId > 0 {
		user, userErr := service.User().GetUserInfoById(ctx, int64(userId))
		if userErr != nil || user == nil {
			return nil, gerror.New("user not found")
		}
		ruleLevel := effectiveTopicRuleLevel(ctx, userId, user.RuleLevel)
		if !checkTopicPermission(ruleLevel, topic) {
			return nil, gerror.New("no permission for topic")
		}
	}
	sessionId, err := ensureAskNumberSession(ctx, userId, "", topic, 0, topicSessionTitle(topic))
	if err != nil {
		return nil, err
	}
	return &v1.ChatSessionCreateRes{
		SessionId:          sessionId,
		Topic:              topic,
		Title:              topicSessionTitle(topic),
		SuggestedQuestions: suggestedQuestionsForTopic(ctx, topic),
		InputPlaceholder:   inputPlaceholderForTopic(topic),
	}, nil
}

func (c *ControllerV1) ChatSessionReset(ctx context.Context, req *v1.ChatSessionResetReq) (res *v1.ChatSessionResetRes, err error) {
	userIdVal := model.UserIdFromContext(ctx)
	userId := 0
	if userIdVal != 0 {
		userId = userIdVal
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
	err = g.DB("master").Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("DELETE FROM ask_number_message WHERE session_id = ? AND user_id = ?", req.SessionId, userId); err != nil {
			return err
		}
		_, err := tx.Exec("UPDATE ask_number_session SET status = ?, update_time = ? WHERE session_id = ? AND user_id = ?",
			"reset", now, req.SessionId, userId)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChatSessionResetRes{SessionId: req.SessionId}, nil
}

func (c *ControllerV1) ExampleQuestions(ctx context.Context, req *v1.ExampleQuestionsReq) (res *v1.ExampleQuestionsRes, err error) {
	query := g.DB("master").Model("admin_example_question").Ctx(ctx).Where("enabled", 1)
	if req.Topic != "" {
		query = query.Where("topic", req.Topic)
	}
	records, err := query.OrderAsc("sort").OrderDesc("update_time").All()
	if err != nil {
		return nil, err
	}
	items := make([]v1.ExampleQuestionItem, 0, len(records))
	for _, r := range records {
		items = append(items, v1.ExampleQuestionItem{
			Id:          r["id"].Int(),
			Topic:       r["topic"].String(),
			Question:    r["question"].String(),
			Description: r["description"].String(),
		})
	}
	return &v1.ExampleQuestionsRes{Items: items}, nil
}

func (c *ControllerV1) GridMajorCaseAnalysis(ctx context.Context, req *v1.GridMajorCaseAnalysisReq) (res *v1.GridMajorCaseAnalysisRes, err error) {
	userIdVal := model.UserIdFromContext(ctx)
	userId := 0
	if userIdVal != 0 {
		userId = userIdVal
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
	if item.Event == "end" {
		r.Response.Writef("data: [DONE]\n\n")
	}
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
	return g.DB("master").Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`INSERT INTO ask_number_message (session_id, user_id, topic, role, content, create_time) VALUES (?, ?, ?, ?, ?, ?)`,
			sessionId, userId, topic, role, content, now); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE ask_number_session SET update_time = ? WHERE session_id = ? AND user_id = ?`,
			now, sessionId, userId)
		return err
	})
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
	questionStatSQL := `CREATE TABLE IF NOT EXISTS ask_number_question_stat (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
user_id INT NOT NULL,
topic VARCHAR(32) NOT NULL,
question_normalized VARCHAR(255) NOT NULL,
hit_count INT NOT NULL DEFAULT 1,
last_asked_at INT NOT NULL,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_user_topic_question (user_id, topic, question_normalized),
INDEX idx_hit_topic (hit_count DESC, topic)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	if _, err := db.Exec(ctx, sessionSQL); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, messageSQL); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, questionStatSQL); err != nil {
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

func normalizeAskNumberQuestion(q string) string {
	return precipitate.NormalizeQuestion(q)
}

func upsertAskNumberQuestionStat(ctx context.Context, userId int, topic, question string) error {
	normalized := normalizeAskNumberQuestion(question)
	if normalized == "" {
		return nil
	}
	now := int(gtime.Timestamp())
	_, err := g.DB("master").Exec(ctx, `
INSERT INTO ask_number_question_stat (user_id, topic, question_normalized, hit_count, last_asked_at, create_time, update_time)
VALUES (?, ?, ?, 1, ?, ?, ?)
ON DUPLICATE KEY UPDATE hit_count = hit_count + 1, last_asked_at = VALUES(last_asked_at), update_time = VALUES(update_time)`,
		userId, topic, normalized, now, now, now)
	return err
}

func isKnownAskNumberTopic(topic string) bool {
	_, ok := topicPermissionMap[topic]
	return ok
}

func topicSessionTitle(topic string) string {
	switch topic {
	case "grid":
		return "网格问数"
	case "population":
		return "人流问数"
	case "traffic":
		return "车流问数"
	default:
		return "问数"
	}
}

func suggestedQuestionsForTopic(ctx context.Context, topic string) []string {
	records, err := g.DB("master").Model("admin_example_question").Ctx(ctx).
		Where("enabled", 1).Where("topic", topic).
		OrderAsc("sort").OrderDesc("update_time").All()
	if err == nil && len(records) > 0 {
		qs := make([]string, 0, len(records))
		for _, r := range records {
			if q := r["question"].String(); q != "" {
				qs = append(qs, q)
			}
		}
		if len(qs) > 0 {
			return qs
		}
	}
	switch topic {
	case "grid":
		return []string{"这个月哪个社区案件最多？", "本月网格案件结案率是多少？", "分析一下当前最需要关注的重大案件"}
	case "population":
		return []string{"今天人流的年龄分布？", "进站来源地排名前5？", "活力指数是多少？", "年轻人进站趋势如何？", "流动人口有异常吗？"}
	case "traffic":
		return []string{"今天车流总量是多少？", "哪个关口车流最多？", "最近 7 天港澳车趋势怎么样？"}
	default:
		return []string{}
	}
}

func inputPlaceholderForTopic(topic string) string {
	switch topic {
	case "grid":
		return "可以问我案件总量、结案率、社区排名、类别分布、重大案件..."
	case "population":
		return "可以问我人流总量、区域排名、峰值时段、年龄分布、来源地占比、活力指数..."
	case "traffic":
		return "可以问我车流总量、关口排名、港澳车、外地车、停留时长..."
	default:
		return "请输入你想查询的问题..."
	}
}

func analyzeGridMajorCase(ctx context.Context, metric string, region string) (*v1.GridMajorCaseAnalysisRes, error) {
	metric = normalizeMajorCaseMetric(metric)
	query := g.DB("master").Model("case_list").Ctx(ctx)
	if count, err := g.DB("master").Model("grid_case_record").Ctx(ctx).Count(); err == nil && count > 0 {
		query = g.DB("master").Model("grid_case_record").Ctx(ctx).Fields(`
id,
case_number,
'' AS case_source,
report_time,
case_status AS pending_step,
COALESCE(NULLIF(case_type2,''), NULLIF(case_type1,'')) AS case_type,
region,
community AS responsibility_unit,
grid_name AS case_location,
case_title AS description`)
	}
	query = query.Where("report_time >= ?", gtime.Now().AddDate(0, 0, -30).StartOfDay().Unix())
	if strings.TrimSpace(region) != "" {
		query = query.WhereLike("region", "%"+utility.EscapeLike(strings.TrimSpace(region))+"%")
	}
	records, err := query.OrderDesc("id").Limit(500).All()
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
	applyDuplicateCaseScores(cases)
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
		fmt.Sprintf("影响度评分：%d，处置难度评分：%d，时效风险评分：%d，综合评分：%d，等级：%s。", top.ImpactScore, top.DifficultyScore, top.TimeRiskScore, top.TotalScore, top.Level),
	}
	basis = append(basis, top.Reasons...)
	return &v1.GridMajorCaseAnalysisRes{
		Metric:     metric,
		Case:       top,
		Basis:      basis,
		Suggestion: top.Suggestion,
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
	impact := 0
	difficulty := 0
	timeRisk := 0
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
	addTimeRisk := func(score int, reason string) {
		timeRisk += score
		reasons = append(reasons, fmt.Sprintf("时效风险：%s（+%d）", reason, score))
	}

	if containsAny(text, []string{"安全", "应急", "消防", "燃气", "漏电", "污染", "群体", "群体投诉", "城市运行"}) {
		addImpact(30, "案件类别或描述属于安全、应急、群体投诉或城市运行类")
	}
	if containsAny(text, []string{"多人", "群体", "反复", "投诉", "隐患", "危险", "停电", "堵塞"}) {
		addImpact(20, "描述包含多人、群体、反复、投诉、隐患、危险、停电或堵塞等高影响关键词")
	}
	if containsAny(text, []string{"地基下沉", "路面塌陷", "塌陷", "沉降", "裂缝", "危房", "坍塌", "结构安全"}) {
		addImpact(20, "涉及地基、塌陷、裂缝或建筑结构安全等高危隐患")
	}
	if containsAny(text, []string{"媒体", "舆情", "督办", "上级", "领导", "12345", "热线", "信访"}) {
		addImpact(15, "来源或描述包含媒体、舆情、督办、热线或信访等关注渠道")
	}
	if containsAny(text, []string{"小区", "居民", "住宅", "楼栋", "物业", "建筑物", "房屋"}) {
		addImpact(10, "涉及住宅小区、居民或建筑物，影响人群和安全面更广")
	}

	if isBlankResponsibility(item.ResponsibilityUnit) {
		addDifficulty(20, "责任单位为空或责任主体不明确")
	}
	if strings.Contains(item.ResponsibilityUnit, ",") || strings.Contains(item.ResponsibilityUnit, "、") || strings.Contains(item.ResponsibilityUnit, ";") || strings.Contains(item.ResponsibilityUnit, "；") {
		addDifficulty(20, "责任单位涉及多个主体，存在跨部门协同难度")
	}
	if containsAny(text, []string{"协调", "多部门", "权属", "历史遗留", "疑难", "无法", "困难", "拒不整改"}) {
		addDifficulty(20, "涉及协调、权属、历史遗留或整改阻力")
	}
	if !isMajorCaseClosed(*item) && strings.TrimSpace(item.PendingStep) != "" {
		addDifficulty(10, "当前仍处于待办/处置链路")
	}
	if containsAny(text, []string{"重复", "反复", "多次", "持续"}) {
		addDifficulty(10, "存在重复、反复或持续性表述")
	}

	if days := majorCaseAgeDays(item.ReportTime); !isMajorCaseClosed(*item) {
		if days >= 7 {
			addTimeRisk(30, fmt.Sprintf("未结且上报已超过%d天", days))
		} else if days >= 3 {
			addTimeRisk(20, fmt.Sprintf("未结且上报已超过%d天", days))
		}
	}

	item.ImpactScore = clampScore(impact)
	item.DifficultyScore = clampScore(difficulty)
	item.TimeRiskScore = clampScore(timeRisk)
	item.TotalScore = clampScore(item.ImpactScore + item.DifficultyScore + item.TimeRiskScore)
	item.Level = majorCaseLevel(item.TotalScore)
	item.Reasons = reasons
	item.Suggestion = majorCaseSuggestions(*item)
}

func applyDuplicateCaseScores(cases []v1.MajorCaseItem) {
	counts := make(map[string]int)
	for _, item := range cases {
		key := duplicateCaseKey(item)
		if key != "" {
			counts[key]++
		}
	}
	for i := range cases {
		count := counts[duplicateCaseKey(cases[i])]
		if count >= 5 {
			addMajorCaseExtraScore(&cases[i], 20, "同区域/同类别近批次重复案件较多")
		} else if count >= 3 {
			addMajorCaseExtraScore(&cases[i], 10, "同区域/同类别存在重复案件")
		}
	}
}

func duplicateCaseKey(item v1.MajorCaseItem) string {
	parts := []string{strings.TrimSpace(item.Region), strings.TrimSpace(item.CaseType)}
	key := strings.Join(parts, "|")
	if strings.Trim(key, "| ") == "" {
		return ""
	}
	return key
}

func addMajorCaseExtraScore(item *v1.MajorCaseItem, score int, reason string) {
	item.DifficultyScore = clampScore(item.DifficultyScore + score)
	item.TotalScore = clampScore(item.TotalScore + score)
	item.Level = majorCaseLevel(item.TotalScore)
	item.Reasons = append(item.Reasons, fmt.Sprintf("历史重复：%s（+%d）", reason, score))
	item.Suggestion = majorCaseSuggestions(*item)
}

func majorCaseLevel(score int) string {
	switch {
	case score >= 80:
		return "重大"
	case score >= 60:
		return "重点关注"
	case score >= 40:
		return "一般关注"
	default:
		return "普通案件"
	}
}

func majorCaseSuggestions(item v1.MajorCaseItem) []string {
	suggestions := make([]string, 0, 3)
	if item.Level == "重大" || item.TimeRiskScore >= 20 {
		suggestions = append(suggestions, "建议优先核实现场风险，并纳入重点跟踪清单。")
	}
	if isBlankResponsibility(item.ResponsibilityUnit) || item.DifficultyScore >= 20 {
		suggestions = append(suggestions, "建议明确责任单位和协同部门，形成处置闭环。")
	}
	if item.ImpactScore >= 30 {
		suggestions = append(suggestions, "建议同步关注群众反馈和舆情风险，必要时提前发布处置进展。")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "建议按常规网格流程持续跟踪处置进度。")
	}
	if len(suggestions) > 3 {
		return suggestions[:3]
	}
	return suggestions
}

func isBlankResponsibility(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "暂无" || value == "未知" || value == "-"
}

func isMajorCaseClosed(item v1.MajorCaseItem) bool {
	return containsAny(item.PendingStep, []string{"结案", "办结", "已办结", "已结案", "closed", "done", "finished", "resolved"})
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
		fmt.Sprintf("综合评分：%d（影响度%d，处置难度%d，时效风险%d），等级：%s", item.TotalScore, item.ImpactScore, item.DifficultyScore, item.TimeRiskScore, emptyText(item.Level, "暂无")),
		"",
		"判断依据：",
	}
	for _, reason := range res.Basis {
		lines = append(lines, "- "+reason)
	}
	suggestions := res.Suggestion
	if len(suggestions) == 0 {
		suggestions = item.Suggestion
	}
	if len(suggestions) > 0 {
		lines = append(lines, "", "处置建议：")
		for _, suggestion := range suggestions {
			lines = append(lines, "- "+suggestion)
		}
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
	if _, err := g.DB("master").Model("admin_operation_log").Ctx(ctx).Data(g.Map{
		"log_type":    "system",
		"username":    username,
		"action_type": actionType,
		"content":     message,
		"result":      result,
		"create_time": int(gtime.Timestamp()),
	}).Insert(); err != nil {
		consts.Logger.Warningf(ctx, "insertSystemQueryLog failed: %v", err)
	}
}
