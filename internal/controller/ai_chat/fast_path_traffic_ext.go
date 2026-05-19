package ai_chat

import (
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func fastTrafficProvinceInside(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom:   from,
		DateTo:     to,
		GroupBy:    "day",
		GateName:   p.GateName,
		PlateRegion: p.RegionName,
	})
	if err != nil {
		return "", "", err
	}
	s := result.Summary
	insideRatio := 0.0
	if s.MainlandCount > 0 {
		insideRatio = float64(s.ProvinceInsideCount) / float64(s.MainlandCount) * 100
	}
	outsideRatio := 100.0 - insideRatio
	answer := fmt.Sprintf("近七天车流中，省内车 %d 辆（占比%.1f%%），省外车 %d 辆（占比%.1f%%）。",
		s.ProvinceInsideCount, insideRatio, s.ProvinceOutsideCount, outsideRatio)
	return answer, "", nil
}

func fastTrafficYoY(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().YoYCompare(ctx, from, to, "day")
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return "暂无同比数据。", "", nil
	}
	xLabels := make([]string, 0, len(items))
	curData := make([]int, 0, len(items))
	priorData := make([]int, 0, len(items))
	tableRows := make([][]any, 0, len(items))
	for _, it := range items {
		xLabels = append(xLabels, it.Name)
		curData = append(curData, it.CurrentVal)
		priorData = append(priorData, it.PriorVal)
		tableRows = append(tableRows, []any{it.Name, it.CurrentVal, it.PriorVal, fmt.Sprintf("%.1f%%", it.ChangePct)})
	}
	chart := buildChart("bar", "车流同比对比", xLabels,
		[]chartSeries{{Name: "今年", Data: curData}, {Name: "去年", Data: priorData}},
		[]string{"日期", "今年", "去年", "变化率"}, tableRows)
	first := items[0]
	dir := "增长"
	if first.ChangePct < 0 {
		dir = "下降"
	}
	answer := fmt.Sprintf("近七天车流同比%s %.1f%%。", dir, first.ChangePct)
	if first.ChangePct >= 0 && first.ChangePct < 1 {
		answer = "近七天车流与去年同期基本持平。"
	}
	return answer, chart, nil
}

func fastTrafficMoM(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, -1, 0).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().MoMCompare(ctx, from, to, "day")
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return "暂无环比数据。", "", nil
	}
	xLabels := make([]string, 0, len(items))
	curData := make([]int, 0, len(items))
	priorData := make([]int, 0, len(items))
	tableRows := make([][]any, 0, len(items))
	for _, it := range items {
		xLabels = append(xLabels, it.Name)
		curData = append(curData, it.CurrentVal)
		priorData = append(priorData, it.PriorVal)
		tableRows = append(tableRows, []any{it.Name, it.CurrentVal, it.PriorVal, fmt.Sprintf("%.1f%%", it.ChangePct)})
	}
	chart := buildChart("bar", "车流环比对比", xLabels,
		[]chartSeries{{Name: "本月", Data: curData}, {Name: "上月", Data: priorData}},
		[]string{"日期", "本月", "上月", "变化率"}, tableRows)
	first := items[0]
	dir := "增长"
	if first.ChangePct < 0 {
		dir = "下降"
	}
	answer := fmt.Sprintf("近一个月车流环比%s %.1f%%。", dir, first.ChangePct)
	if first.ChangePct >= 0 && first.ChangePct < 1 {
		answer = "近一个月车流与上月基本持平。"
	}
	return answer, chart, nil
}

func fastTrafficHoliday(ctx context.Context, p ExtractedParams) (string, string, error) {
	holidayName := p.HolidayName
	if holidayName == "" {
		holidayName = "国庆"
	}
	result, err := service.Traffic().HolidayTraffic(ctx, holidayName)
	if err != nil {
		return "", "", err
	}
	if result == nil || len(result.Series) == 0 {
		return fmt.Sprintf("暂无%s期间的车流数据。", holidayName), "", nil
	}
	s := result.Summary
	answer := fmt.Sprintf("%s期间车流总计 %d 辆，日均 %.0f 辆。",
		holidayName, s.Total, float64(s.Total)/float64(len(result.Series)))
	xLabels := make([]string, 0, len(result.Series))
	totalData := make([]int, 0, len(result.Series))
	tableRows := make([][]any, 0, len(result.Series))
	for _, item := range result.Series {
		xLabels = append(xLabels, item.Name)
		totalData = append(totalData, item.Total)
		tableRows = append(tableRows, []any{item.Name, item.Total, item.InCount, item.OutCount})
	}
	chart := buildChart("line", holidayName+"期间车流趋势", xLabels,
		[]chartSeries{{Name: "车流量", Data: totalData}},
		[]string{"日期", "合计", "进入", "离开"}, tableRows)
	return answer, chart, nil
}

func fastTrafficStayDistribution(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d")
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().StayDistribution(ctx, from, to, p.RegionName)
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return "近七天暂无停留时长分布数据。", "", nil
	}
	xLabels := make([]string, 0, len(items))
	countData := make([]int, 0, len(items))
	tableRows := make([][]any, 0, len(items))
	var totalVehicles int
	for _, it := range items {
		xLabels = append(xLabels, it.Bucket)
		countData = append(countData, it.VehicleCount)
		totalVehicles += it.VehicleCount
		tableRows = append(tableRows, []any{it.Bucket, it.VehicleCount, fmt.Sprintf("%.1f", it.AvgStayMinutes)})
	}
	chart := buildChart("bar", "停留时长分布", xLabels,
		[]chartSeries{{Name: "车辆数", Data: countData}},
		[]string{"时段", "车辆数", "平均停留(分钟)"}, tableRows)
	topBucket := items[0].Bucket
	for _, it := range items {
		if it.VehicleCount > 0 {
			topBucket = it.Bucket
			break
		}
	}
	answer := fmt.Sprintf("近七天共有 %d 辆车有停留记录，主要集中在%s时段。", totalVehicles, topBucket)
	return answer, chart, nil
}

