package ai_chat

import (
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
	items, err := service.Traffic().StayDistribution(ctx, from, to, p.RegionName, "")
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
	// Build multiple charts: trend line + gate rank bar + province inside pie
	charts := make([]string, 0, 3)

	// Chart 1: Trend line
	xLabels := make([]string, 0, len(result.Series))
	totalData := make([]int, 0, len(result.Series))
	inData := make([]int, 0, len(result.Series))
	outData := make([]int, 0, len(result.Series))
	trendRows := make([][]any, 0, len(result.Series))
	for _, item := range result.Series {
		xLabels = append(xLabels, item.Name)
		totalData = append(totalData, item.Total)
		inData = append(inData, item.InCount)
		outData = append(outData, item.OutCount)
		trendRows = append(trendRows, []any{item.Name, item.Total, item.InCount, item.OutCount})
	}
	if len(xLabels) > 0 {
		charts = append(charts, buildChart("line", "近七天车流趋势", xLabels,
			[]chartSeries{{Name: "合计", Data: totalData}, {Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
			[]string{"日期", "合计", "进入", "离开"}, trendRows))
	}

	// Chart 2: Gate rank bar (sort series by total desc)
	type gateItem struct {
		Name  string
		Total int
	}
	gates := make([]gateItem, 0, len(result.Series))
	for _, item := range result.Series {
		gates = append(gates, gateItem{Name: item.Name, Total: item.Total})
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].Total > gates[j].Total })
	if len(gates) > 5 {
		gates = gates[:5]
	}
	gateX := make([]string, 0, len(gates))
	gateData := make([]int, 0, len(gates))
	gateRows := make([][]any, 0, len(gates))
	for i, g := range gates {
		gateX = append(gateX, g.Name)
		gateData = append(gateData, g.Total)
		gateRows = append(gateRows, []any{fmt.Sprintf("%d", i+1), g.Name, g.Total})
	}
	if len(gateX) > 0 {
		charts = append(charts, buildChart("bar", "卡口车流排名 Top5", gateX,
			[]chartSeries{{Name: "车流量", Data: gateData}},
			[]string{"排名", "卡口", "车流量"}, gateRows))
	}

	// Chart 3: Province inside pie
	if s.MainlandCount > 0 {
		pieX := []string{"省内车", "省外车"}
		pieData := []int{s.ProvinceInsideCount, s.MainlandCount - s.ProvinceInsideCount}
		pieRows := [][]any{
			{"省内车", s.ProvinceInsideCount, fmt.Sprintf("%.1f%%", insideRatio)},
			{"省外车", s.MainlandCount - s.ProvinceInsideCount, fmt.Sprintf("%.1f%%", 100-insideRatio)},
		}
		charts = append(charts, buildChart("pie", "省内/省外车占比", pieX,
			[]chartSeries{{Name: "车辆数", Data: pieData}},
			[]string{"类别", "车辆数", "占比"}, pieRows))
	}

	// Combine all charts
	chartData := ""
	for _, c := range charts {
		chartData += c + "\n\n"
	}
	return answer, chartData, nil
}

func fastPopOverview(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}

	result, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "day",
	})
	if err != nil {
		return "", "", err
	}
	if result == nil || len(result.Series) == 0 {
		return "近七天暂无人流数据。", "", nil
	}

	s := result.Summary
	answer := fmt.Sprintf("近七天人流总计进入 %d 人次，离开 %d 人次，净流入 %d 人次。流动人口 %d 人。",
		s.TotalInCount, s.TotalOutCount, s.TotalNetInCount, s.TotalFloatingPopulation)

	// Build multiple charts
	charts := make([]string, 0, 3)

	// Chart 1: Trend line
	xLabels := make([]string, 0, len(result.Series))
	inData := make([]int, 0, len(result.Series))
	outData := make([]int, 0, len(result.Series))
	trendRows := make([][]any, 0, len(result.Series))
	for _, item := range result.Series {
		xLabels = append(xLabels, item.Name)
		inData = append(inData, item.InCount)
		outData = append(outData, item.OutCount)
		trendRows = append(trendRows, []any{item.Name, item.InCount, item.OutCount, item.NetInCount, item.FloatingPopulation})
	}
	if len(xLabels) > 0 {
		charts = append(charts, buildChart("line", "近七天人流趋势", xLabels,
			[]chartSeries{{Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
			[]string{"日期", "进入", "离开", "净流入", "流动人口"}, trendRows))
	}

	// Chart 2: Region rank bar
	regionResult, regionErr := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "region",
	})
	if regionErr == nil && regionResult != nil && len(regionResult.Series) > 0 {
		type regionItem struct {
			Name    string
			InCount int
		}
		regions := make([]regionItem, 0, len(regionResult.Series))
		for _, item := range regionResult.Series {
			regions = append(regions, regionItem{Name: item.Name, InCount: item.InCount})
		}
		sort.Slice(regions, func(i, j int) bool { return regions[i].InCount > regions[j].InCount })
		if len(regions) > 5 {
			regions = regions[:5]
		}
		regionX := make([]string, 0, len(regions))
		regionData := make([]int, 0, len(regions))
		regionRows := make([][]any, 0, len(regions))
		for i, r := range regions {
			regionX = append(regionX, r.Name)
			regionData = append(regionData, r.InCount)
			regionRows = append(regionRows, []any{fmt.Sprintf("%d", i+1), r.Name, r.InCount})
		}
		if len(regionX) > 0 {
			charts = append(charts, buildChart("bar", "区域人流排名 Top5", regionX,
				[]chartSeries{{Name: "进入人次", Data: regionData}},
				[]string{"排名", "区域", "进入人次"}, regionRows))
		}
	}

	// Combine all charts
	chartData := ""
	for _, c := range charts {
		chartData += c + "\n\n"
	}
	return answer, chartData, nil
}

