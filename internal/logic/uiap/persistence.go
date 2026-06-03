package uiap

import (
	"context"
	"fmt"
	"strings"

	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/integration"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func CreateTables(ctx context.Context) error {
	db := g.DB("master")
	statements := []string{
		`CREATE TABLE IF NOT EXISTS uiap_user_binding (
id INT PRIMARY KEY AUTO_INCREMENT,
external_user_id VARCHAR(128) NOT NULL,
user_id INT NOT NULL,
username VARCHAR(64) NOT NULL,
display_name VARCHAR(128),
org_code VARCHAR(64),
org_name VARCHAR(128),
permission_hash VARCHAR(128),
last_permission_sync_at INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_external_user (external_user_id),
UNIQUE KEY uk_user_id (user_id),
INDEX idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS uiap_sync_state (
id INT PRIMARY KEY AUTO_INCREMENT,
sync_key VARCHAR(64) NOT NULL,
sync_value VARCHAR(255) NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_sync_key (sync_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func ApplyUserPermissions(ctx context.Context, info *UserInfo, permission *PermissionData) (int64, error) {
	if info == nil {
		info = &UserInfo{}
	}
	externalUserId := strings.TrimSpace(info.UserId)
	if permission != nil && strings.TrimSpace(permission.UserId) != "" {
		externalUserId = strings.TrimSpace(permission.UserId)
	}
	username := strings.TrimSpace(info.Username)
	if username == "" {
		username = externalUserId
	}
	if externalUserId == "" || username == "" {
		return 0, fmt.Errorf("uiap user id or username is empty")
	}

	now := int(gtime.Timestamp())
	userId, err := ensureLocalUser(ctx, externalUserId, username, permissionRuleLevel(permission), now)
	if err != nil {
		return 0, err
	}
	if err = upsertBinding(ctx, externalUserId, userId, username, info, permission, now); err != nil {
		return 0, err
	}
	if err = upsertProfile(ctx, userId, info, permission, now); err != nil {
		return 0, err
	}
	if err = replaceKnowledgePermissions(ctx, userId, permission, now); err != nil {
		return 0, err
	}
	return userId, nil
}

func ensureLocalUser(ctx context.Context, externalUserId string, username string, ruleLevel int, now int) (int64, error) {
	if record, err := g.DB("master").Model("uiap_user_binding").Ctx(ctx).Fields("user_id").
		Where("external_user_id = ?", externalUserId).One(); err == nil && record != nil {
		return record["user_id"].Int64(), nil
	}

	if record, err := g.DB("master").Model("user").Ctx(ctx).Fields("user_id").
		Where("username = ?", username).One(); err == nil && record != nil {
		userId := record["user_id"].Int64()
		if _, err := g.DB("master").Model("user").Ctx(ctx).Where("user_id = ?", userId).Data(g.Map{
			"rule_level":  ruleLevel,
			"update_time": now,
		}).Update(); err != nil {
			consts.Logger.Warningf(ctx, "uiap update user rule_level failed: %v", err)
		}
		return userId, nil
	}

	password, _ := integration.RandomHex(24)
	return g.DB("master").Model("user").Ctx(ctx).Data(g.Map{
		"username":    username,
		"password":    "uiap:" + password,
		"verify":      0,
		"rule_level":  ruleLevel,
		"create_time": now,
		"update_time": now,
	}).InsertAndGetId()
}

func upsertBinding(ctx context.Context, externalUserId string, userId int64, username string, info *UserInfo, permission *PermissionData, now int) error {
	data := g.Map{
		"external_user_id":        externalUserId,
		"user_id":                 userId,
		"username":                username,
		"display_name":            info.DisplayName,
		"org_code":                info.OrgCode,
		"org_name":                info.OrgName,
		"permission_hash":         "",
		"last_permission_sync_at": now,
		"update_time":             now,
	}
	if permission != nil {
		data["permission_hash"] = permission.PermissionHash
	}
	count, err := g.DB("master").Model("uiap_user_binding").Ctx(ctx).Where("external_user_id = ?", externalUserId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model("uiap_user_binding").Ctx(ctx).Where("external_user_id = ?", externalUserId).Data(data).Update()
		return err
	}
	data["create_time"] = now
	_, err = g.DB("master").Model("uiap_user_binding").Ctx(ctx).Data(data).Insert()
	return err
}

func upsertProfile(ctx context.Context, userId int64, info *UserInfo, permission *PermissionData, now int) error {
	enabled := 1
	if permission != nil && !permission.Enabled {
		enabled = 0
	}
	version := 1
	if permission != nil && permission.PermissionVersion > 0 {
		version = permission.PermissionVersion
	}
	data := g.Map{
		"department":         info.OrgName,
		"display_name":       info.DisplayName,
		"enabled":            enabled,
		"rule_level":         permissionRuleLevel(permission),
		"permission_version": version,
		"last_login_at":      now,
		"update_time":        now,
	}
	count, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).Data(data).Update()
		return err
	}
	data["user_id"] = userId
	data["create_time"] = now
	_, err = g.DB("master").Model("admin_user_profile").Ctx(ctx).Data(data).Insert()
	return err
}

func replaceKnowledgePermissions(ctx context.Context, userId int64, permission *PermissionData, now int) error {
	if permission == nil {
		return nil
	}
	if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).
		Where("user_id = ?", userId).Delete(); err != nil {
		return err
	}
	for _, item := range permission.KnowledgePermissions {
		code := normalizeKnowledgeCode(item.KnowledgeCode)
		if code == "" {
			continue
		}
		enabled := 0
		if item.Enabled {
			enabled = 1
		}
		if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Data(g.Map{
			"user_id":        userId,
			"knowledge_code": code,
			"enabled":        enabled,
			"create_time":    now,
			"update_time":    now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func permissionRuleLevel(permission *PermissionData) int {
	if permission == nil {
		return 0
	}
	level := 0
	for _, item := range permission.TopicPermissions {
		if !item.Enabled {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.Topic)) {
		case "grid":
			level |= 1
		case "population", "people", "person":
			level |= 2
		case "traffic", "vehicle":
			level |= 4
		}
	}
	return level
}

func normalizeKnowledgeCode(code string) string {
	switch strings.TrimSpace(code) {
	case "policy_files":
		return "policy"
	case "laws":
		return "manual"
	default:
		return strings.TrimSpace(code)
	}
}
