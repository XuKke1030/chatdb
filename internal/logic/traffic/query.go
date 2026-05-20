package traffic

import (
	"ai-chat-sql/internal/logic/plate"
	"ai-chat-sql/internal/model"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func (s *sTraffic) Aggregate(ctx context.Context, query model.TrafficAggregateQuery) (*model.TrafficAggregateResult, error) {
	groupBy := normalizeTrafficGroupBy(query.GroupBy)
	if result, ok, err := s.aggregateFromMetrics(ctx, query, groupBy); err != nil {
		return nil, err
	} else if ok {
		return result, nil
	}
	db := g.DB("master")

	base := applyTrafficRecordFilters(db.Model("traffic_gate_record").Ctx(ctx), query.DateFrom, query.DateTo, query.DeviceId, query.Plate, "")
	base = applyTrafficExtraFilters(base, query.GateName, query.PlateRegion)
	summaryRecord, err := base.Fields(trafficCountFields()).One()
	if err != nil {
		return nil, err
	}
	summary := aggregateSummaryFromRecord(summaryRecord)

	groupExpr := trafficGroupExpr(groupBy)
	seriesModel := applyTrafficRecordFilters(db.Model("traffic_gate_record").Ctx(ctx), query.DateFrom, query.DateTo, query.DeviceId, query.Plate, "")
	seriesModel = applyTrafficExtraFilters(seriesModel, query.GateName, query.PlateRegion)
	seriesRecords, err := seriesModel.
		Fields(fmt.Sprintf("%s AS name, %s, SUM(CASE WHEN LEFT(plate_normalized,1)='粤' AND is_hk_macau=0 THEN 1 ELSE 0 END) AS province_inside_count, SUM(CASE WHEN is_hk_macau=0 AND (LEFT(plate_normalized,1)<>'粤') THEN 1 ELSE 0 END) AS province_outside_count", groupExpr, trafficCountFields())).
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
			Name:                record["name"].String(),
			Total:               record["total"].Int(),
			InCount:             record["in_count"].Int(),
			OutCount:            record["out_count"].Int(),
			HkMacauCount:        record["hk_macau_count"].Int(),
			ProvinceInsideCount:  record["province_inside_count"].Int(),
			ProvinceOutsideCount: record["province_outside_count"].Int(),
		})
	}
	return &model.TrafficAggregateResult{
		Summary: summary,
		Series:  series,
		GroupBy: groupBy,
	}, nil
}

func (s *sTraffic) aggregateFromMetrics(ctx context.Context, query model.TrafficAggregateQuery, groupBy string) (*model.TrafficAggregateResult, bool, error) {
	if strings.TrimSpace(query.Plate) != "" || groupBy == "plateregion" || groupBy == "indir" {
		return nil, false, nil
	}
	table := "traffic_metric_daily"
	timeColumn := "metric_date"
	nameExpr := "DATE(metric_date)"
	orderBy := "name ASC"
	if groupBy == "hour" {
		table = "traffic_metric_hourly"
		timeColumn = "metric_hour"
		nameExpr = "DATE_FORMAT(metric_hour, '%Y-%m-%d %H:00')"
	} else if groupBy == "gate" {
		nameExpr = "COALESCE(NULLIF(device_name, ''), device_id)"
		orderBy = "total DESC"
	}

	db := g.DB("master")
	base := applyTrafficMetricFilters(db.Model(table).Ctx(ctx), timeColumn, query)
	count, err := base.Count()
	if err != nil {
		return nil, false, err
	}
	if count == 0 {
		return nil, false, nil
	}

	summaryRecord, err := applyTrafficMetricFilters(db.Model(table).Ctx(ctx), timeColumn, query).
		Fields(`SUM(total) AS total,
SUM(in_count) AS in_count,
SUM(out_count) AS out_count,
SUM(hk_macau_count) AS hk_macau_count,
SUM(mainland_count) AS mainland_count,
SUM(total - in_count - out_count) AS unknown_dir_count,
SUM(province_inside_count) AS province_inside_count,
SUM(province_outside_count) AS province_outside_count`).One()
	if err != nil {
		return nil, false, err
	}
	summary := aggregateSummaryFromRecord(summaryRecord)
	if summary.MainlandCount == 0 && summary.Total > summary.HkMacauCount {
		summary.MainlandCount = summary.Total - summary.HkMacauCount
	}

	records, err := applyTrafficMetricFilters(db.Model(table).Ctx(ctx), timeColumn, query).
		Fields(fmt.Sprintf("%s AS name, SUM(total) AS total, SUM(in_count) AS in_count, SUM(out_count) AS out_count, SUM(hk_macau_count) AS hk_macau_count, SUM(province_inside_count) AS province_inside_count, SUM(province_outside_count) AS province_outside_count", nameExpr)).
		Group("name").
		Order(orderBy).
		Limit(200).
		All()
	if err != nil {
		return nil, false, err
	}
	series := make([]model.TrafficAggregateSeriesItem, 0, len(records))
	for _, record := range records {
		series = append(series, model.TrafficAggregateSeriesItem{
			Name:                record["name"].String(),
			Total:               record["total"].Int(),
			InCount:             record["in_count"].Int(),
			OutCount:            record["out_count"].Int(),
			HkMacauCount:        record["hk_macau_count"].Int(),
			ProvinceInsideCount:  record["province_inside_count"].Int(),
			ProvinceOutsideCount: record["province_outside_count"].Int(),
		})
	}
	return &model.TrafficAggregateResult{Summary: summary, Series: series, GroupBy: groupBy}, true, nil
}

