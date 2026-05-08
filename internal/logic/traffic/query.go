package traffic

import (
	"ai-chat-sql/internal/model"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

func (s *sTraffic) Aggregate(ctx context.Context, query model.TrafficAggregateQuery) (*model.TrafficAggregateResult, error) {
	groupBy := normalizeTrafficGroupBy(query.GroupBy)
	db := g.DB("master")
	dbType := dbType(db)

	base := applyTrafficRecordFilters(db.Model("traffic_gate_record").Ctx(ctx), query.DateFrom, query.DateTo, query.DeviceId, query.Plate, "")
	summaryRecord, err := base.Fields(trafficCountFields()).One()
	if err != nil {
		return nil, err
	}
	summary := aggregateSummaryFromRecord(summaryRecord)

	groupExpr := trafficGroupExpr(dbType, groupBy)
	seriesRecords, err := applyTrafficRecordFilters(db.Model("traffic_gate_record").Ctx(ctx), query.DateFrom, query.DateTo, query.DeviceId, query.Plate, "").
		Fields(fmt.Sprintf("%s AS name, %s", groupExpr, trafficCountFields())).
		Group("name").
		Order(trafficGroupOrder(groupBy)).
		Limit(200).
		All()
	if err != nil {
		return nil, err
	}
	series := make([]model.TrafficAggregateSeriesItem, 0, len(seriesRecords))
	for _, record := range seriesRecords {
		series = append(series, model.TrafficAggregateSeriesItem{
			Name:         record["name"].String(),
			Total:        record["total"].Int(),
			InCount:      record["in_count"].Int(),
			OutCount:     record["out_count"].Int(),
			HkMacauCount: record["hk_macau_count"].Int(),
		})
	}
	return &model.TrafficAggregateResult{
		Summary: summary,
		Series:  series,
		GroupBy: groupBy,
	}, nil
}

func (s *sTraffic) ListRecords(ctx context.Context, query model.TrafficRecordQuery) (*model.TrafficRecordListResult, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	db := g.DB("master")
	base := applyTrafficRecordFilters(db.Model("traffic_gate_record").Ctx(ctx), query.DateFrom, query.DateTo, query.DeviceId, query.Plate, query.IsHkMacau)
	total, err := base.Count()
	if err != nil {
		return nil, err
	}
	records, err := applyTrafficRecordFilters(db.Model("traffic_gate_record").Ctx(ctx), query.DateFrom, query.DateTo, query.DeviceId, query.Plate, query.IsHkMacau).
		OrderDesc("snapshot_time").
		OrderDesc("id").
		Limit((page-1)*pageSize, pageSize).
		All()
	if err != nil {
		return nil, err
	}
	list := make([]model.TrafficRecordItem, 0, len(records))
	for _, record := range records {
		inDir := record["in_dir"].Int()
		list = append(list, model.TrafficRecordItem{
			Id:               record["id"].Int64(),
			DeviceId:         record["device_id"].String(),
			DeviceName:       record["device_name"].String(),
			CameraIp:         record["camera_ip"].String(),
			PlateChar:        record["plate_char"].String(),
			PlateNormalized:  record["plate_normalized"].String(),
			PlateType:        record["plate_type"].String(),
			PlateColor:       record["plate_color"].String(),
			VehicleType:      record["vehicle_type"].String(),
			VehicleTypeExt:   record["vehicle_type_ext"].String(),
			VehicleColor:     record["vehicle_color"].String(),
			VehicleSpeed:     record["vehicle_speed"].Int(),
			InDir:            inDir,
			InDirName:        inDirName(inDir),
			VehicleDir:       record["vehicle_dir"].String(),
			CarDrvDir:        record["car_drv_dir"].String(),
			LaneId:           record["lane_id"].Int(),
			SnapshotTime:     record["snapshot_time"].String(),
			CollectTime:      record["collect_time"].String(),
			SourceInsertTime: record["source_insert_time"].String(),
			PlateOrigin:      record["plate_origin"].String(),
			PlateRegionType:  record["plate_region_type"].String(),
			IsHkMacau:        record["is_hk_macau"].Int() == 1,
			PlatePicture:     record["plate_picture"].String(),
			PanoramaPicture:  record["panorama_picture"].String(),
			VehiclePicture:   record["vehicle_picture"].String(),
		})
	}
	return &model.TrafficRecordListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *sTraffic) ListDevices(ctx context.Context, query model.TrafficDeviceQuery) (*model.TrafficDeviceListResult, error) {
	db := g.DB("master")
	m := db.Model("traffic_gate_device").Ctx(ctx)
	if enabled, ok := parseBoolFilter(query.Enabled); ok {
		m = m.Where("enabled = ?", boolInt(enabled))
	}
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		m = m.Where("(device_id LIKE ? OR device_sn LIKE ? OR device_name LIKE ? OR address LIKE ? OR region LIKE ?)", like, like, like, like, like)
	}
	records, err := m.OrderDesc("latest_seen_at").OrderAsc("device_name").All()
	if err != nil {
		return nil, err
	}
	list := make([]model.TrafficDeviceItem, 0, len(records))
	for _, record := range records {
		list = append(list, model.TrafficDeviceItem{
			Id:               record["id"].Int64(),
			DeviceId:         record["device_id"].String(),
			DeviceSn:         record["device_sn"].String(),
			DeviceName:       record["device_name"].String(),
			Category:         record["category"].String(),
			Model:            record["model"].String(),
			ConnectionStatus: record["connection_status"].String(),
			WorkStatus:       record["work_status"].String(),
			Region:           record["region"].String(),
			Address:          record["address"].String(),
			Longitude:        record["longitude"].Float64(),
			Latitude:         record["latitude"].Float64(),
			OwnerName:        record["owner_name"].String(),
			OwnerPhone:       record["owner_phone"].String(),
			Enabled:          record["enabled"].Int() == 1,
			LatestSeenAt:     record["latest_seen_at"].String(),
			LastReportedAt:   record["last_reported_at"].String(),
			DeviceCreatedAt:  record["device_created_at"].String(),
			Remark:           record["remark"].String(),
		})
	}
	return &model.TrafficDeviceListResult{List: list}, nil
}

