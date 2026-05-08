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

	sessionId, err := ensureAskNumberSession(ctx, userId, req.SessionId, req.Topic, req.DatabaseId, req.Message)
	if err != nil {
		return nil, err
	}
	req.SessionId = sessionId
	storedHistory, err := loadAskNumberSessionHistory(ctx, userId, sessionId, 12)
	if err != nil {
		return nil, err
	}
	req.History = mergeChatHistory(storedHistory, req.History)
	if err = appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "user", req.Message); err != nil {
		return nil, err
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

	var jsonData []byte
	var assistantBuilder strings.Builder
	for v := range respChan {
		switch item := v.(type) {
		case string:
			r.Response.Writef("%s\n\n", item)
		case error:
			r.Response.Writef("data: %s\n\n", gjson.MustEncodeString(g.Map{"error": fmt.Sprintf("%s", item)}))
		default:
			if out, ok := item.(model.ChatOutDataItem); ok {
				if out.Event == "message" && out.Content != "" && (out.Role == "" || strings.EqualFold(out.Role, "assistant")) {
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
	if err = ensureAskNumberSessionTables(ctx); err != nil {
		return nil, err
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
	if err := ensureAskNumberSessionTables(ctx); err != nil {
		return "", err
	}
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

func ensureAskNumberSessionTables(ctx context.Context) error {
	db := g.DB("master")
	dbType := ""
	if cfg := db.GetConfig(); cfg != nil {
		dbType = cfg.Type
	}
	return createAskNumberSessionTables(ctx, db, dbType)
}

func createAskNumberSessionTables(ctx context.Context, db gdb.DB, dbType string) error {
	var sessionSQL string
	var messageSQL string
	switch dbType {
	case "sqlite":
		sessionSQL = `CREATE TABLE IF NOT EXISTS ask_number_session (
session_id TEXT PRIMARY KEY,
user_id INTEGER NOT NULL,
topic TEXT,
database_id INTEGER NOT NULL DEFAULT 0,
title TEXT,
status TEXT NOT NULL DEFAULT 'active',
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`
		messageSQL = `CREATE TABLE IF NOT EXISTS ask_number_message (
id INTEGER PRIMARY KEY AUTOINCREMENT,
session_id TEXT NOT NULL,
user_id INTEGER NOT NULL,
topic TEXT,
role TEXT NOT NULL,
content TEXT NOT NULL,
create_time INTEGER NOT NULL
)`
	default:
		sessionSQL = `CREATE TABLE IF NOT EXISTS ask_number_session (
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
		messageSQL = `CREATE TABLE IF NOT EXISTS ask_number_message (
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
	}
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