// StayDistribution returns stay time distribution buckets for a date range.
func (s *sTraffic) StayDistribution(ctx context.Context, dateFrom, dateTo, region, isHkMacau string) ([]model.TrafficStayBucketItem, error) {
	db := g.DB("master")
	m := db.Model("traffic_stay_distribution_daily").Ctx(ctx)
	if dateFrom != "" {
		m = m.Where("metric_date >= ?", normalizeQueryDateOnly(dateFrom))
	}
	if dateTo != "" {
		m = m.Where("metric_date <= ?", normalizeQueryDateOnly(dateTo))
	}
	if region != "" {
		m = m.Where("region LIKE ?", "%"+strings.TrimSpace(region)+"%")
	}
	records, err := m.Fields("bucket, SUM(vehicle_count) AS vehicle_count, AVG(avg_stay_minutes) AS avg_stay_minutes").
		Group("bucket").
		Order("bucket ASC").
		All()
	if err != nil {
		return nil, err
	}
	items := make([]model.TrafficStayBucketItem, 0, len(records))
	for _, r := range records {
		items = append(items, model.TrafficStayBucketItem{
			Bucket:        r["bucket"].String(),
			VehicleCount:  r["vehicle_count"].Int(),
			AvgStayMinutes: r["avg_stay_minutes"].Float64(),
		})
	}
	return items, nil
}

// OriginRank returns vehicle origin ranking by province/city for a date range.
func (s *sTraffic) OriginRank(ctx context.Context, dateFrom, dateTo string, topN int) ([]model.TrafficOriginRankItem, error) {
	if topN <= 0 {
		topN = 10
	}
	db := g.DB("master")
	m := db.Model("traffic_origin_daily").Ctx(ctx)
	if dateFrom != "" {
		m = m.Where("metric_date >= ?", normalizeQueryDateOnly(dateFrom))
	}
	if dateTo != "" {
		m = m.Where("metric_date <= ?", normalizeQueryDateOnly(dateTo))
	}
	records, err := m.Fields("province, city, SUM(vehicle_count) AS vehicle_count, MAX(is_hk_macau) AS is_hk_macau").
		Group("province, city").
		OrderDesc("vehicle_count").
		Limit(topN).
		All()
	if err != nil {
		return nil, err
	}
	items := make([]model.TrafficOriginRankItem, 0, len(records))
	for _, r := range records {
		items = append(items, model.TrafficOriginRankItem{
			Province:     r["province"].String(),
			City:         r["city"].String(),
			VehicleCount: r["vehicle_count"].Int(),
			IsHkMacau:    r["is_hk_macau"].Int() == 1,
		})
	}
	return items, nil
}