func applyTrafficRecordFilters(m *gdb.Model, dateFrom string, dateTo string, deviceId string, plate string, isHkMacau string) *gdb.Model {
	if from := normalizeQueryTime(dateFrom, false); from != "" {
		m = m.Where("snapshot_time >= ?", from)
	}
	if to := normalizeQueryTime(dateTo, true); to != "" {
		m = m.Where("snapshot_time <= ?", to)
	}
	if strings.TrimSpace(deviceId) != "" {
		m = m.Where("device_id = ?", strings.TrimSpace(deviceId))
	}
	if strings.TrimSpace(plate) != "" {
		normalized := normalizePlate(plate)
		m = m.Where("(plate_normalized = ? OR plate_char = ?)", normalized, strings.TrimSpace(plate))
	}
	if enabled, ok := parseBoolFilter(isHkMacau); ok {
		m = m.Where("is_hk_macau = ?", boolInt(enabled))
	}
	return m
}

func trafficCountFields() string {
	return `COUNT(*) AS total,
SUM(CASE WHEN in_dir = 0 THEN 1 ELSE 0 END) AS in_count,
SUM(CASE WHEN in_dir = 1 THEN 1 ELSE 0 END) AS out_count,
SUM(CASE WHEN in_dir <> 0 AND in_dir <> 1 THEN 1 ELSE 0 END) AS unknown_dir_count,
SUM(CASE WHEN is_hk_macau = 1 THEN 1 ELSE 0 END) AS hk_macau_count`
}

func aggregateSummaryFromRecord(record gdb.Record) model.TrafficAggregateSummary {
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

func normalizeTrafficGroupBy(groupBy string) string {
	switch strings.ToLower(strings.TrimSpace(groupBy)) {
	case "hour", "day", "gate", "plateregion", "indir":
		return strings.ToLower(strings.TrimSpace(groupBy))
	default:
		return "day"
	}
}

func trafficGroupExpr(dbType string, groupBy string) string {
	switch groupBy {
	case "hour":
		if dbType == "sqlite" {
			return "substr(snapshot_time, 1, 13) || ':00'"
		}
		return "DATE_FORMAT(snapshot_time, '%Y-%m-%d %H:00')"
	case "gate":
		return "COALESCE(NULLIF(device_name, ''), device_id)"
	case "plateregion":
		return "COALESCE(NULLIF(plate_region_type, ''), 'unknown')"
	case "indir":
		return "CASE WHEN in_dir = 0 THEN '进' WHEN in_dir = 1 THEN '出' ELSE '未知' END"
	default:
		if dbType == "sqlite" {
			return "substr(snapshot_time, 1, 10)"
		}
		return "DATE(snapshot_time)"
	}
}

func trafficGroupOrder(groupBy string) string {
	switch groupBy {
	case "gate", "plateregion", "indir":
		return "total DESC"
	default:
		return "name ASC"
	}
}

func normalizeQueryTime(value string, endOfDay bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) == len("2006-01-02") {
		if endOfDay {
			return value + " 23:59:59"
		}
		return value + " 00:00:00"
	}
	if len(value) == len("2006-01-02 15:04") {
		return value + ":00"
	}
	return value
}

func parseBoolFilter(value string) (bool, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false, false
	}
	switch value {
	case "1", "true", "yes", "y", "是":
		return true, true
	case "0", "false", "no", "n", "否":
		return false, true
	default:
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed, true
		}
		return false, false
	}
}

func inDirName(inDir int) string {
	switch inDir {
	case 0:
		return "进"
	case 1:
		return "出"
	default:
		return "未知"
	}
}
