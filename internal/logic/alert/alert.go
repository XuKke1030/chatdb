package alert

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func init() {
	service.RegisterAlert(NewAlert())
}

func NewAlert() *sAlert {
	return &sAlert{}
}

type sAlert struct {
}

// topicPermissionMap 主题权限位掩码映射
var topicPermissionMap = map[string]int{
	"grid":       1, // bit 0
	"population": 2, // bit 1
	"traffic":    4, // bit 2
}

// checkTopicPermission 检查用户是否有指定主题的权限
func checkTopicPermission(ruleLevel int, topic string) bool {
	if topic == "" {
		return true
	}
	bit, ok := topicPermissionMap[topic]
	if !ok {
		return false
	}
	return ruleLevel&bit != 0
}

// GetAlertList 获取告警列表
func (s *sAlert) GetAlertList(ctx context.Context, userId int, topic string) ([]model.AlertItem, error) {
	topics := normalizeTopics(topic)
	if len(topics) == 0 {
		topics = []string{"grid", "population", "traffic"}
	}
	grouped, err := s.GetAlertListByTopics(ctx, userId, topics)
	if err != nil {
		return nil, err
	}
	result := make([]model.AlertItem, 0)
	for _, itemTopic := range []string{"grid", "population", "traffic"} {
		result = append(result, grouped[itemTopic]...)
	}
	return result, nil
}

func (s *sAlert) GetAlertListByTopics(ctx context.Context, userId int, topics []string) (map[string][]model.AlertItem, error) {
	// 获取用户权限等级
	ruleLevel, err := getUserRuleLevel(ctx, userId)
	if err != nil {
		return nil, err
	}

	allowedTopics := filterAllowedTopics(ruleLevel, topics)
	result := make(map[string][]model.AlertItem, len(allowedTopics))
	for _, itemTopic := range allowedTopics {
		result[itemTopic] = []model.AlertItem{}
	}
	if len(allowedTopics) == 0 {
		return result, nil
	}

	dismissed, err := s.getDismissedAlertMap(ctx, userId)
	if err != nil {
		return nil, err
	}
	dynamicAlerts := s.generateThresholdAlerts(ctx, allowedTopics)
	for _, alert := range dynamicAlerts {
		if _, ok := result[alert.Topic]; !ok {
			continue
		}
		if dismissed[alert.Id] {
			continue
		}
		alert.DisplayTimeText = formatAlertDisplayTime(alert.CreateTime)
		result[alert.Topic] = append(result[alert.Topic], alert)
	}

	query := g.DB("master").Model("alert_event").Ctx(ctx).Where("status", "active")
	query = query.WhereIn("topic", allowedTopics)
	records, err := query.OrderDesc("create_time").All()
	if err != nil {
		return nil, err
	}

	var dbAlerts []*alertEventRecord
	if err = records.Structs(&dbAlerts); err != nil {
		return nil, err
	}

	for _, item := range dbAlerts {
		if item == nil {
			continue
		}
		if dismissed[item.Id] {
			continue
		}
		if !checkTopicPermission(ruleLevel, item.Topic) {
			continue
		}
		alert := item.toModel()
		alert.DisplayTimeText = formatAlertDisplayTime(alert.CreateTime)
		result[alert.Topic] = append(result[alert.Topic], alert)
	}
	return result, nil
}

// DismissAlert 关闭告警
func (s *sAlert) DismissAlert(ctx context.Context, userId int, alertId int) error {
	now := int(gtime.Timestamp())
	count, err := g.DB("master").Model("alert_user_state").Ctx(ctx).Where("alert_id = ? AND user_id = ?", alertId, userId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model("alert_user_state").Ctx(ctx).
			Where("alert_id = ? AND user_id = ?", alertId, userId).
			Data(g.Map{"status": "dismissed", "update_time": now}).
			Update()
		return err
	}
	_, err = g.DB("master").Model("alert_user_state").Ctx(ctx).Data(g.Map{
		"alert_id":    alertId,
		"user_id":     userId,
		"status":      "dismissed",
		"create_time": now,
		"update_time": now,
	}).Insert()
	return err
}

