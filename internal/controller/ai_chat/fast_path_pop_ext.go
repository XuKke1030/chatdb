package ai_chat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/frame/g"
)

// fastPopTagProportionQuery answers proportion/percentage questions (age/gender/origin).
func fastPopTagProportionQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	tag := p.Tag
	if tag == "" {
		tag = "年龄"
	}
	from, to := popTagEffectiveRange(7)

	result, err := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to,
		Tag:      tag,
		Type:     p.PopType,
		Area:     p.Area,
		Labels:   p.Labels,
	})
	if err != nil {
		return "查询标签占比时出错：" + err.Error(), "", nil
	}
	if result == nil || len(result.Items) == 0 {
		return fmt.Sprintf("暂无「%s」维度的人流标签数据。", tag), "", nil
	}

	typeName := "总计"
	if p.PopType == 2 {
		typeName = "进站"
	} else if p.PopType == 3 {
		typeName = "出站"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "「%s」维度占比（%s，%s）：", tag, typeName, p.PeriodLabel(7))
	for i, it := range result.Items {
		if i > 0 {
			sb.WriteString("，")
		}
		fmt.Fprintf(&sb, "%s占%s%%", it.Label, it.Pct)
	}
	top := result.Items[0]
	fmt.Fprintf(&sb, "。占比最高为「%s」（%s%%）。", top.Label, top.Pct)

	xLabels := make([]string, 0, len(result.Items))
	countData := make([]int, 0, len(result.Items))
	pieRows := make([][]any, 0, len(result.Items))
	for _, it := range result.Items {
		xLabels = append(xLabels, it.Label)
		countData = append(countData, it.Count)
		pieRows = append(pieRows, []any{it.Label, it.Count, it.Pct + "%"})
	}
	chart := buildChart("pie", tag+"占比分布（"+typeName+"）", xLabels,
		[]chartSeries{{Name: "人数", Data: countData}},
		[]string{"标签", "人数", "占比"}, pieRows)

	return sb.String(), chart, nil
}

