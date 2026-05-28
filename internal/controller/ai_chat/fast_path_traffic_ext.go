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

func trafficDateRange(p ExtractedParams, defaultDays int) (from, to string) {
	if defaultDays <= 0 {
		defaultDays = 7
	}
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
		return
	}
	from = gtime.Now().AddDate(0, 0, -(defaultDays-1)).Format("Y-m-d")
	to = gtime.Now().Format("Y-m-d") + " 23:59:59"
	return
}

func fastTrafficProvinceInside(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)
	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom:    from,
		DateTo:      to,
		GroupBy:     "day",
		GateName:    p.GateName,
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
		answer := fmt.Sprintf("%s车流中，省内车 %d 辆（占比%.1f%%），省外车 %d 辆（占比%.1f%%）。",
			p.PeriodLabel(7), s.ProvinceInsideCount, insideRatio, s.ProvinceOutsideCount, outsideRatio)
	return answer, "", nil
}

func fastTrafficYoY(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)
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
	answer := fmt.Sprintf("%s车流同比%s %.1f%%。", p.PeriodLabel(7), dir, first.ChangePct)
	if first.ChangePct >= 0 && first.ChangePct < 1 {
		answer = p.PeriodLabel(7) + "车流与去年同期基本持平。"
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
		[]chartSeries{{Name: "本期", Data: curData}, {Name: "上期", Data: priorData}},
		[]string{"日期", "本期", "上期", "变化率"}, tableRows)
	first := items[0]
	dir := "增长"
	if first.ChangePct < 0 {
		dir = "下降"
	}
	answer := fmt.Sprintf("%s车流环比%s %.1f%%。", p.PeriodLabel(30), dir, first.ChangePct)
	if first.ChangePct >= 0 && first.ChangePct < 1 {
		answer = p.PeriodLabel(30) + "车流与上期基本持平。"
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
	from, to := trafficDateRange(p, 7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().StayDistribution(ctx, from, to, p.RegionName, "")
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return p.PeriodLabel(7) + "暂无停留时长分布数据。", "", nil
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
	answer := fmt.Sprintf("%s共有 %d 辆车有停留记录，主要集中在%s时段。", p.PeriodLabel(7), totalVehicles, topBucket)
	return answer, chart, nil
}

func fastTrafficOriginByProvince(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().OriginRank(ctx, from, to, 10)
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return p.PeriodLabel(7) + "暂无外省来源数据。", "", nil
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
	answer := fmt.Sprintf("%s省外车主要来自「%s」，共 %d 辆。", p.PeriodLabel(7), topLabel, top.VehicleCount)
	return answer, chart, nil
}

// ---- Composite overview fast paths ----

func fastTrafficOverview(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)
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
	days := p.Days
	if days <= 0 {
		days = 7
	}
	var periodLabel string
	if days == 1 {
		periodLabel = from
	} else {
		periodLabel = fmt.Sprintf("近%d天", days)
	}
	answer := fmt.Sprintf("%s车流总计 %d 辆，日均 %.0f 辆。港澳车 %d 辆（占比%.1f%%），省内车 %d 辆（占大陆车%.1f%%）。",
		periodLabel, s.Total, float64(s.Total)/float64(days), s.HkMacauCount, hkRatio, s.ProvinceInsideCount, insideRatio)
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
		charts = append(charts, buildChart("line", p.PeriodLabel(7)+"车流趋势", xLabels,
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
	from, to := popEffectiveRange(7)
	toFull := to + " 23:59:59"
	days := p.Days
	if days <= 0 {
		days = 7
	}

	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		toFull = p.DateTo
	}

	var periodLabel string
	if days == 1 {
		periodLabel = from
	} else {
		periodLabel = fmt.Sprintf("近%d天", days)
	}

	result, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   toFull,
		GroupBy:  "day",
	})
	if err != nil {
		return "", "", err
	}
	if result == nil || len(result.Series) == 0 {
		return fmt.Sprintf("%s暂无人流数据。", periodLabel), "", nil
	}

	s := result.Summary
	answer := fmt.Sprintf("%s人流总计进入 %d 人次，离开 %d 人次，净流入 %d 人次。流动人口 %d 人。",
		periodLabel, s.TotalInCount, s.TotalOutCount, s.TotalNetInCount, s.TotalFloatingPopulation)

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
		charts = append(charts, buildChart("line", p.PeriodLabel(7)+"人流趋势", xLabels,
			[]chartSeries{{Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
			[]string{"日期", "进入", "离开", "净流入", "流动人口"}, trendRows))
	}

	// Chart 2: Region rank bar
	regionResult, regionErr := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   toFull,
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

func fastPopYoYQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := popEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}

	items, err := service.Population().YoYCompare(ctx, from, to, "day")
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

	parsedTo, _ := time.Parse("2006-01-02", to)
	priorFrom := parsedTo.AddDate(-1, 0, -6).Format("2006-01-02")
	priorTo := parsedTo.AddDate(-1, 0, 0).Format("Y-m-d")
	priorItems, _ := service.Population().YoYCompare(ctx, priorFrom, priorTo, "day")

	var lastTotal int
	if len(priorItems) > 0 {
		for _, it := range priorItems {
			lastTotal += it.PriorVal
		}
	} else {
		return fmt.Sprintf("%s人流总计 %d 人次，但去年同期无数据，无法计算同比。", p.PeriodLabel(7), thisTotal), "", nil
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
	answer := fmt.Sprintf("%s人流同比%s %.1f%%（今年 %d 人次，去年同期 %d 人次）。", p.PeriodLabel(7), direction, absF(rate), thisTotal, lastTotal)
	return answer, chart, nil
}

func fastPopMultiRegionCompareQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := popEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	toFull := to + " 23:59:59"

	result, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   toFull,
		GroupBy:  "region",
	})
	if err != nil {
		return p.PeriodLabel(7) + "暂无分区域人流数据。", "", nil
	}
	if result == nil || len(result.Series) == 0 {
		return p.PeriodLabel(7) + "暂无分区域人流数据。", "", nil
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

	chart := buildChart("line", p.PeriodLabel(7)+"各区域人流趋势对比", dates, series,
		[]string{"日期", "区域", "人流总量"}, nil)

	answer := fmt.Sprintf("%s人流最多的区域为「%s」，共 %d 人次。", p.PeriodLabel(7), sorted[0].Region, sorted[0].Total)
	return answer, chart, nil
}

func fastPopFloatingAnomalyQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	count, err := db.Model("population_floating_daily").Ctx(ctx).Count()
	if err != nil || count == 0 {
		return fastPopFloatingFromMetric(ctx, p)
	}

	from, to := popEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}

	records, err := db.Ctx(ctx).Raw(
		"SELECT region, SUM(floating_population_count) AS total FROM population_floating_daily WHERE metric_date >= ? AND metric_date <= ? GROUP BY region ORDER BY total DESC LIMIT 10",
		from, to).All()
	if err != nil || len(records) == 0 {
		return "暂无流动人口数据。", "", nil
	}

	prevFrom := gtime.NewFromStr(to).AddDate(0, 0, -13).Format("Y-m-d")
	prevTo := gtime.NewFromStr(to).AddDate(0, 0, -7).Format("Y-m-d")

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

	chart := buildChart("bar", p.PeriodLabel(7)+"流动人口排名", xLabels,
		[]chartSeries{{Name: "流动人口", Data: totalData}},
		[]string{"区域", "本期流动人口", "上期流动人口", "增长率"}, tableRows)

	answer := fmt.Sprintf("%s流动人口最多的区域为「%s」，共 %d 人。", p.PeriodLabel(7), xLabels[0], totalData[0])
	if len(anomalyRegions) > 0 {
		answer += "异常增长区域：" + strings.Join(anomalyRegions, "、") + "。"
	} else {
		answer += "各区域流动人口增长均在 20% 以内，未发现异常增长。"
	}
	return answer, chart, nil
}