// AddAlert 添加告警（内部使用，供测试或后续扩展）
func (s *sAlert) AddAlert(ctx context.Context, alert model.AlertItem) {
	now := int(gtime.Timestamp())
	if alert.CreateTime <= 0 {
		alert.CreateTime = now
	}
	if alert.Level == "" {
		alert.Level = "warning"
	}
	if alert.Question == "" {
		alert.Question = alert.Content
	}
	if _, err := g.DB("master").Model("alert_event").Ctx(ctx).Data(g.Map{
		"topic":       alert.Topic,
		"title":       alert.Title,
		"content":     alert.Content,
		"question":    alert.Question,
		"level":       alert.Level,
		"status":      "active",
		"create_time": alert.CreateTime,
		"update_time": now,
	}).Insert(); err != nil {
		consts.Logger.Errorf(ctx, "添加告警失败: %s", err.Error())
	}
}

func CreateAlertTables(ctx context.Context, db gdb.DB) error {
	alertTableSQL := `CREATE TABLE IF NOT EXISTS alert_event (
id INT PRIMARY KEY AUTO_INCREMENT,
topic VARCHAR(32) NOT NULL,
title VARCHAR(128) NOT NULL,
content TEXT,
question TEXT,
level VARCHAR(32) NOT NULL DEFAULT 'warning',
status VARCHAR(32) NOT NULL DEFAULT 'active',
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_topic_status (topic, status),
INDEX idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	userStateTableSQL := `CREATE TABLE IF NOT EXISTS alert_user_state (
id INT PRIMARY KEY AUTO_INCREMENT,
alert_id INT NOT NULL,
user_id INT NOT NULL,
status VARCHAR(32) NOT NULL DEFAULT 'dismissed',
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_alert_user (alert_id, user_id),
INDEX idx_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	if _, err := db.Exec(ctx, alertTableSQL); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, userStateTableSQL); err != nil {
		return err
	}
	return nil
}

func SeedAlertTables(ctx context.Context, db gdb.DB) error {
	count, err := db.Model("alert_event").Ctx(ctx).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := int(gtime.Timestamp())
	for _, alert := range seedAlerts() {
		if alert.CreateTime <= 0 {
			alert.CreateTime = now
		}
		if _, err = db.Model("alert_event").Ctx(ctx).Data(g.Map{
			"topic":       alert.Topic,
			"title":       alert.Title,
			"content":     alert.Content,
			"question":    alert.Question,
			"level":       alert.Level,
			"status":      "active",
			"create_time": alert.CreateTime,
			"update_time": now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func (s *sAlert) getDismissedAlertMap(ctx context.Context, userId int) (map[int]bool, error) {
	dismissed := make(map[int]bool)
	if userId <= 0 {
		return dismissed, nil
	}
	records, err := g.DB("master").Model("alert_user_state").Ctx(ctx).
		Fields("alert_id").
		Where("user_id = ? AND status = ?", userId, "dismissed").
		All()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		dismissed[record["alert_id"].Int()] = true
	}
	return dismissed, nil
}

func formatAlertDisplayTime(createTime int) string {
	diff := int(gtime.Timestamp()) - createTime
	switch {
	case diff < 60:
		return "刚刚"
	case diff < 3600:
		return fmt.Sprintf("%d分钟前", diff/60)
	default:
		hours := diff / 3600
		if hours < 24 {
			return fmt.Sprintf("%d小时前", hours)
		}
		return gtime.NewFromTimeStamp(int64(createTime)).Format("m-d H:i")
	}
}

func getUserRuleLevel(ctx context.Context, userId int) (int, error) {
	if userId <= 0 {
		return 0, nil
	}
	user, err := service.User().GetUserInfoById(ctx, int64(userId))
	if err != nil {
		return 0, nil
	}
	if user == nil {
		return 0, nil
	}
	ruleLevel := user.RuleLevel
	record, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).
		Fields("enabled, rule_level").
		Where("user_id = ?", userId).
		One()
	if err == nil && record != nil {
		if record["enabled"].Val() != nil && record["enabled"].Int() == 0 {
			return 0, nil
		}
		if record["rule_level"].Val() != nil {
			ruleLevel = record["rule_level"].Int()
		}
	}
	return ruleLevel, nil
}

func normalizeTopics(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	return filterKnownTopics(strings.Split(raw, ","))
}

func filterKnownTopics(topics []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(topics))
	for _, item := range topics {
		topic := strings.ToLower(strings.TrimSpace(item))
		if _, ok := topicPermissionMap[topic]; !ok || seen[topic] {
			continue
		}
		seen[topic] = true
		out = append(out, topic)
	}
	return out
}

func filterAllowedTopics(ruleLevel int, topics []string) []string {
	topics = filterKnownTopics(topics)
	if len(topics) == 0 {
		topics = []string{"grid", "population", "traffic"}
	}
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		if checkTopicPermission(ruleLevel, topic) {
			out = append(out, topic)
		}
	}
	return out
}