// fastPopMultiTagTrendQuery shows multiple labels' daily trend within a tag.
func fastPopMultiTagTrendQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	tag := p.Tag
	if tag == "" {
		tag = "年龄"
	}
	topN := p.TopN
	if topN < 3 {
		topN = 5
	}
	from, to := popTagEffectiveRange(7)

	// Step 1: get top-N labels
	distResult, err := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to,
		Tag:      tag,
		Type:     p.PopType,
		Area:     p.Area,
	})
	if err != nil {
		return "查询多标签趋势时出错：" + err.Error(), "", nil
	}
	if distResult == nil || len(distResult.Items) == 0 {
		return fmt.Sprintf("暂无「%s」维度的人流标签数据。", tag), "", nil
	}
	topItems := distResult.Items
	if len(topItems) > topN {
		topItems = topItems[:topN]
	}
	labels := make([]string, 0, len(topItems))
	for _, it := range topItems {
		labels = append(labels, it.Label)
	}

	// Step 2: for age tag, expand broad labels to DB labels
	dbLabels := labels
	if tag == "年龄" {
		dbLabels = make([]string, 0)
		for _, l := range labels {
			if dbLs, ok := ageBroadToDBLabels[l]; ok {
				dbLabels = append(dbLabels, dbLs...)
			} else {
				dbLabels = append(dbLabels, l)
			}
		}
	}

	// Step 3: query daily trend per label from population_tag_daily
	labelPlaceholders := make([]string, 0, len(dbLabels))
	labelArgs := make([]any, 0, len(dbLabels))
	for _, l := range dbLabels {
		labelPlaceholders = append(labelPlaceholders, "?")
		labelArgs = append(labelArgs, l)
	}
	typeName := "总计"
	typeVal := 1
	if p.PopType == 2 {
		typeName = "进站"
		typeVal = 2
	} else if p.PopType == 3 {
		typeName = "出站"
		typeVal = 3
	}

	sql := fmt.Sprintf(
		"SELECT day, label, SUM(label_cnt) AS cnt FROM population_tag_daily WHERE day >= DATE(?) AND day <= DATE(?) AND tag = ? AND type = ? AND label IN (%s) GROUP BY day, label ORDER BY day",
		strings.Join(labelPlaceholders, ","),
	)
	args := append([]any{from, to, tag, typeVal}, labelArgs...)
	records, err := g.DB("master").Ctx(ctx).Raw(sql, args...).All()
	if err != nil {
		return "查询多标签趋势时出错：" + err.Error(), "", nil
	}
	if records == nil || len(records) == 0 {
		return fmt.Sprintf("暂无「%s」维度%s的趋势数据。", tag, p.PeriodLabel(7)), "", nil
	}

	// Step 4: merge age fine-grained labels into broad groups
	type dayLabel struct {
		Day   string
		Label string
		Count int
	}
	items := make([]dayLabel, 0, len(records))
	for _, r := range records {
		displayLabel := r["label"].String()
		if tag == "年龄" {
			if broad, ok := ageLabelGroup[displayLabel]; ok {
				displayLabel = broad
			}
		}
		items = append(items, dayLabel{Day: r["day"].String(), Label: displayLabel, Count: r["cnt"].Int()})
	}

	// Step 5: build chart data
	dateSet := make(map[string]bool)
	labelSet := make(map[string]bool)
	for _, it := range items {
		dateSet[it.Day] = true
		labelSet[it.Label] = true
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	chartLabels := make([]string, 0, len(labelSet))
	for l := range labelSet {
		chartLabels = append(chartLabels, l)
	}
	sort.Strings(chartLabels)

	// map: label -> date -> count
	countMap := make(map[string]map[string]int)
	for _, it := range items {
		if countMap[it.Label] == nil {
			countMap[it.Label] = make(map[string]int)
		}
		countMap[it.Label][it.Day] += it.Count
	}

	series := make([]chartSeries, 0, len(chartLabels))
	tableRows := make([][]any, 0, len(dates))
	for _, l := range chartLabels {
		data := make([]int, 0, len(dates))
		for _, d := range dates {
			data = append(data, countMap[l][d])
		}
		series = append(series, chartSeries{Name: l, Data: data})
	}
	for _, d := range dates {
		row := []any{d}
		for _, l := range chartLabels {
			row = append(row, countMap[l][d])
		}
		tableRows = append(tableRows, row)
	}
	cols := []string{"日期"}
	cols = append(cols, chartLabels...)

	chart := buildChart("line", tag+"多维趋势（"+typeName+"）", dates, series, cols, tableRows)

	topLabel := chartLabels[0]
	trendDir := "平稳"
	if len(dates) >= 2 {
		last := countMap[topLabel][dates[len(dates)-1]]
		prev := countMap[topLabel][dates[0]]
		if last > prev {
			trendDir = "上升"
		} else if last < prev {
			trendDir = "下降"
		}
	}
	answer := fmt.Sprintf("%s「%s」维度中，「%s」趋势%s。", p.PeriodLabel(7), tag, topLabel, trendDir)
	return answer, chart, nil
}

