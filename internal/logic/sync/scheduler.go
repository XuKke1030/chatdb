package sync

import (
	"context"
	"time"

	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/aidgp"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func StartScheduler(ctx context.Context) {
	interval := consts.Config.Sync.IntervalSeconds
	if interval <= 0 {
		interval = 1800
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Minute):
		}

		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runScheduledSyncs(ctx)
			}
		}
	}()
}

func runScheduledSyncs(ctx context.Context) {
	dataSources := []struct {
		sourceType string
		syncType   string
	}{
		{"traffic", aidgp.SyncTrafficData},
		{"population", aidgp.SyncPopulationData},
		{"grid", aidgp.SyncGridData},
	}

	for _, ds := range dataSources {
		record, err := g.DB("master").Model("admin_data_source").Ctx(ctx).
			Where("source_type = ?", ds.sourceType).One()
		if err != nil || record == nil || record["enabled"].Int() == 0 {
			continue
		}

		cfg := buildAidgpConfig()
		client := aidgp.NewClient(cfg)

		scope := aidgp.SyncScope{}
		if record["latest_sync"].Int() > 0 {
			scope.Since = time.Unix(int64(record["latest_sync"].Int()), 0).Format(time.RFC3339)
		}

		_, syncErr := client.SyncByType(ctx, ds.syncType)
		now := int(gtime.Timestamp())
		statusVal := "ready"
		if syncErr != nil {
			g.Log().Warningf(ctx, "scheduled sync failed for %s: %v", ds.sourceType, syncErr)
			statusVal = "error"
		}

		_, _ = g.DB("master").Model("admin_data_source").Ctx(ctx).
			Where("source_type = ?", ds.sourceType).
			Data(g.Map{
				"latest_sync": now,
				"status":      statusVal,
				"update_time": now,
			}).Update()
	}
}

func buildAidgpConfig() aidgp.Config {
	qa := consts.Config.QaConfig
	if qa == nil || qa.Sync == nil || qa.Sync.Aidgp == nil {
		return aidgp.Config{}
	}
	a := qa.Sync.Aidgp
	return aidgp.Config{
		Provider:                 aidgp.ProviderAidgp,
		BaseUrl:                  a.BaseUrl,
		AppKey:                   a.AppKey,
		AppSecret:                a.AppSecret,
		TokenPath:                a.TokenPath,
		TrafficQueryPath:         a.TrafficQueryPath,
		PopulationQueryPath:      a.PopulationQueryPath,
		GridQueryPath:            a.GridQueryPath,
		KnowledgeBasesPath:       a.KnowledgeBasesPath,
		DocumentsPath:            a.DocumentsPath,
		DocumentSegmentsPath:     a.DocumentSegmentsPath,
		KnowledgePermissionsPath: a.KnowledgePermissionsPath,
		TimeoutSeconds:           a.TimeoutSeconds,
		RetryTimes:               a.RetryTimes,
		TokenExpireSkewSeconds:   a.TokenExpireSkewSeconds,
	}
}