// YoYCompare returns year-over-year comparison for a date range and groupBy.
func (s *sTraffic) YoYCompare(ctx context.Context, dateFrom, dateTo, groupBy string) ([]model.TrafficYoYCompareItem, error) {
	groupBy = normalizeTrafficGroupBy(groupBy)
	now := gtime.Now()
	if dateFrom == "" {
		dateFrom = now.AddDate(0, 0, -6).Format("Y-m-d")
	}
	if dateTo == "" {
		dateTo = now.Format("Y-m-d") + " 23:59:59"
	}
	// Compute same period last year
	parsedFrom, _ := gtime.StrToTime(dateFrom)
	parsedTo, _ := gtime.StrToTime(dateTo)
	priorFrom := parsedFrom.AddDate(-1, 0, 0).Format("Y-m-d")
	priorTo := parsedTo.AddDate(-1, 0, 0).Format("Y-m-d 15:04:05")

	db := g.DB("master")
	// Current period
	curRecords, err := s.aggregateGrouped(ctx, db, dateFrom, dateTo, groupBy)
	if err != nil {
		return nil, err
	}
	// Prior period
	priorRecords, err := s.aggregateGrouped(ctx, db, priorFrom, priorTo, groupBy)
	if err != nil {
		return nil, err
	}
	priorMap := make(map[string]int, len(priorRecords))
	for _, r := range priorRecords {
		priorMap[r.Name] = r.Total
	}
	items := make([]model.TrafficYoYCompareItem, 0, len(curRecords))
	for _, r := range curRecords {
		priorVal := priorMap[r.Name]
		changePct := 0.0
		if priorVal > 0 {
			changePct = float64(r.Total-priorVal) / float64(priorVal) * 100
		}
		items = append(items, model.TrafficYoYCompareItem{
			Name:       r.Name,
			CurrentVal: r.Total,
			PriorVal:   priorVal,
			ChangePct:  changePct,
		})
	}
	return items, nil
}

// MoMCompare returns month-over-month comparison.
func (s *sTraffic) MoMCompare(ctx context.Context, dateFrom, dateTo, groupBy string) ([]model.TrafficYoYCompareItem, error) {
	groupBy = normalizeTrafficGroupBy(groupBy)
	now := gtime.Now()
	if dateFrom == "" {
		dateFrom = now.AddDate(0, -1, 0).Format("Y-m-d")
	}
	if dateTo == "" {
		dateTo = now.Format("Y-m-d") + " 23:59:59"
	}
	parsedFrom, _ := gtime.StrToTime(dateFrom)
	parsedTo, _ := gtime.StrToTime(dateTo)
	priorFrom := parsedFrom.AddDate(0, -1, 0).Format("Y-m-d")
	priorTo := parsedTo.AddDate(0, -1, 0).Format("Y-m-d 15:04:05")

	db := g.DB("master")
	curRecords, err := s.aggregateGrouped(ctx, db, dateFrom, dateTo, groupBy)
	if err != nil {
		return nil, err
	}
	priorRecords, err := s.aggregateGrouped(ctx, db, priorFrom, priorTo, groupBy)
	if err != nil {
		return nil, err
	}
	priorMap := make(map[string]int, len(priorRecords))
	for _, r := range priorRecords {
		priorMap[r.Name] = r.Total
	}
	items := make([]model.TrafficYoYCompareItem, 0, len(curRecords))
	for _, r := range curRecords {
		priorVal := priorMap[r.Name]
		changePct := 0.0
		if priorVal > 0 {
			changePct = float64(r.Total-priorVal) / float64(priorVal) * 100
		}
		items = append(items, model.TrafficYoYCompareItem{
			Name:       r.Name,
			CurrentVal: r.Total,
			PriorVal:   priorVal,
			ChangePct:  changePct,
		})
	}
	return items, nil
}

// HolidayTraffic returns aggregate traffic for a given holiday period.
func (s *sTraffic) HolidayTraffic(ctx context.Context, holidayName string) (*model.TrafficAggregateResult, error) {
	db := g.DB("master")
	// Find the holiday date range
	records, err := db.Model("traffic_holiday").Ctx(ctx).
		Where("holiday_name LIKE ? AND holiday_type = 'holiday'", "%"+strings.TrimSpace(holidayName)+"%").
		OrderAsc("holiday_date").All()
	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("未找到节假日「%s」的配置", holidayName)
	}
	dateFrom := records[0]["holiday_date"].String()
	dateTo := records[len(records)-1]["holiday_date"].String()
	return s.Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: dateFrom,
		DateTo:   dateTo + " 23:59:59",
		GroupBy:  "day",
	})
}

