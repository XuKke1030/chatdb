package uiap

import (
	"context"
	"strings"
	"time"

	"ai-chat-sql/internal/consts"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

const permissionSyncStateKey = "uiap_permission_updated_after"

func ConfigFromSystem() Config {
	cfg := consts.Config.Uiap
	if cfg == nil {
		return Config{}
	}
	return Config{
		Enabled:                cfg.Enabled,
		BaseUrl:                cfg.BaseUrl,
		ClientId:               cfg.ClientId,
		ClientSecret:           cfg.ClientSecret,
		TokenPath:              cfg.TokenPath,
		UserInfoPath:           cfg.UserInfoPath,
		PermissionPath:         cfg.PermissionPath,
		BatchPermissionPath:    cfg.BatchPermissionPath,
		TimeoutSeconds:         cfg.TimeoutSeconds,
		TokenExpireSkewSeconds: cfg.TokenExpireSkewSeconds,
	}
}

func EnabledConfigFromSystem() (Config, bool) {
	cfg := ConfigFromSystem()
	ok := cfg.Enabled && strings.TrimSpace(cfg.BaseUrl) != "" && strings.TrimSpace(cfg.ClientId) != "" && strings.TrimSpace(cfg.ClientSecret) != ""
	return cfg, ok
}

func StartPermissionPoller(ctx context.Context) {
	cfg, ok := EnabledConfigFromSystem()
	if !ok {
		return
	}
	interval := consts.Config.Uiap.PermissionPollIntervalSeconds
	if interval <= 0 {
		interval = 300
	}
	pageSize := consts.Config.Uiap.PermissionPollPageSize
	if pageSize <= 0 {
		pageSize = 500
	}

	client := NewClient(cfg)
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := PollPermissions(ctx, client, pageSize); err != nil {
					g.Log().Warningf(ctx, "uiap permission poll failed: %v", err)
				}
			}
		}
	}()
}

func PollPermissions(ctx context.Context, client *Client, pageSize int) error {
	updatedAfter := readSyncState(ctx, permissionSyncStateKey)
	page := 1
	for {
		resp, err := client.BatchPermissions(ctx, updatedAfter, page, pageSize)
		if err != nil {
			return err
		}
		for _, item := range resp.Data.List {
			info := &UserInfo{UserId: item.UserId, Enabled: item.Enabled}
			if _, err = ApplyUserPermissions(ctx, info, &item); err != nil {
				return err
			}
		}
		if !resp.Data.HasMore {
			break
		}
		page++
	}
	return writeSyncState(ctx, permissionSyncStateKey, time.Now().Format(time.RFC3339))
}

func readSyncState(ctx context.Context, key string) string {
	record, err := g.DB("master").Model("uiap_sync_state").Ctx(ctx).Fields("sync_value").Where("sync_key = ?", key).One()
	if err != nil || record == nil {
		return ""
	}
	return record["sync_value"].String()
}

func writeSyncState(ctx context.Context, key string, value string) error {
	now := int(gtime.Timestamp())
	count, err := g.DB("master").Model("uiap_sync_state").Ctx(ctx).Where("sync_key = ?", key).Count()
	if err != nil {
		return err
	}
	data := g.Map{"sync_value": value, "update_time": now}
	if count > 0 {
		_, err = g.DB("master").Model("uiap_sync_state").Ctx(ctx).Where("sync_key = ?", key).Data(data).Update()
		return err
	}
	data["sync_key"] = key
	_, err = g.DB("master").Model("uiap_sync_state").Ctx(ctx).Data(data).Insert()
	return err
}