func fastPopFloatingFromMetric(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	from, to := popEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}

	records, err := db.Ctx(ctx).Raw(
		"SELECT region, SUM(floating_population_count) AS total FROM population_metric_daily WHERE metric_date >= ? AND metric_date <= ? GROUP BY region ORDER BY total DESC LIMIT 10",
		from, to).All()
	if err != nil || len(records) == 0 {
		return "暂无流动人口数据。", "", nil
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

	chart := buildChart("bar", p.PeriodLabel(7)+"流动人口排名", xLabels,
		[]chartSeries{{Name: "流动人口", Data: totalData}},
		[]string{"区域", "流动人口"}, tableRows)

	answer := fmt.Sprintf("%s流动人口最多的区域为「%s」，共 %d 人。", p.PeriodLabel(7), xLabels[0], totalData[0])
	return answer, chart, nil
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func fastTrafficHkMacauStay(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	items, err := service.Traffic().StayDistribution(ctx, from, to, p.RegionName, "1")
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		return p.PeriodLabel(7) + "暂无港澳车停留数据。", "", nil
	}
	answer := p.PeriodLabel(7) + "港澳车停留时长分布："
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

func fastPopTagDistributionQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	tag := p.Tag
	if tag == "" {
		tag = "年龄"
	}

	dateFrom, dateTo := p.DateFrom, p.DateTo
	if dateFrom == "" {
		dateFrom, dateTo = popTagEffectiveRange(7)
	}

	result, err := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		AllDates: p.AllDates,
		Tag:      tag,
		Type:     p.PopType,
		Area:     p.Area,
		Labels:   p.Labels,
	})
	if err != nil {
		return "查询人流标签分布时出错：" + err.Error(), "", nil
	}
	if result == nil || len(result.Items) == 0 {
		return "暂无「" + tag + "」维度的人流标签数据。", "", nil
	}

	// Build chart data
	kind := "pie"
	xLabels := make([]string, 0, len(result.Items))
	countData := make([]int, 0, len(result.Items))
	tableRows := make([][]any, 0, len(result.Items))
	for _, item := range result.Items {
		xLabels = append(xLabels, item.Label)
		countData = append(countData, item.Count)
		tableRows = append(tableRows, []any{item.Label, item.Count, item.Pct + "%"})
	}

	typeName := "总人数"
	switch result.Type {
	case 2:
		typeName = "进站人数"
	case 3:
		typeName = "出站人数"
	}

	chart := buildChart(kind, tag+"分布（"+typeName+"）", xLabels,
		[]chartSeries{{Name: "人数", Data: countData}},
		[]string{tag, "人数", "占比"}, tableRows)

	// Build answer text
	topItem := result.Items[0]
	answer := fmt.Sprintf("「%s」维度%s分布：最多的是「%s」，共%d人，占比%s%%。共%d人。",
		tag, typeName, topItem.Label, topItem.Count, topItem.Pct, result.Total)

	return answer, chart, nil
}

func fastPopTagTopNQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	tag := p.Tag
	if tag == "" {
		tag = "省外来源"
	}
	topN := p.TopN
	if topN <= 0 {
		topN = 5
	}

	dateFrom, dateTo := p.DateFrom, p.DateTo
	if dateFrom == "" {
		dateFrom, dateTo = popTagEffectiveRange(7)
	}

	result, err := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		AllDates: p.AllDates,
		Tag:      tag,
		Type:     p.PopType,
		Area:     p.Area,
		Labels:   p.Labels,
	})
	if err != nil {
		return "查询人流标签排名时出错：" + err.Error(), "", nil
	}
	if result == nil || len(result.Items) == 0 {
		return "暂无「" + tag + "」维度的人流标签数据。", "", nil
	}

	items := result.Items
	if len(items) > topN {
		items = items[:topN]
	}

	typeName := "总人数"
	switch result.Type {
	case 2:
		typeName = "进站人数"
	case 3:
		typeName = "出站人数"
	}

	xLabels := make([]string, 0, len(items))
	countData := make([]int, 0, len(items))
	tableRows := make([][]any, 0, len(items))
	for i, item := range items {
		xLabels = append(xLabels, item.Label)
		countData = append(countData, item.Count)
		tableRows = append(tableRows, []any{fmt.Sprintf("%d", i+1), item.Label, item.Count, item.Pct + "%"})
	}

	chart := buildChart("bar", tag+"排名Top"+fmt.Sprintf("%d", topN)+"（"+typeName+"）", xLabels,
		[]chartSeries{{Name: "人数", Data: countData}},
		[]string{"排名", tag, "人数", "占比"}, tableRows)

	topItem := items[0]
	answer := fmt.Sprintf("「%s」维度%s排名前%d：第1名为「%s」，共%d人，占比%s%%。",
		tag, typeName, topN, topItem.Label, topItem.Count, topItem.Pct)
	if len(items) > 1 {
		answer += fmt.Sprintf(" 第2名「%s」（%s%%）。", items[1].Label, items[1].Pct)
	}

	return answer, chart, nil
}