func (s *sAlert) generateThresholdAlerts(ctx context.Context, topics []string) []model.AlertItem {
	alerts := make([]model.AlertItem, 0)
	now := int(gtime.Timestamp())
	for _, topic := range topics {
		switch topic {
		case "traffic":
			alerts = append(alerts, trafficThresholdAlerts(ctx, now)...)
		case "population":
			alerts = append(alerts, populationThresholdAlerts(ctx, now)...)
		case "grid":
			alerts = append(alerts, gridThresholdAlerts(ctx, now)...)
		}
	}
	return alerts
}

func trafficThresholdAlerts(ctx context.Context, now int) []model.AlertItem {
	today, okToday := dailyTrafficMetric(ctx, "CURDATE()")
	yesterday, okYesterday := dailyTrafficMetric(ctx, "DATE_SUB(CURDATE(), INTERVAL 1 DAY)")
	if !okToday || !okYesterday {
		return nil
	}
	alerts := make([]model.AlertItem, 0, 3)
	if rate := growthRate(today["total"], yesterday["total"]); rate >= 20 {
		alerts = append(alerts, thresholdAlert(-1001, "traffic", "车流异常增长", fmt.Sprintf("今日车流较昨日增长 %.1f%%", rate), "分析一下今日车流异常增长情况", "warning", now))
	}
	if rate := growthRate(today["hk_macau_count"], yesterday["hk_macau_count"]); rate >= 20 {
		alerts = append(alerts, thresholdAlert(-1002, "traffic", "港澳车异常增长", fmt.Sprintf("今日港澳车较昨日增长 %.1f%%", rate), "分析一下今日港澳车流异常增长原因", "warning", now))
	}
	if rate := growthRate(today["foreign_count"], yesterday["foreign_count"]); rate >= 20 {
		alerts = append(alerts, thresholdAlert(-1003, "traffic", "外地车异常增长", fmt.Sprintf("今日外地车较昨日增长 %.1f%%", rate), "分析一下今日外地车来源和停留情况", "warning", now))
	}
	return alerts
}

func populationThresholdAlerts(ctx context.Context, now int) []model.AlertItem {
	today, okToday := dailyPopulationMetric(ctx, "CURDATE()")
	yesterday, okYesterday := dailyPopulationMetric(ctx, "DATE_SUB(CURDATE(), INTERVAL 1 DAY)")
	if !okToday || !okYesterday {
		return nil
	}
	alerts := make([]model.AlertItem, 0, 2)
	if rate := growthRate(today["in_count"]+today["out_count"], yesterday["in_count"]+yesterday["out_count"]); rate >= 30 {
		alerts = append(alerts, thresholdAlert(-2001, "population", "区域人流异常增长", fmt.Sprintf("今日人流较昨日增长 %.1f%%", rate), "分析一下该区域人流异常增长情况", "warning", now))
	}
	if rate := growthRate(today["floating_population_count"], yesterday["floating_population_count"]); rate >= 20 {
		alerts = append(alerts, thresholdAlert(-2002, "population", "流动人口异常增长", fmt.Sprintf("今日流动人口较昨日增长 %.1f%%", rate), "分析一下今日流动人口变化情况", "warning", now))
	}
	return alerts
}

func gridThresholdAlerts(ctx context.Context, now int) []model.AlertItem {
	alerts := make([]model.AlertItem, 0, 2)
	latestMonth, latestTotal, ok := latestGridMonthlyCount(ctx)
	if ok {
		if avg, avgOK := previousGridMonthlyAvg(ctx, latestMonth); avgOK {
			if rate := growthRate(latestTotal, avg); rate >= 30 {
				alerts = append(alerts, thresholdAlert(-3001, "grid", "案件量异常增长", fmt.Sprintf("%s 网格案件数较近 3 个周期均值增长 %.1f%%", latestMonth, rate), "分析一下近期网格案件增长情况", "warning", now))
			}
		}
	}
	record, err := g.DB("master").Model("grid_case_record").Ctx(ctx).
		Fields("COUNT(1) AS count").
		Where("major_score > 0").
		WhereNotIn("case_status", []string{"已办结", "已结案", "closed", "done"}).
		One()
	if err == nil && record != nil && record["count"].Int() > 0 {
		alerts = append(alerts, thresholdAlert(-3002, "grid", "重大案件提醒", "存在未办结重大案件需要关注", "分析一下当前最需要关注的重大案件", "warning", now))
	}
	return alerts
}