// fastPopComprehensiveQuery produces a multi-dimensional composite analysis.
func fastPopComprehensiveQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := popEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}
	toFull := to + " 23:59:59"

	charts := make([]string, 0, 4)

	// 1. 7-day in/out trend
	result, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   toFull,
		GroupBy:  "day",
	})
	if err != nil || result == nil || len(result.Series) == 0 {
		return p.PeriodLabel(7) + "暂无人流数据。", "", nil
	}

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

	s := result.Summary

	// 2. Region rank
	regionResult, regionErr := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   toFull,
		GroupBy:  "region",
	})
	topRegion := ""
	topRegionCount := 0
	if regionErr == nil && regionResult != nil && len(regionResult.Series) > 0 {
		type ri struct {
			Name    string
			InCount int
		}
		regions := make([]ri, 0, len(regionResult.Series))
		for _, item := range regionResult.Series {
			regions = append(regions, ri{Name: item.Name, InCount: item.InCount})
		}
		sort.Slice(regions, func(i, j int) bool { return regions[i].InCount > regions[j].InCount })
		top5 := regions
		if len(top5) > 5 {
			top5 = top5[:5]
		}
		regionX := make([]string, 0, len(top5))
		regionData := make([]int, 0, len(top5))
		regionRows := make([][]any, 0, len(top5))
		for i, r := range top5 {
			regionX = append(regionX, r.Name)
			regionData = append(regionData, r.InCount)
			regionRows = append(regionRows, []any{fmt.Sprintf("%d", i+1), r.Name, r.InCount})
		}
		if len(regionX) > 0 {
			charts = append(charts, buildChart("bar", "区域人流排名 Top5", regionX,
				[]chartSeries{{Name: "进入人次", Data: regionData}},
				[]string{"排名", "区域", "进入人次"}, regionRows))
		}
		topRegion = top5[0].Name
		topRegionCount = top5[0].InCount
	}

	// 3. Age distribution pie
	ageResult, ageErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from[:10],
		DateTo:   to[:10],
		Tag:      "年龄",
	})
	ageTopLabel := ""
	ageTopPct := ""
	if ageErr == nil && ageResult != nil && len(ageResult.Items) > 0 {
		ageX := make([]string, 0, len(ageResult.Items))
		ageData := make([]int, 0, len(ageResult.Items))
		ageRows := make([][]any, 0, len(ageResult.Items))
		for _, it := range ageResult.Items {
			ageX = append(ageX, it.Label)
			ageData = append(ageData, it.Count)
			ageRows = append(ageRows, []any{it.Label, it.Count, it.Pct + "%"})
		}
		if len(ageX) > 0 {
			charts = append(charts, buildChart("pie", "年龄占比分布", ageX,
				[]chartSeries{{Name: "人数", Data: ageData}},
				[]string{"年龄段", "人数", "占比"}, ageRows))
		}
		ageTopLabel = ageResult.Items[0].Label
		ageTopPct = ageResult.Items[0].Pct
	}

	// 4. Origin bar_rank
	originResult, originErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from[:10],
		DateTo:   to[:10],
		Tag:      "省外来源",
	})
	originTopLabel := ""
	originTopPct := ""
	if originErr == nil && originResult != nil && len(originResult.Items) > 0 {
		originX := make([]string, 0, len(originResult.Items))
		originData := make([]int, 0, len(originResult.Items))
		originRows := make([][]any, 0, len(originResult.Items))
		for i, it := range originResult.Items {
			originX = append(originX, it.Label)
			originData = append(originData, it.Count)
			originRows = append(originRows, []any{fmt.Sprintf("%d", i+1), it.Label, it.Count, it.Pct + "%"})
		}
		if len(originX) > 0 {
			charts = append(charts, buildChart("bar", "省外来源排名 Top5", originX[:minInt(5, len(originX))],
				[]chartSeries{{Name: "人数", Data: originData[:min(5, len(originData))]}},
				[]string{"排名", "来源地", "人数", "占比"}, originRows[:min(5, len(originRows))]))
		}
		originTopLabel = originResult.Items[0].Label
		originTopPct = originResult.Items[0].Pct
	}

	// Build answer
	answer := fmt.Sprintf("%s人流总计进入 %d 人次，离开 %d 人次，净流入 %d 人次。",
			p.PeriodLabel(7), s.TotalInCount, s.TotalOutCount, s.TotalNetInCount)
	if topRegion != "" {
		answer += fmt.Sprintf("区域最多为「%s」（%d人次）。", topRegion, topRegionCount)
	}
	if ageTopLabel != "" {
		answer += fmt.Sprintf("年龄占比最高为「%s」(%s%%)。", ageTopLabel, ageTopPct)
	}
	if originTopLabel != "" {
		answer += fmt.Sprintf("省外来源第1为「%s」(%s%%)。", originTopLabel, originTopPct)
	}

	chartData := ""
	for _, c := range charts {
		chartData += c + "\n\n"
	}
	return answer, chartData, nil
}

