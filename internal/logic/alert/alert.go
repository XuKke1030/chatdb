package alert

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"
	"sync"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func init() {
	service.RegisterAlert(NewAlert())
}

func NewAlert() *sAlert {
	s := &sAlert{
		alerts:          seedAlerts(),
		dismissedByUser: make(map[int]map[int]bool),
		nextId:          3,
	}
	return s
}

type sAlert struct {
	mu              sync.RWMutex
	alerts          []model.AlertItem
	dismissedByUser map[int]map[int]bool
	nextId          int
	ensureMu        sync.Mutex
	tablesReady     bool
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
	if err := s.ensureTables(ctx); err != nil {
		consts.Logger.Errorf(ctx, "告警表初始化失败，使用内存告警降级: %s", err.Error())
		return s.getMemoryAlertList(ctx, userId, topic)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 获取用户权限等级
	ruleLevel, err := getUserRuleLevel(ctx, userId)
	if err != nil {
		return nil, err
	}

	query := g.DB("master").Model("alert_event").Ctx(ctx).Where("status", "active")
	if topic != "" {
		query = query.Where("topic", topic)
	}
	records, err := query.OrderDesc("create_time").All()
	if err != nil {
		return nil, err
	}

	var dbAlerts []*alertEventRecord
	if err = records.Structs(&dbAlerts); err != nil {
		return nil, err
	}

	dismissed, err := s.getDismissedAlertMap(ctx, userId)
	if err != nil {
		return nil, err
	}

	var result []model.AlertItem
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
		result = append(result, alert)
	}
	return result, nil
}

func (s *sAlert) getMemoryAlertList(ctx context.Context, userId int, topic string) ([]model.AlertItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ruleLevel, err := getUserRuleLevel(ctx, userId)
	if err != nil {
		return nil, err
	}

	var result []model.AlertItem
	for _, alert := range s.alerts {
		if alert.Dismissed || s.isDismissedByUser(userId, alert.Id) {
			continue
		}
		if topic != "" && alert.Topic != topic {
			continue
		}
		if !checkTopicPermission(ruleLevel, alert.Topic) {
			continue
		}
		alert.DisplayTimeText = formatAlertDisplayTime(alert.CreateTime)
		result = append(result, alert)
	}
	return result, nil
}

// DismissAlert 关闭告警
func (s *sAlert) DismissAlert(ctx context.Context, userId int, alertId int) error {
	if err := s.ensureTables(ctx); err != nil {
		consts.Logger.Errorf(ctx, "告警表初始化失败，使用内存关闭降级: %s", err.Error())
		return s.dismissMemoryAlert(userId, alertId)
	}
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

func (s *sAlert) dismissMemoryAlert(userId int, alertId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dismissedByUser[userId] == nil {
		s.dismissedByUser[userId] = make(map[int]bool)
	}
	s.dismissedByUser[userId][alertId] = true
	return nil
}

// AddAlert 添加告警（内部使用，供测试或后续扩展）
func (s *sAlert) AddAlert(ctx context.Context, alert model.AlertItem) {
	if err := s.ensureTables(ctx); err == nil {
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
		if _, err = g.DB("master").Model("alert_event").Ctx(ctx).Data(g.Map{
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
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextId++
	alert.Id = s.nextId
	alert.CreateTime = int(gtime.Timestamp())
	s.alerts = append(s.alerts, alert)
}

func (s *sAlert) ensureTables(ctx context.Context) error {
	s.ensureMu.Lock()
	defer s.ensureMu.Unlock()

	if s.tablesReady {
		return nil
	}

	db := g.DB("master")
	if err := createAlertTables(ctx, db); err != nil {
		return err
	}
	if err := s.seedDatabaseAlerts(ctx, db); err != nil {
		return err
	}

	s.tablesReady = true
	return nil
}

func createAlertTables(ctx context.Context, db gdb.DB) error {
	dbType := ""
	if cfg := db.GetConfig(); cfg != nil {
		dbType = cfg.Type
	}

	var alertTableSQL string
	var userStateTableSQL string
	switch dbType {
	case "sqlite":
		alertTableSQL = `CREATE TABLE IF NOT EXISTS alert_event (
id INTEGER PRIMARY KEY AUTOINCREMENT,
topic TEXT NOT NULL,
title TEXT NOT NULL,
content TEXT,
question TEXT,
level TEXT NOT NULL DEFAULT 'warning',
status TEXT NOT NULL DEFAULT 'active',
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL
)`
		userStateTableSQL = `CREATE TABLE IF NOT EXISTS alert_user_state (
id INTEGER PRIMARY KEY AUTOINCREMENT,
alert_id INTEGER NOT NULL,
user_id INTEGER NOT NULL,
status TEXT NOT NULL DEFAULT 'dismissed',
create_time INTEGER NOT NULL,
update_time INTEGER NOT NULL,
UNIQUE(alert_id, user_id)
)`
	default:
		alertTableSQL = `CREATE TABLE IF NOT EXISTS alert_event (
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
		userStateTableSQL = `CREATE TABLE IF NOT EXISTS alert_user_state (
id INT PRIMARY KEY AUTO_INCREMENT,
alert_id INT NOT NULL,
user_id INT NOT NULL,
status VARCHAR(32) NOT NULL DEFAULT 'dismissed',
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_alert_user (alert_id, user_id),
INDEX idx_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	}

	if _, err := db.Exec(ctx, alertTableSQL); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, userStateTableSQL); err != nil {
		return err
	}
	return nil
}

func (s *sAlert) seedDatabaseAlerts(ctx context.Context, db gdb.DB) error {
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

func (s *sAlert) isDismissedByUser(userId int, alertId int) bool {
	if s.dismissedByUser[userId] == nil {
		return false
	}
	return s.dismissedByUser[userId][alertId]
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
		return 7, nil
	}
	user, err := service.User().GetUserInfoById(ctx, int64(userId))
	if err != nil {
		return 7, nil
	}
	if user == nil {
		return 7, nil
	}
	return user.RuleLevel, nil
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