func fastPopYoYQuery(ctx context.Context) (string, string, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)

	thisFrom := now.AddDate(0, 0, -6).Format("2006-01-02")
	thisTo := now.Format("2006-01-02")

	items, err := service.Population().YoYCompare(ctx, thisFrom, thisTo, "day")
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return "当前周期内未查询到人流数据，无法计算同比。", "", nil
	}

	thisTotal := 0
	for _, it := range items {
		thisTotal += it.CurrentVal
	}

	priorFrom := now.AddDate(-1, 0, -6).Format("2006-01-02")
	priorTo := now.AddDate(-1, 0, 0).Format("2006-01-02")
	priorItems, _ := service.Population().YoYCompare(ctx, priorFrom, priorTo, "day")

	var lastTotal int
	if len(priorItems) > 0 {
		for _, it := range priorItems {
			lastTotal += it.PriorVal
		}
	} else {
		return fmt.Sprintf("近七天人流总计 %d 人次，但去年同期无数据，无法计算同比。", thisTotal), "", nil
	}

	var rate float64
	if lastTotal > 0 {
		rate = float64(thisTotal-lastTotal) / float64(lastTotal) * 100
	}

	xLabels := make([]string, 0, len(items))
	curData := make([]int, 0, len(items))
	priorData := make([]int, 0, len(items))
	for _, it := range items {
		xLabels = append(xLabels, it.Name)
		curData = append(curData, it.CurrentVal)
		priorData = append(priorData, it.PriorVal)
	}

	chart := buildChart("bar_compare", "人流同比（今年 vs 去年同期）", xLabels,
		[]chartSeries{{Name: "今年", Data: curData}, {Name: "去年同期", Data: priorData}},
		[]string{"日期", "今年人流", "去年同期人流"}, nil)

	direction := "上升"
	if rate < 0 {
		direction = "下降"
	}
	answer := fmt.Sprintf("近七天人流同比%s %.1f%%（今年 %d 人次，去年同期 %d 人次）。", direction, absF(rate), thisTotal, lastTotal)
	return answer, chart, nil
}

func fastPopMultiRegionCompareQuery(ctx context.Context) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d") + " 23:59:59"

	result, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "region",
	})
	if err != nil {
		return "近七天暂无分区域人流数据。", "", nil
	}
	if result == nil || len(result.Series) == 0 {
		return "近七天暂无分区域人流数据。", "", nil
	}

	// Group by region, aggregate totals
	regionTotals := make(map[string]int)
	for _, item := range result.Series {
		regionTotals[item.Name] += item.InCount + item.OutCount
	}

	type kv struct {
		Region string
		Total  int
	}
	var sorted []kv
	for r, t := range regionTotals {
		sorted = append(sorted, kv{r, t})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Total > sorted[j].Total })
	if len(sorted) > 5 {
		sorted = sorted[:5]
	}

	series := make([]chartSeries, 0, len(sorted))
	xLabelsSet := make(map[string]bool)
	var dates []string
	// Re-query for top 5 regions with day grouping to get trend
	for _, s := range sorted {
		regionResult, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
			DateFrom: from,
			DateTo:   to,
			Region:   s.Region,
			GroupBy:  "day",
		})
		if err != nil || regionResult == nil {
			continue
		}
		data := make([]int, 0, len(regionResult.Series))
		for _, item := range regionResult.Series {
			if !xLabelsSet[item.Name] {
				xLabelsSet[item.Name] = true
				dates = append(dates, item.Name)
			}
			data = append(data, item.InCount+item.OutCount)
		}
		series = append(series, chartSeries{Name: s.Region, Data: data})
	}

	chart := buildChart("line", "近七天各区域人流趋势对比", dates, series,
		[]string{"日期", "区域", "人流总量"}, nil)

	answer := fmt.Sprintf("近七天人流最多的区域为「%s」，共 %d 人次。", sorted[0].Region, sorted[0].Total)
	return answer, chart, nil
}

