package ai_chat

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func fastGridOverview(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	source := gridQuerySource(ctx, db)
	if _, ok := gridQuerySourceConfigs[source.Table]; !ok {
		return "数据源配置异常。", "", nil
	}

	dateLabel := "全部日期"
	dateCond := ""
	dateArgs := make([]any, 0, 2)
	if p.AllDates {
		dateLabel = "全部日期"
	} else if p.DateFrom != "" && !p.AllDates {
		dateLabel = fmt.Sprintf("%s至%s", p.DateFrom[:10], p.DateTo[:10])
		dateCond = " AND report_time >= ? AND report_time <= ?"
		dateArgs = append(dateArgs, p.DateFrom, p.DateTo)
	}

	regionCond := ""
	regionArgs := make([]any, 0, 2)
	regionLabel := ""
	if p.RegionName != "" {
		regionCond = fmt.Sprintf(" AND %s LIKE ?", source.RegionExpr)
		regionArgs = append(regionArgs, "%"+p.RegionName+"%")
		regionLabel = p.RegionName
	}

	label := dateLabel
	if regionLabel != "" {
		label += regionLabel
	}

	dimCond := ""
	dimArgs := make([]any, 0, 4)
	if source.HasCommunity && p.Community != "" {
		dimCond += " AND community LIKE ?"
		dimArgs = append(dimArgs, "%"+p.Community+"%")
		label += p.Community
	}
	if source.HasCommunity && p.GridName != "" {
		dimCond += " AND grid_name LIKE ?"
		dimArgs = append(dimArgs, "%"+p.GridName+"%")
	}
	if p.CaseType != "" {
		dimCond += fmt.Sprintf(" AND %s LIKE ?", source.CaseTypeExpr)
		dimArgs = append(dimArgs, "%"+p.CaseType+"%")
		label += p.CaseType
	}

	whereClause := dateCond + regionCond + dimCond
	whereArgs := append(dateArgs, append(regionArgs, dimArgs...)...)

	// 1. Total case count + close rate
	totalQuery := fmt.Sprintf("SELECT COUNT(*) AS cnt FROM %s WHERE 1=1%s", source.Table, whereClause)
	totalRecord, err := db.Ctx(ctx).Raw(totalQuery, whereArgs...).One()
	if err != nil {
		return "", "", err
	}
	total := totalRecord["cnt"].Int()

	closedQuery := fmt.Sprintf("SELECT COUNT(*) AS cnt FROM %s WHERE %s%s", source.Table, source.ClosedWhere, whereClause)
	closedRecord, err := db.Ctx(ctx).Raw(closedQuery, whereArgs...).One()
	if err != nil {
		return "", "", err
	}
	closed := closedRecord["cnt"].Int()
	closeRate := 0.0
	if total > 0 {
		closeRate = float64(closed) / float64(total) * 100
	}

	// 2. Region rank (top 10)
	regionRankQuery := fmt.Sprintf(`
SELECT COALESCE(NULLIF(%s,''), '未知') AS name, COUNT(*) AS total
FROM %s
WHERE 1=1%s
GROUP BY name ORDER BY total DESC LIMIT 10`, source.RegionExpr, source.Table, whereClause)
	regionRecords, _ := db.Ctx(ctx).Raw(regionRankQuery, whereArgs...).All()

	topRegion := ""
	topRegionCount := 0
	regionX := make([]string, 0, len(regionRecords))
	regionData := make([]int, 0, len(regionRecords))
	regionRows := make([][]any, 0, len(regionRecords))
	for i, r := range regionRecords {
		name := r["name"].String()
		cnt := r["total"].Int()
		regionX = append(regionX, name)
		regionData = append(regionData, cnt)
		regionRows = append(regionRows, []any{fmt.Sprintf("%d", i+1), name, cnt})
		if i == 0 {
			topRegion = name
			topRegionCount = cnt
		}
	}

	// 3. Case type distribution (top 10)
	typeQuery := fmt.Sprintf(`
SELECT COALESCE(NULLIF(%s,''), '未分类') AS name, COUNT(*) AS total
FROM %s
WHERE 1=1%s
GROUP BY name ORDER BY total DESC LIMIT 10`, source.CaseTypeExpr, source.Table, whereClause)
	typeRecords, _ := db.Ctx(ctx).Raw(typeQuery, whereArgs...).All()

	topType := ""
	topTypePct := 0.0
	typeAllTotal := 0
	typeX := make([]string, 0, len(typeRecords))
	typeData := make([]int, 0, len(typeRecords))
	typeRows := make([][]any, 0, len(typeRecords))
	for _, r := range typeRecords {
		typeAllTotal += r["total"].Int()
	}
	for i, r := range typeRecords {
		name := r["name"].String()
		cnt := r["total"].Int()
		typeX = append(typeX, name)
		typeData = append(typeData, cnt)
		pct := 0.0
		if typeAllTotal > 0 {
			pct = float64(cnt) / float64(typeAllTotal) * 100
		}
		typeRows = append(typeRows, []any{fmt.Sprintf("%d", i+1), name, cnt, fmt.Sprintf("%.1f%%", pct)})
		if i == 0 {
			topType = name
			topTypePct = pct
		}
	}

	// Build conclusion text
	answer := fmt.Sprintf("%s案件总量 %d 件，已结案 %d 件，结案率 %.1f%%。", label, total, closed, closeRate)
	if topRegion != "" {
		answer += fmt.Sprintf("案件最多区域为「%s」（%d件）。", topRegion, topRegionCount)
	}
	if topType != "" {
		answer += fmt.Sprintf("案件最多类型为「%s」（占比%.1f%%）。", topType, topTypePct)
	}

	// Build multiple charts
	charts := make([]string, 0, 3)

	if len(regionX) > 0 {
		charts = append(charts, buildChart("bar", label+"区域案件数量排名 Top10", regionX,
			[]chartSeries{{Name: "案件数", Data: regionData}},
			[]string{"排名", "区域", "案件数"}, regionRows))
	}

	if len(typeX) > 0 {
		charts = append(charts, buildChart("pie", label+"案件类型分布 Top10", typeX,
			[]chartSeries{{Name: "案件数", Data: typeData}},
			[]string{"排名", "案件类型", "案件数", "占比"}, typeRows))
	}

	// Combine charts into one chartData string
	chartData := ""
	for _, c := range charts {
		chartData += c + "\n\n"
	}

	return answer, chartData, nil
}