// fastTrafficInOutRatio answers in/out direction proportion questions.
func fastTrafficInOutRatio(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)

	result, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "indir",
	})
	if err != nil {
		return "", "", err
	}
	s := result.Summary

	total := s.InCount + s.OutCount + s.UnknownDirCount
	inPct := 0.0
	outPct := 0.0
	if total > 0 {
		inPct = float64(s.InCount) / float64(total) * 100
		outPct = float64(s.OutCount) / float64(total) * 100
	}

	answer := fmt.Sprintf("%s车流中，进入 %d 辆（占比%.1f%%），离开 %d 辆（占比%.1f%%），方向不明 %d 辆。",
			p.PeriodLabel(7), s.InCount, inPct, s.OutCount, outPct, s.UnknownDirCount)

	pieX := []string{"进入", "离开"}
	if s.UnknownDirCount > 0 {
		pieX = append(pieX, "方向不明")
	}
	pieData := []int{s.InCount, s.OutCount}
	pieRows := [][]any{
		{"进入", s.InCount, fmt.Sprintf("%.1f%%", inPct)},
		{"离开", s.OutCount, fmt.Sprintf("%.1f%%", outPct)},
	}
	if s.UnknownDirCount > 0 {
		pieData = append(pieData, s.UnknownDirCount)
		pieRows = append(pieRows, []any{"方向不明", s.UnknownDirCount, fmt.Sprintf("%.1f%%", 100-inPct-outPct)})
	}
	chart := buildChart("pie", "进出方向占比", pieX,
		[]chartSeries{{Name: "车流量", Data: pieData}},
		[]string{"方向", "车流量", "占比"}, pieRows)

	return answer, chart, nil
}

// fastTrafficMultiGateCompare compares daily trends of top gates.
func fastTrafficMultiGateCompare(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := trafficDateRange(p, 7)

	// Step 1: Get top 5 gates
	gateResult, err := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
		DateFrom: from,
		DateTo:   to,
		GroupBy:  "gate",
	})
	if err != nil {
		return "", "", err
	}
	if gateResult == nil || len(gateResult.Series) == 0 {
		return p.PeriodLabel(7) + "暂无卡口数据。", "", nil
	}

	type gateInfo struct {
		Name  string
		Total int
	}
	gates := make([]gateInfo, 0, len(gateResult.Series))
	for _, item := range gateResult.Series {
		gates = append(gates, gateInfo{Name: item.Name, Total: item.Total})
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].Total > gates[j].Total })
	if len(gates) > 5 {
		gates = gates[:5]
	}

	// Step 2: Get daily trend for each top gate
	dayMap := make(map[string]map[string]int) // day -> gate -> count
	var allDays []string
	for _, g := range gates {
		trendResult, trendErr := service.Traffic().Aggregate(ctx, model.TrafficAggregateQuery{
			DateFrom: from,
			DateTo:   to,
			GroupBy:  "day",
			GateName: g.Name,
		})
		if trendErr != nil || trendResult == nil {
			continue
		}
		for _, item := range trendResult.Series {
			if dayMap[item.Name] == nil {
				dayMap[item.Name] = make(map[string]int)
			}
			dayMap[item.Name][g.Name] = item.Total
			allDays = append(allDays, item.Name)
		}
	}

	if len(allDays) == 0 {
		return p.PeriodLabel(7) + "暂无卡口趋势数据。", "", nil
	}

	// Deduplicate and sort days
	daySet := make(map[string]bool)
	uniqueDays := make([]string, 0)
	for _, d := range allDays {
		if !daySet[d] {
			daySet[d] = true
			uniqueDays = append(uniqueDays, d)
		}
	}
	sort.Strings(uniqueDays)

	// Build chart
	series := make([]chartSeries, 0, len(gates))
	for _, g := range gates {
		data := make([]int, 0, len(uniqueDays))
		for _, d := range uniqueDays {
			data = append(data, dayMap[d][g.Name])
		}
		series = append(series, chartSeries{Name: g.Name, Data: data})
	}

	cols := []string{"日期"}
	for _, g := range gates {
		cols = append(cols, g.Name)
	}
	tableRows := make([][]any, 0, len(uniqueDays))
	for _, d := range uniqueDays {
		row := []any{d}
		for _, g := range gates {
			row = append(row, dayMap[d][g.Name])
		}
		tableRows = append(tableRows, row)
	}

	chart := buildChart("line", "Top5卡口"+p.PeriodLabel(7)+"趋势对比", uniqueDays, series, cols, tableRows)

	topGate := gates[0]
	answer := fmt.Sprintf("%s车流量最大的卡口为「%s」（%d辆）。各卡口趋势对比如下。", p.PeriodLabel(7), topGate.Name, topGate.Total)
	return answer, chart, nil
}