func fastTrafficOriginByProvince(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d")
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().OriginRank(ctx, from, to, 10)
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return "近七天暂无外省来源数据。", "", nil
	}
	xLabels := make([]string, 0, len(items))
	countData := make([]int, 0, len(items))
	tableRows := make([][]any, 0, len(items))
	for i, it := range items {
		label := it.Province
		if it.City != "" {
			label = it.Province + "-" + it.City
		}
		xLabels = append(xLabels, label)
		countData = append(countData, it.VehicleCount)
		tableRows = append(tableRows, []any{fmt.Sprintf("%d", i+1), label, it.VehicleCount})
	}
	chart := buildChart("bar", "外省车来源排名 Top10", xLabels,
		[]chartSeries{{Name: "车流量", Data: countData}},
		[]string{"排名", "来源地", "车流量"}, tableRows)
	top := items[0]
	topLabel := top.Province
	if top.City != "" {
		topLabel = top.Province + "-" + top.City
	}
	answer := fmt.Sprintf("近七天省外车主要来自「%s」，共 %d 辆。", topLabel, top.VehicleCount)
	return answer, chart, nil
}

// ---- Composite overview fast paths ----

func fastTrafficOverview(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	// 1. Summary aggregate
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "day",
	})
	if err != nil {
		return "", "", err
	}
	s := result.Summary
	// 2. Top gate
	topGate := ""
	topGateCount := 0
	for _, item := range result.Series {
		if item.Total > topGateCount {
			topGateCount = item.Total
			topGate = item.Name
		}
	}
	// 3. HK/Macau ratio
	hkRatio := 0.0
	if s.Total > 0 {
		hkRatio = float64(s.HkMacauCount) / float64(s.Total) * 100
	}
	// 4. Province inside ratio
	insideRatio := 0.0
	if s.MainlandCount > 0 {
		insideRatio = float64(s.ProvinceInsideCount) / float64(s.MainlandCount) * 100
	}
	answer := fmt.Sprintf("近七天车流总计 %d 辆，日均 %.0f 辆。港澳车 %d 辆（占比%.1f%%），省内车 %d 辆（占大陆车%.1f%%）。",
		s.Total, float64(s.Total)/7.0, s.HkMacauCount, hkRatio, s.ProvinceInsideCount, insideRatio)
	if topGate != "" {
		answer += fmt.Sprintf("车流最大卡口为「%s」（%d辆）。", topGate, topGateCount)
	}
	// Build trend chart
	xLabels := make([]string, 0, len(result.Series))
	totalData := make([]int, 0, len(result.Series))
	inData := make([]int, 0, len(result.Series))
	outData := make([]int, 0, len(result.Series))
	tableRows := make([][]any, 0, len(result.Series))
	for _, item := range result.Series {
		xLabels = append(xLabels, item.Name)
		totalData = append(totalData, item.Total)
		inData = append(inData, item.InCount)
		outData = append(outData, item.OutCount)
		tableRows = append(tableRows, []any{item.Name, item.Total, item.InCount, item.OutCount})
	}
	chart := buildChart("line", "近七天车流趋势", xLabels,
		[]chartSeries{{Name: "合计", Data: totalData}, {Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
		[]string{"日期", "合计", "进入", "离开"}, tableRows)
	return answer, chart, nil
}

func fastPopOverview(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	tableName := discoverPopTable(ctx, db)
	if tableName == "" {
		return "当前未接入人流数据，无法直接查询，请联系管理员开通人流数据接入。", "", nil
	}
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	records, err := db.Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT DATE(snapshot_time) AS day,
		       SUM(CASE WHEN direction = 'in' OR in_dir = 0 THEN 1 ELSE 0 END) AS in_count,
		       SUM(CASE WHEN direction = 'out' OR in_dir = 1 THEN 1 ELSE 0 END) AS out_count,
		       COUNT(*) AS total
		FROM %s
		WHERE snapshot_time >= ? AND snapshot_time <= ?
		GROUP BY day ORDER BY day`, tableName), from, to).All()
	if err != nil || len(records) == 0 {
		return "近七天暂无人流数据。", "", nil
	}
	var totalAll int
	xLabels := make([]string, 0, len(records))
	totalData := make([]int, 0, len(records))
	inData := make([]int, 0, len(records))
	outData := make([]int, 0, len(records))
	tableRows := make([][]any, 0, len(records))
	for _, r := range records {
		day := r["day"].String()
		ic := r["in_count"].Int()
		oc := r["out_count"].Int()
		t := r["total"].Int()
		totalAll += t
		xLabels = append(xLabels, day)
		totalData = append(totalData, t)
		inData = append(inData, ic)
		outData = append(outData, oc)
		tableRows = append(tableRows, []any{day, ic, oc, t})
	}
	chart := buildChart("line", "近七天人流趋势", xLabels,
		[]chartSeries{{Name: "合计", Data: totalData}, {Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
		[]string{"日期", "进入", "离开", "合计"}, tableRows)
	answer := fmt.Sprintf("近七天人流总计 %d 人次，日均 %.0f 人次。", totalAll, float64(totalAll)/float64(len(records)))
	return answer, chart, nil
}