func fastGridAvgHandle(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")

	dateFrom := gtime.Now().AddDate(0, -1, 0).Format("Y-m") + "-01"
	dateTo := gtime.Now().AddDate(0, 0, -1).Format("Y-m-d")
	if p.DateFrom != "" && !p.AllDates {
		dateFrom = p.DateFrom[:7]
		if len(p.DateTo) >= 7 {
			dateTo = p.DateTo[:7]
		}
	}

	records, err := db.Ctx(ctx).Raw(
		`SELECT COALESCE(NULLIF(case_type1,''), '未分类') AS name,
		        AVG(avg_handle_hours) AS avg_hours
		 FROM grid_metric_monthly
		 WHERE metric_month >= ? AND metric_month <= ?
		 GROUP BY name ORDER BY avg_hours DESC LIMIT 10`, dateFrom, dateTo).All()
	if err != nil {
		return "", "", err
	}
	if len(records) == 0 {
		return "暂无平均处理时长数据。", "", nil
	}

	xLabels := make([]string, 0, len(records))
	avgData := make([]float64, 0, len(records))
	tableRows := make([][]any, 0, len(records))
	for i, r := range records {
		name := r["name"].String()
		hours := r["avg_hours"].Float64()
		xLabels = append(xLabels, name)
		avgData = append(avgData, hours)
		tableRows = append(tableRows, []any{fmt.Sprintf("%d", i+1), name, fmt.Sprintf("%.1f", hours)})
	}

	answer := fmt.Sprintf("各案件类型平均处理时长最高为「%s」（%.1f小时）。", records[0]["name"].String(), records[0]["avg_hours"].Float64())
	chart := buildChart("bar", "案件类型平均处理时长 Top10", xLabels,
		[]chartSeries{{Name: "平均时长(小时)", Data: avgData}},
		[]string{"排名", "案件类型", "平均时长(小时)"}, tableRows)
	return answer, chart, nil
}
