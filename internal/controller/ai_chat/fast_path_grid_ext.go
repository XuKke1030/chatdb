package ai_chat

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

func fastGridOverview(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	source := gridQuerySource(ctx, db)

	// 1. Total case count + close rate
	total, err := db.Model(source.Table).Ctx(ctx).Count()
	if err != nil {
		return "", "", err
	}
	closed, _ := db.Model(source.Table).Ctx(ctx).Where(source.ClosedWhere).Count()
	closeRate := 0.0
	if total > 0 {
		closeRate = float64(closed) / float64(total) * 100
	}

	// 2. Region rank (top 10)
	regionRecords, _ := db.Ctx(ctx).Raw(fmt.Sprintf(`
SELECT COALESCE(NULLIF(%s,''), '未知') AS name, COUNT(*) AS total
FROM %s
GROUP BY name ORDER BY total DESC LIMIT 10`, source.RegionExpr, source.Table)).All()

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
	typeRecords, _ := db.Ctx(ctx).Raw(fmt.Sprintf(`
SELECT COALESCE(NULLIF(%s,''), '未分类') AS name, COUNT(*) AS total
FROM %s
GROUP BY name ORDER BY total DESC LIMIT 10`, source.CaseTypeExpr, source.Table)).All()

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
	answer := fmt.Sprintf("当前案件总量 %d 件，已结案 %d 件，结案率 %.1f%%。", total, closed, closeRate)
	if topRegion != "" {
		answer += fmt.Sprintf("案件最多区域为「%s」（%d件）。", topRegion, topRegionCount)
	}
	if topType != "" {
		answer += fmt.Sprintf("案件最多类型为「%s」（占比%.1f%%）。", topType, topTypePct)
	}

	// Build multiple charts
	charts := make([]string, 0, 3)

	if len(regionX) > 0 {
		charts = append(charts, buildChart("bar", "区域案件数量排名 Top10", regionX,
			[]chartSeries{{Name: "案件数", Data: regionData}},
			[]string{"排名", "区域", "案件数"}, regionRows))
	}

	if len(typeX) > 0 {
		charts = append(charts, buildChart("pie", "案件类型分布 Top10", typeX,
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