// aggregateGrouped is a helper for YoY/MoM that returns grouped totals.
func (s *sTraffic) aggregateGrouped(ctx context.Context, db gdb.DB, dateFrom, dateTo, groupBy string) ([]model.TrafficAggregateSeriesItem, error) {
	result, err := s.Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		GroupBy:  groupBy,
	})
	if err != nil {
		return nil, err
	}
	return result.Series, nil
}

func applyTrafficMetricFilters(m *gdb.Model, timeColumn string, query model.TrafficAggregateQuery) *gdb.Model {
	if from := normalizeQueryTime(query.DateFrom, false); from != "" {
		if timeColumn == "metric_date" {
			m = m.Where("metric_date >= DATE(?)", from)
		} else {
			m = m.Where(timeColumn+" >= ?", from)
		}
	}
	if to := normalizeQueryTime(query.DateTo, true); to != "" {
		if timeColumn == "metric_date" {
			m = m.Where("metric_date <= DATE(?)", to)
		} else {
			m = m.Where(timeColumn+" <= ?", to)
		}
	}
	if strings.TrimSpace(query.DeviceId) != "" {
		m = m.Where("device_id = ?", strings.TrimSpace(query.DeviceId))
	}
	if gateName := strings.TrimSpace(query.GateName); gateName != "" {
		m = m.Where("device_name LIKE ?", "%"+gateName+"%")
	}
	if region := strings.TrimSpace(query.PlateRegion); region != "" {
		m = m.Where("(region LIKE ? OR device_name LIKE ?)", "%"+region+"%", "%"+region+"%")
	}
	return m
}

func applyTrafficExtraFilters(m *gdb.Model, gateName string, plateRegion string) *gdb.Model {
	if gateName := strings.TrimSpace(gateName); gateName != "" {
		m = m.Where("device_name LIKE ?", "%"+gateName+"%")
	}
	if plateRegion := strings.TrimSpace(plateRegion); plateRegion != "" {
		m = m.Where("(plate_region_type LIKE ? OR device_name LIKE ?)", "%"+plateRegion+"%", "%"+plateRegion+"%")
	}
	return m
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

func applyTrafficRecordFilters(m *gdb.Model, dateFrom string, dateTo string, deviceId string, plateStr string, isHkMacau string) *gdb.Model {
	if from := normalizeQueryTime(dateFrom, false); from != "" {
		m = m.Where("snapshot_time >= ?", from)
	}
	if to := normalizeQueryTime(dateTo, true); to != "" {
		m = m.Where("snapshot_time <= ?", to)
	}
	if strings.TrimSpace(deviceId) != "" {
		m = m.Where("device_id = ?", strings.TrimSpace(deviceId))
	}
	if strings.TrimSpace(plateStr) != "" {
		normalized := plate.NormalizePlate(plateStr)
		m = m.Where("(plate_normalized = ? OR plate_char = ?)", normalized, strings.TrimSpace(plateStr))
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
	hkRatio := 0.0
	if total > 0 {
		hkRatio = float64(hkMacau) / float64(total)
	}
	insideCount := record["province_inside_count"].Int()
	outsideCount := record["province_outside_count"].Int()
	insideRatio := 0.0
	mainland := total - hkMacau
	if mainland > 0 {
		insideRatio = float64(insideCount) / float64(mainland)
	}
	return model.TrafficAggregateSummary{
		Total:               total,
		InCount:             record["in_count"].Int(),
		OutCount:            record["out_count"].Int(),
		HkMacauCount:        hkMacau,
		HkMacauRatio:        hkRatio,
		MainlandCount:       mainland,
		UnknownDirCount:     record["unknown_dir_count"].Int(),
		ProvinceInsideCount:  insideCount,
		ProvinceOutsideCount: outsideCount,
		ProvinceInsideRatio:  insideRatio,
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

func trafficGroupExpr(groupBy string) string {
	switch groupBy {
	case "hour":
		return "DATE_FORMAT(snapshot_time, '%Y-%m-%d %H:00')"
	case "gate":
		return "COALESCE(NULLIF(device_name, ''), device_id)"
	case "plateregion":
		return "COALESCE(NULLIF(plate_region_type, ''), 'unknown')"
	case "indir":
		return "CASE WHEN in_dir = 0 THEN '进' WHEN in_dir = 1 THEN '出' ELSE '未知' END"
	default:
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