// fastPopMultiTagCompareQuery compares multiple tag dimensions side-by-side.
func fastPopMultiTagCompareQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := popTagEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom[:10]
		to = p.DateTo[:10]
	}

	charts := make([]string, 0, 3)
	var sb strings.Builder

	// 1. Age pie
	ageResult, ageErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to,
		Tag:      "年龄",
	})
	ageTopLabel := ""
	ageTopPct := ""
	if ageErr == nil && ageResult != nil && len(ageResult.Items) > 0 {
		ageX := make([]string, 0, len(ageResult.Items))
		ageData := make([]int, 0, len(ageResult.Items))
		ageRows := make([][]any, 0, len(ageResult.Items))
		for _, it := range ageResult.Items {
			ageX = append(ageX, it.Label)
			ageData = append(ageData, it.Count)
			ageRows = append(ageRows, []any{it.Label, it.Count, it.Pct + "%"})
		}
		charts = append(charts, buildChart("pie", "年龄占比分布", ageX,
			[]chartSeries{{Name: "人数", Data: ageData}},
			[]string{"年龄段", "人数", "占比"}, ageRows))
		ageTopLabel = ageResult.Items[0].Label
		ageTopPct = ageResult.Items[0].Pct
	}

	// 2. Gender pie
	genderResult, genderErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to,
		Tag:      "性别",
	})
	malePct := "0"
	femalePct := "0"
	if genderErr == nil && genderResult != nil && len(genderResult.Items) > 0 {
		genderX := make([]string, 0, len(genderResult.Items))
		genderData := make([]int, 0, len(genderResult.Items))
		genderRows := make([][]any, 0, len(genderResult.Items))
		for _, it := range genderResult.Items {
			genderX = append(genderX, it.Label)
			genderData = append(genderData, it.Count)
			genderRows = append(genderRows, []any{it.Label, it.Count, it.Pct + "%"})
		}
		charts = append(charts, buildChart("pie", "性别占比分布", genderX,
			[]chartSeries{{Name: "人数", Data: genderData}},
			[]string{"性别", "人数", "占比"}, genderRows))
		for _, it := range genderResult.Items {
			if strings.Contains(it.Label, "男") {
				malePct = it.Pct
			} else if strings.Contains(it.Label, "女") {
				femalePct = it.Pct
			}
		}
	}

	// 3. Origin bar_rank
	originResult, originErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to,
		Tag:      "省外来源",
	})
	originTopLabel := ""
	originTopPct := ""
	if originErr == nil && originResult != nil && len(originResult.Items) > 0 {
		originX := make([]string, 0, len(originResult.Items))
		originData := make([]int, 0, len(originResult.Items))
		originRows := make([][]any, 0, len(originResult.Items))
		for i, it := range originResult.Items {
			originX = append(originX, it.Label)
			originData = append(originData, it.Count)
			originRows = append(originRows, []any{fmt.Sprintf("%d", i+1), it.Label, it.Count, it.Pct + "%"})
		}
		topN := minInt(5, len(originX))
		charts = append(charts, buildChart("bar", "省外来源排名 Top5", originX[:topN],
			[]chartSeries{{Name: "人数", Data: originData[:topN]}},
			[]string{"排名", "来源地", "人数", "占比"}, originRows[:topN]))
		originTopLabel = originResult.Items[0].Label
		originTopPct = originResult.Items[0].Pct
	}

	// Build answer
	fmt.Fprintf(&sb, "多维标签对比：")
	if ageTopLabel != "" {
		fmt.Fprintf(&sb, "年龄占比最高「%s」(%s%%)；", ageTopLabel, ageTopPct)
	}
	if malePct != "0" || femalePct != "0" {
		fmt.Fprintf(&sb, "性别：男%s%%/女%s%%；", malePct, femalePct)
	}
	if originTopLabel != "" {
		fmt.Fprintf(&sb, "省外来源第1「%s」(%s%%)；", originTopLabel, originTopPct)
	}
	answer := strings.TrimRight(sb.String(), "；") + "。"

	chartData := ""
	for _, c := range charts {
		chartData += c + "\n\n"
	}
	return answer, chartData, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