func dailyTrafficMetric(ctx context.Context, dateExpr string) (map[string]float64, bool) {
	record, err := g.DB("master").Model("traffic_metric_daily").Ctx(ctx).
		Fields("COALESCE(SUM(total),0) AS total, COALESCE(SUM(hk_macau_count),0) AS hk_macau_count, COALESCE(SUM(foreign_count),0) AS foreign_count").
		Where(fmt.Sprintf("metric_date = %s", dateExpr)).
		One()
	if err != nil || record == nil {
		return nil, false
	}
	return map[string]float64{
		"total":          record["total"].Float64(),
		"hk_macau_count": record["hk_macau_count"].Float64(),
		"foreign_count":  record["foreign_count"].Float64(),
	}, true
}

func dailyPopulationMetric(ctx context.Context, dateExpr string) (map[string]float64, bool) {
	record, err := g.DB("master").Model("population_metric_daily").Ctx(ctx).
		Fields("COALESCE(SUM(in_count),0) AS in_count, COALESCE(SUM(out_count),0) AS out_count, COALESCE(SUM(floating_population_count),0) AS floating_population_count").
		Where(fmt.Sprintf("metric_date = %s", dateExpr)).
		One()
	if err != nil || record == nil {
		return nil, false
	}
	return map[string]float64{
		"in_count":                  record["in_count"].Float64(),
		"out_count":                 record["out_count"].Float64(),
		"floating_population_count": record["floating_population_count"].Float64(),
	}, true
}

func latestGridMonthlyCount(ctx context.Context) (string, float64, bool) {
	record, err := g.DB("master").Model("grid_metric_monthly").Ctx(ctx).
		Fields("metric_month, COALESCE(SUM(case_count),0) AS case_count").
		Group("metric_month").
		OrderDesc("metric_month").
		Limit(1).
		One()
	if err != nil || record == nil {
		return "", 0, false
	}
	month := record["metric_month"].String()
	if month == "" {
		return "", 0, false
	}
	return month, record["case_count"].Float64(), true
}

func previousGridMonthlyAvg(ctx context.Context, latestMonth string) (float64, bool) {
	records, err := g.DB("master").Model("grid_metric_monthly").Ctx(ctx).
		Fields("metric_month, COALESCE(SUM(case_count),0) AS case_count").
		Where("metric_month < ?", latestMonth).
		Group("metric_month").
		OrderDesc("metric_month").
		Limit(3).
		All()
	if err != nil || len(records) == 0 {
		return 0, false
	}
	var total float64
	for _, record := range records {
		total += record["case_count"].Float64()
	}
	return total / float64(len(records)), true
}

func growthRate(current, base float64) float64 {
	if current <= 0 || base <= 0 {
		return 0
	}
	return (current - base) / base * 100
}

func thresholdAlert(id int, topic string, title string, content string, question string, level string, now int) model.AlertItem {
	return model.AlertItem{
		Id:         id,
		Topic:      topic,
		Title:      title,
		Content:    content,
		Question:   question,
		Level:      level,
		CreateTime: now,
	}
}

func seedAlerts() []model.AlertItem {
	return []model.AlertItem{
		{Id: 1, Title: "人流异常增长", Content: "高新区人流异常增长", Question: "分析高新区人流异常增长的原因和近一周进出趋势", Topic: "population", Level: "warning", CreateTime: int(gtime.Timestamp() - 600), Dismissed: false},
		{Id: 2, Title: "重复案件上报", Content: "重复案件上报", Question: "查询网格内重复上报案件，并找出影响最大或处置难度最大的案件", Topic: "grid", Level: "warning", CreateTime: int(gtime.Timestamp() - 3600), Dismissed: false},
		{Id: 3, Title: "车流持续异常", Content: "重点卡口车流持续异常", Question: "分析重点卡口车流持续异常情况，并统计港澳车占比", Topic: "traffic", Level: "critical", CreateTime: int(gtime.Timestamp() - 900), Dismissed: false},
	}
}

type alertEventRecord struct {
	Id         int    `orm:"id"`
	Title      string `orm:"title"`
	Content    string `orm:"content"`
	Question   string `orm:"question"`
	Topic      string `orm:"topic"`
	Level      string `orm:"level"`
	CreateTime int    `orm:"create_time"`
}

func (r *alertEventRecord) toModel() model.AlertItem {
	return model.AlertItem{
		Id:         r.Id,
		Title:      r.Title,
		Content:    r.Content,
		Question:   r.Question,
		Topic:      r.Topic,
		Level:      r.Level,
		CreateTime: r.CreateTime,
	}
}
