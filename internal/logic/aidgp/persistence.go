package aidgp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func persistRecord(ctx context.Context, syncType string, item map[string]any, fallback int) SyncLog {
	external := externalId(syncType, item, fallback)
	raw := rawJSON(item)
	if err := upsertRawRecord(ctx, syncType, external, syncVersion(item), raw); err != nil {
		return SyncLog{ExternalId: external, Action: "raw_upsert", Status: "failed", Message: err.Error()}
	}

	switch syncType {
	case SyncKnowledgeBases:
		return persistKnowledgeBase(ctx, external, item)
	case SyncDocuments:
		return persistDocument(ctx, external, item)
	case SyncPermissions:
		return persistKnowledgePermission(ctx, external, item)
	case SyncTrafficData:
		return persistTraffic(ctx, external, item)
	case SyncPopulationData:
		return persistPopulation(ctx, external, item)
	case SyncGridData:
		return persistGrid(ctx, external, item)
	default:
		return SyncLog{ExternalId: external, LocalId: external, Action: "raw_upsert", Status: "success", Message: "AIDGP raw record persisted"}
	}
}

func refreshTrafficAggregates(ctx context.Context) SyncLog {
	if err := service.Traffic().RefreshAggregates(ctx, "", ""); err != nil {
		return SyncLog{Action: "traffic_aggregate_refresh", Status: "failed", Message: err.Error()}
	}
	return SyncLog{Action: "traffic_aggregate_refresh", Status: "success", Message: "traffic aggregate tables refreshed"}
}

func refreshPopulationAggregates(ctx context.Context) SyncLog {
	if err := service.Population().RefreshAggregates(ctx, "", ""); err != nil {
		return SyncLog{Action: "population_aggregate_refresh", Status: "failed", Message: err.Error()}
	}
	return SyncLog{Action: "population_aggregate_refresh", Status: "success", Message: "population aggregate tables refreshed"}
}

func refreshGridMetrics(ctx context.Context) SyncLog {
	if err := rebuildGridMetrics(ctx); err != nil {
		return SyncLog{Action: "grid_metric_refresh", Status: "failed", Message: err.Error()}
	}
	return SyncLog{Action: "grid_metric_refresh", Status: "success", Message: "grid metric tables refreshed"}
}

func upsertRawRecord(ctx context.Context, syncType string, externalId string, version string, raw string) error {
	now := int(gtime.Timestamp())
	data := g.Map{
		"sync_version":   version,
		"payload_hash":   payloadHash(raw),
		"raw_payload":    raw,
		"last_sync_time": now,
		"update_time":    now,
	}
	count, err := g.DB("master").Model("aidgp_sync_record").Ctx(ctx).
		Where("sync_type = ? AND external_id = ?", syncType, externalId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model("aidgp_sync_record").Ctx(ctx).
			Where("sync_type = ? AND external_id = ?", syncType, externalId).Data(data).Update()
		return err
	}
	data["sync_type"] = syncType
	data["external_id"] = externalId
	data["create_time"] = now
	_, err = g.DB("master").Model("aidgp_sync_record").Ctx(ctx).Data(data).Insert()
	return err
}

func persistKnowledgeBase(ctx context.Context, externalId string, item map[string]any) SyncLog {
	now := int(gtime.Timestamp())
	code := stringFieldAny(item, "knowledgeCode", "code")
	if code == "" {
		code = externalId
	}
	enabled := 1
	if status := strings.ToLower(stringFieldAny(item, "status")); status == "disabled" || status == "deleted" {
		enabled = 0
	}
	if boolFieldAny(item, "enabled", "active") {
		enabled = 1
	}
	data := g.Map{
		"name":            firstNonEmpty(stringFieldAny(item, "knowledgeName", "name"), code),
		"description":     stringFieldAny(item, "description"),
		"enabled":         enabled,
		"sort":            intFieldAny(item, "sort"),
		"source_provider": "aidgp",
		"external_id":     externalId,
		"sync_version":    syncVersion(item),
		"last_sync_time":  now,
		"permission_hash": stringFieldAny(item, "permissionHash"),
		"update_time":     now,
	}
	count, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Where("code = ?", code).Count()
	if err != nil {
		return failedLog(externalId, code, "knowledge_upsert", err)
	}
	if count > 0 {
		_, err = g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Where("code = ?", code).Data(data).Update()
	} else {
		data["code"] = code
		data["create_time"] = now
		_, err = g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Data(data).Insert()
	}
	if err != nil {
		return failedLog(externalId, code, "knowledge_upsert", err)
	}
	return SyncLog{ExternalId: externalId, LocalId: code, Action: "knowledge_upsert", Status: "success", Message: "knowledge base persisted"}
}