// fastTrafficHkMacauYoYQuery computes year-over-year comparison for HK/Macau vehicles.
func fastTrafficHkMacauYoYQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	thisFrom, thisToFull := trafficDateRange(p, 7)
	thisTo := thisToFull
	if len(thisTo) > 10 {
		thisTo = thisTo[:10]
	}

	db := g.DB("master")

	// Current period
	curRecords, curErr := db.Ctx(ctx).Raw(
		"SELECT DATE(snapshot_time) AS d, COUNT(*) AS total FROM traffic_gate_record WHERE snapshot_time >= DATE(?) AND snapshot_time <= DATE(?) AND is_hk_macau = 1 GROUP BY d ORDER BY d",
		thisFrom, thisTo,
	).All()
	if curErr != nil {
		return "", "", curErr
	}

	parsedFrom, _ := gtime.StrToTime(thisFrom)
	parsedTo, _ := gtime.StrToTime(thisTo)
	priorFrom := parsedFrom.AddDate(-1, 0, 0).Format("Y-m-d")
	priorTo := parsedTo.AddDate(-1, 0, 0).Format("Y-m-d")

	priorRecords, priorErr := db.Ctx(ctx).Raw(
		"SELECT DATE(snapshot_time) AS d, COUNT(*) AS total FROM traffic_gate_record WHERE snapshot_time >= DATE(?) AND snapshot_time <= DATE(?) AND is_hk_macau = 1 GROUP BY d ORDER BY d",
		priorFrom, priorTo,
	).All()

	thisTotal := 0
	curMap := make(map[string]int)
	if curRecords != nil {
		for _, r := range curRecords {
			cnt := r["total"].Int()
			curMap[r["d"].String()] = cnt
			thisTotal += cnt
		}
	}

	priorTotal := 0
	priorMap := make(map[string]int)
	if priorErr == nil && priorRecords != nil {
		for _, r := range priorRecords {
			priorMap[r["d"].String()] = r["total"].Int()
			priorTotal += r["total"].Int()
		}
	}

	if thisTotal == 0 {
		return p.PeriodLabel(7) + "暂无港澳车数据，无法计算同比。", "", nil
	}

	var rate float64
	dir := "持平"
	if priorTotal > 0 {
		rate = float64(thisTotal-priorTotal) / float64(priorTotal) * 100
		if rate > 0.5 {
			dir = "增长"
		} else if rate < -0.5 {
			dir = "下降"
		}
	} else {
		dir = "增长"
		rate = 100.0
	}

	// Build bar_compare chart
	allDays := make([]string, 0, len(curMap))
	for d := range curMap {
		allDays = append(allDays, d)
	}
	sort.Strings(allDays)

	xLabels := make([]string, 0, len(allDays))
	curData := make([]int, 0, len(allDays))
	priorData := make([]int, 0, len(allDays))
	for _, d := range allDays {
		xLabels = append(xLabels, d)
		curData = append(curData, curMap[d])
		priorData = append(priorData, priorMap[d])
	}
	chart := buildChart("bar_compare", "港澳车同比（今年 vs 去年同期）", xLabels,
		[]chartSeries{{Name: "今年", Data: curData}, {Name: "去年同期", Data: priorData}},
		[]string{"日期", "今年港澳车", "去年同期"}, nil)

	answer := fmt.Sprintf("%s港澳车同比%s %.1f%%（今年 %d 辆，去年同期 %d 辆）。", p.PeriodLabel(7), dir, absF(rate), thisTotal, priorTotal)
	return answer, chart, nil
}