func fastPopFloatingAnomalyQuery(ctx context.Context) (string, string, error) {
	db := g.DB("master")
	count, err := db.Model("population_floating_daily").Ctx(ctx).Count()
	if err != nil || count == 0 {
		return fastPopFloatingFromMetric(ctx)
	}

	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d")

	records, err := db.Ctx(ctx).Raw(
		"SELECT region, SUM(floating_population_count) AS total FROM population_floating_daily WHERE metric_date >= ? AND metric_date <= ? GROUP BY region ORDER BY total DESC LIMIT 10",
		from, to).All()
	if err != nil || len(records) == 0 {
		return "近七天暂无流动人口数据。", "", nil
	}

	prevFrom := gtime.Now().AddDate(0, 0, -13).Format("Y-m-d")
	prevTo := gtime.Now().AddDate(0, 0, -7).Format("Y-m-d")

	prevRecords, _ := db.Ctx(ctx).Raw(
		"SELECT region, SUM(floating_population_count) AS total FROM population_floating_daily WHERE metric_date >= ? AND metric_date <= ? GROUP BY region",
		prevFrom, prevTo).All()
	prevMap := make(map[string]int)
	for _, r := range prevRecords {
		prevMap[r["region"].String()] = r["total"].Int()
	}

	xLabels := make([]string, 0, len(records))
	totalData := make([]int, 0, len(records))
	anomalyRegions := make([]string, 0)
	tableRows := make([][]any, 0, len(records))
	for _, r := range records {
		region := r["region"].String()
		if region == "" {
			region = "未知区域"
		}
		total := r["total"].Int()
		xLabels = append(xLabels, region)
		totalData = append(totalData, total)

		prevTotal := prevMap[region]
		var growthPct float64
		if prevTotal > 0 {
			growthPct = float64(total-prevTotal) / float64(prevTotal) * 100
		}
		tableRows = append(tableRows, []any{region, total, prevTotal, fmt.Sprintf("%.1f%%", growthPct)})
		if growthPct >= 20 {
			anomalyRegions = append(anomalyRegions, fmt.Sprintf("「%s」增长 %.1f%%", region, growthPct))
		}
	}

	chart := buildChart("bar", "近七天流动人口排名", xLabels,
		[]chartSeries{{Name: "流动人口", Data: totalData}},
		[]string{"区域", "本周流动人口", "上周流动人口", "增长率"}, tableRows)

	answer := fmt.Sprintf("近七天流动人口最多的区域为「%s」，共 %d 人。", xLabels[0], totalData[0])
	if len(anomalyRegions) > 0 {
		answer += "异常增长区域：" + strings.Join(anomalyRegions, "、") + "。"
	} else {
		answer += "各区域流动人口增长均在 20% 以内，未发现异常增长。"
	}
	return answer, chart, nil
}

func fastPopFloatingFromMetric(ctx context.Context) (string, string, error) {
	db := g.DB("master")
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d")

	records, err := db.Ctx(ctx).Raw(
		"SELECT region, SUM(floating_population_count) AS total FROM population_metric_daily WHERE metric_date >= ? AND metric_date <= ? GROUP BY region ORDER BY total DESC LIMIT 10",
		from, to).All()
	if err != nil || len(records) == 0 {
		return "近七天暂无流动人口数据。", "", nil
	}

	xLabels := make([]string, 0, len(records))
	totalData := make([]int, 0, len(records))
	tableRows := make([][]any, 0, len(records))
	for _, r := range records {
		region := r["region"].String()
		if region == "" {
			region = "未知区域"
		}
		total := r["total"].Int()
		xLabels = append(xLabels, region)
		totalData = append(totalData, total)
		tableRows = append(tableRows, []any{region, total})
	}

	chart := buildChart("bar", "近七天流动人口排名", xLabels,
		[]chartSeries{{Name: "流动人口", Data: totalData}},
		[]string{"区域", "流动人口"}, tableRows)

	answer := fmt.Sprintf("近七天流动人口最多的区域为「%s」，共 %d 人。", xLabels[0], totalData[0])
	return answer, chart, nil
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func fastTrafficHkMacauStay(ctx context.Context, p ExtractedParams) (string, string, error) {
	from := gtime.Now().AddDate(0, 0, -6).Format("Y-m-d")
	to := gtime.Now().Format("Y-m-d")
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().StayDistribution(ctx, from, to, p.RegionName, "1")
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return "近七天暂无港澳车停留数据。", "", nil
	}
	answer := "近七天港澳车停留时长分布："
	for _, item := range items {
		answer += fmt.Sprintf(" %s(%d辆，均%.0f分钟)；", item.Bucket, item.VehicleCount, item.AvgStayMinutes)
	}
	xLabels := make([]string, 0, len(items))
	vehicleData := make([]int, 0, len(items))
	tableRows := make([][]any, 0, len(items))
	for _, item := range items {
		xLabels = append(xLabels, item.Bucket)
		vehicleData = append(vehicleData, item.VehicleCount)
		tableRows = append(tableRows, []any{item.Bucket, item.VehicleCount, fmt.Sprintf("%.1f", item.AvgStayMinutes)})
	}
	chart := buildChart("bar", "港澳车停留时长分布", xLabels,
		[]chartSeries{{Name: "车辆数", Data: vehicleData}},
		[]string{"时段", "车辆数", "平均停留(分钟)"}, tableRows)
	return answer, chart, nil
}