func persistDocument(ctx context.Context, externalId string, item map[string]any) SyncLog {
	now := int(gtime.Timestamp())
	knowledgeCode := stringFieldAny(item, "knowledgeCode", "code")
	title := stringFieldAny(item, "title", "documentTitle", "name")
	if knowledgeCode == "" || title == "" {
		return SyncLog{ExternalId: externalId, Action: "document_skip", Status: "skipped", Message: "missing knowledgeCode or title"}
	}
	data := g.Map{
		"knowledge_code":  knowledgeCode,
		"title":           title,
		"file_name":       stringFieldAny(item, "fileName"),
		"file_type":       stringFieldAny(item, "fileType"),
		"source_provider": "aidgp",
		"external_id":     externalId,
		"status":          firstNonEmpty(stringFieldAny(item, "status"), "active"),
		"update_time":     now,
	}
	record, err := g.DB("master").Model("qa_document").Ctx(ctx).Fields("id").
		Where("source_provider = ? AND external_id = ?", "aidgp", externalId).One()
	if err != nil {
		return failedLog(externalId, "", "document_upsert", err)
	}
	localId := ""
	if record != nil {
		localId = record["id"].String()
		_, err = g.DB("master").Model("qa_document").Ctx(ctx).Where("id = ?", record["id"].Int64()).Data(data).Update()
	} else {
		data["create_time"] = now
		id, insertErr := g.DB("master").Model("qa_document").Ctx(ctx).Data(data).InsertAndGetId()
		err = insertErr
		localId = fmt.Sprint(id)
	}
	if err != nil {
		return failedLog(externalId, localId, "document_upsert", err)
	}
	return SyncLog{ExternalId: externalId, LocalId: localId, Action: "document_upsert", Status: "success", Message: "document metadata persisted"}
}

func persistKnowledgePermission(ctx context.Context, externalId string, item map[string]any) SyncLog {
	knowledgeCode := stringFieldAny(item, "knowledgeCode", "code")
	if knowledgeCode == "" {
		return SyncLog{ExternalId: externalId, Action: "permission_skip", Status: "skipped", Message: "missing knowledgeCode"}
	}
	// AIDGP can provide permission snapshots before UIAP user ids are mapped; keep
	// the raw payload and mark it for the UIAP permission sync to apply later.
	return SyncLog{ExternalId: externalId, LocalId: knowledgeCode, Action: "permission_raw_upsert", Status: "success", Message: "knowledge permission raw payload persisted"}
}

func persistTraffic(ctx context.Context, externalId string, item map[string]any) SyncLog {
	record, ok := mapTrafficRecord(item)
	if !ok {
		return SyncLog{ExternalId: externalId, Action: "traffic_raw_upsert", Status: "success", Message: "traffic raw payload persisted; record fields incomplete"}
	}
	inserted, err := service.Traffic().SaveGateRecord(ctx, record)
	if err != nil {
		return failedLog(externalId, record.DeviceId, "traffic_upsert", err)
	}
	action := "traffic_skip_duplicate"
	if inserted {
		action = "traffic_insert"
	}
	return SyncLog{ExternalId: externalId, LocalId: record.DeviceId + ":" + record.PlateNormalized, Action: action, Status: "success", Message: "traffic record mapped to local table"}
}

func persistPopulation(ctx context.Context, externalId string, item map[string]any) SyncLog {
	now := int(gtime.Timestamp())
	metricTime := parseTimeField(item, "metricTime", "snapshotTime", "time", "date")
	if metricTime.IsZero() {
		metricTime = gtime.Now().Time
	}
	data := g.Map{
		"sync_version":              syncVersion(item),
		"metric_time":               metricTime.Format("2006-01-02 15:04:05"),
		"region":                    stringFieldAny(item, "region"),
		"grid_name":                 stringFieldAny(item, "gridName"),
		"in_count":                  intFieldAny(item, "inCount", "enterCount"),
		"out_count":                 intFieldAny(item, "outCount", "leaveCount"),
		"net_in_count":              intFieldAny(item, "netInCount"),
		"floating_population_count": intFieldAny(item, "floatingPopulationCount"),
		"source_provider":           "aidgp",
		"raw_payload":               rawJSON(item),
		"update_time":               now,
	}
	err := upsertByExternal(ctx, "population_flow_record", externalId, data, now)
	if err != nil {
		return failedLog(externalId, externalId, "population_upsert", err)
	}
	return SyncLog{ExternalId: externalId, LocalId: externalId, Action: "population_upsert", Status: "success", Message: "population record persisted"}
}

func persistGrid(ctx context.Context, externalId string, item map[string]any) SyncLog {
	now := int(gtime.Timestamp())
	data := g.Map{
		"sync_version":    syncVersion(item),
		"case_number":     stringFieldAny(item, "caseNumber", "caseNo"),
		"region":          stringFieldAny(item, "region"),
		"community":       stringFieldAny(item, "community"),
		"grid_name":       stringFieldAny(item, "gridName"),
		"case_type1":      stringFieldAny(item, "caseType1"),
		"case_type2":      stringFieldAny(item, "caseType2", "caseType"),
		"case_title":      stringFieldAny(item, "caseTitle", "title"),
		"case_status":     stringFieldAny(item, "caseStatus", "status"),
		"report_time":     nullableTime(parseTimeField(item, "reportTime", "createTime")),
		"close_time":      nullableTime(parseTimeField(item, "closeTime", "finishTime")),
		"major_score":     floatFieldAny(item, "majorScore"),
		"source_provider": "aidgp",
		"raw_payload":     rawJSON(item),
		"update_time":     now,
	}
	err := upsertByExternal(ctx, "grid_case_record", externalId, data, now)
	if err != nil {
		return failedLog(externalId, externalId, "grid_upsert", err)
	}
	return SyncLog{ExternalId: externalId, LocalId: externalId, Action: "grid_upsert", Status: "success", Message: "grid case record persisted"}
}

func rebuildGridMetrics(ctx context.Context) error {
	db := g.DB("master")
	if _, err := db.Exec(ctx, "DELETE FROM grid_metric_daily WHERE source_provider = ?", "aidgp"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, "DELETE FROM grid_metric_monthly WHERE source_provider = ?", "aidgp"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `
INSERT INTO grid_metric_daily (
metric_date, region, community, grid_name, case_type1, case_type2,
case_count, closed_count, close_rate, avg_handle_hours, source_provider, update_time
)
SELECT
DATE(COALESCE(report_time, FROM_UNIXTIME(create_time))) AS metric_date,
COALESCE(region, '') AS region,
COALESCE(community, '') AS community,
COALESCE(grid_name, '') AS grid_name,
COALESCE(case_type1, '') AS case_type1,
COALESCE(case_type2, '') AS case_type2,
COUNT(*) AS case_count,
SUM(CASE WHEN LOWER(COALESCE(case_status, '')) IN ('closed','done','finished','resolved') OR case_status LIKE '%结案%' OR case_status LIKE '%办结%' THEN 1 ELSE 0 END) AS closed_count,
CASE WHEN COUNT(*) = 0 THEN 0 ELSE SUM(CASE WHEN LOWER(COALESCE(case_status, '')) IN ('closed','done','finished','resolved') OR case_status LIKE '%结案%' OR case_status LIKE '%办结%' THEN 1 ELSE 0 END) / COUNT(*) END AS close_rate,
AVG(CASE WHEN report_time IS NOT NULL AND close_time IS NOT NULL THEN TIMESTAMPDIFF(HOUR, report_time, close_time) ELSE 0 END) AS avg_handle_hours,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM grid_case_record
GROUP BY metric_date, region, community, grid_name, case_type1, case_type2`); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
INSERT INTO grid_metric_monthly (
metric_month, region, community, grid_name, case_type1, case_type2,
case_count, closed_count, close_rate, avg_handle_hours, source_provider, update_time
)
SELECT
DATE_FORMAT(COALESCE(report_time, FROM_UNIXTIME(create_time)), '%Y-%m') AS metric_month,
COALESCE(region, '') AS region,
COALESCE(community, '') AS community,
COALESCE(grid_name, '') AS grid_name,
COALESCE(case_type1, '') AS case_type1,
COALESCE(case_type2, '') AS case_type2,
COUNT(*) AS case_count,
SUM(CASE WHEN LOWER(COALESCE(case_status, '')) IN ('closed','done','finished','resolved') OR case_status LIKE '%结案%' OR case_status LIKE '%办结%' THEN 1 ELSE 0 END) AS closed_count,
CASE WHEN COUNT(*) = 0 THEN 0 ELSE SUM(CASE WHEN LOWER(COALESCE(case_status, '')) IN ('closed','done','finished','resolved') OR case_status LIKE '%结案%' OR case_status LIKE '%办结%' THEN 1 ELSE 0 END) / COUNT(*) END AS close_rate,
AVG(CASE WHEN report_time IS NOT NULL AND close_time IS NOT NULL THEN TIMESTAMPDIFF(HOUR, report_time, close_time) ELSE 0 END) AS avg_handle_hours,
'aidgp' AS source_provider,
UNIX_TIMESTAMP() AS update_time
FROM grid_case_record
GROUP BY metric_month, region, community, grid_name, case_type1, case_type2`)
	return err
}

func upsertByExternal(ctx context.Context, table string, externalId string, data g.Map, now int) error {
	count, err := g.DB("master").Model(table).Ctx(ctx).Where("external_id = ?", externalId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model(table).Ctx(ctx).Where("external_id = ?", externalId).Data(data).Update()
		return err
	}
	data["external_id"] = externalId
	data["create_time"] = now
	_, err = g.DB("master").Model(table).Ctx(ctx).Data(data).Insert()
	return err
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02 15:04:05")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func failedLog(externalId string, localId string, action string, err error) SyncLog {
	return SyncLog{ExternalId: externalId, LocalId: localId, Action: action, Status: "failed", Message: err.Error()}
}
