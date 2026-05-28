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

// ageLabelGroup maps fine-grained DB age labels to broad display groups.
var ageLabelGroup = map[string]string{
	"(0,18]":  "0-18",
	"(19,22]": "18-30",
	"(23,25]": "18-30",
	"(26,30]": "18-30",
	"(31,35]": "30-50",
	"(36,40]": "30-50",
	"(41,45]": "30-50",
	"(46,50]": "30-50",
	"(51,55]": "50-70",
	"(56,60]": "50-70",
	">60":     "70+",
}

// ageBroadLabels is the ordered list of broad age group display names.
var ageBroadLabels = []string{"0-18", "18-30", "30-50", "50-70", "70+"}

// ageBroadToDBLabels maps broad group names back to fine-grained DB labels.
var ageBroadToDBLabels = map[string][]string{
	"0-18":  {"(0,18]"},
	"18-30": {"(19,22]", "(23,25]", "(26,30]"},
	"30-50": {"(31,35]", "(36,40]", "(41,45]", "(46,50]"},
	"50-70": {"(51,55]", "(56,60]"},
	"70+":   {">60"},
}

func fastPopTagTrendQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	tag := p.Tag
	if tag == "" {
		tag = "年龄"
	}
	topN := p.TopN
	if topN <= 0 {
		topN = 5
	}

	// If no specific labels given, find top N labels for this tag first
	labels := p.Labels
	if len(labels) == 0 {
		distResult, err := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
			DateFrom: p.DateFrom,
			DateTo:   p.DateTo,
			Tag:      tag,
			Type:     p.PopType,
			Area:     p.Area,
		})
		if err != nil {
			return "查询人流标签趋势时出错：" + err.Error(), "", nil
		}
		if distResult == nil || len(distResult.Items) == 0 {
			return "暂无「" + tag + "」维度的人流标签数据。", "", nil
		}
		topItems := distResult.Items
		if len(topItems) > topN {
			topItems = topItems[:topN]
		}
		for _, it := range topItems {
			labels = append(labels, it.Label)
		}
	}

	// For age tag, expand broad group labels back to fine-grained DB labels for querying
	dbLabels := labels
	if tag == "年龄" {
		expanded := make([]string, 0)
		for _, l := range labels {
			if dbList, ok := ageBroadToDBLabels[l]; ok {
				expanded = append(expanded, dbList...)
			} else {
				expanded = append(expanded, l)
			}
		}
		dbLabels = expanded
	}

	// Query trend by day for the selected labels
	db := g.DB("master")
	from, to := popTagEffectiveRange(7)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom
		to = p.DateTo
	}

	// Build label placeholders
	labelPh := ""
	labelArgs := make([]any, 0, len(dbLabels))
	for i, l := range dbLabels {
		if i > 0 {
			labelPh += ","
		}
		labelPh += "?"
		labelArgs = append(labelArgs, l)
	}

	args := []any{from, to, tag}
	args = append(args, labelArgs...)
	if p.PopType > 0 {
		args = append(args, p.PopType)
	}
	if p.Area != "" {
		args = append(args, p.Area)
	}

	typeCondition := ""
	if p.PopType > 0 {
		typeCondition = " AND type = ?"
	}
	areaCondition := ""
	if p.Area != "" {
		areaCondition = " AND area = ?"
	}

	sqlStr := "SELECT day, label, SUM(label_cnt) AS cnt FROM population_tag_daily WHERE day >= DATE(?) AND day <= DATE(?) AND tag = ? AND label IN (" + labelPh + ")" + typeCondition + areaCondition + " GROUP BY day, label ORDER BY day, label"

	records, err := db.Ctx(ctx).Raw(sqlStr, args...).All()
	if err != nil {
		return "查询人流标签趋势时出错：" + err.Error(), "", nil
	}
	if len(records) == 0 {
		return p.PeriodLabel(7) + "暂无「" + tag + "」维度的趋势数据。", "", nil
	}

	typeName := "总人数"
	switch p.PopType {
	case 2:
		typeName = "进站人数"
	case 3:
		typeName = "出站人数"
	}

	// For age tag, merge fine-grained DB labels into broad groups
	if tag == "年龄" {
		type trendRow struct {
			day   string
			label string
			cnt   int
		}
		rows := make([]trendRow, 0, len(records))
		for _, r := range records {
			dbLabel := r["label"].String()
			group, ok := ageLabelGroup[dbLabel]
			if !ok {
				group = dbLabel
			}
			rows = append(rows, trendRow{day: r["day"].String(), label: group, cnt: r["cnt"].Int()})
		}
		// Re-aggregate by day+group
		merged := make(map[string]map[string]int)
		for _, r := range rows {
			if merged[r.day] == nil {
				merged[r.day] = make(map[string]int)
			}
			merged[r.day][r.label] += r.cnt
		}
		// Flatten
		flattened := make([]trendRow, 0)
		for day, labelMap := range merged {
			for label, cnt := range labelMap {
				flattened = append(flattened, trendRow{day: day, label: label, cnt: cnt})
			}
		}
		sort.Slice(flattened, func(i, j int) bool {
			if flattened[i].day != flattened[j].day {
				return flattened[i].day < flattened[j].day
			}
			return flattened[i].label < flattened[j].label
		})

		// Build chart
		daySet := make(map[string]bool)
		labelSet := make(map[string]bool)
		for _, r := range flattened {
			daySet[r.day] = true
			labelSet[r.label] = true
		}
		days := make([]string, 0, len(daySet))
		for d := range daySet {
			days = append(days, d)
		}
		sort.Strings(days)

		sortedLabels := make([]string, 0, len(labelSet))
		for l := range labelSet {
			sortedLabels = append(sortedLabels, l)
		}
		labelOrder := make(map[string]int)
		for i, l := range ageBroadLabels {
			labelOrder[l] = i
		}
		sort.Slice(sortedLabels, func(i, j int) bool {
			oi, okI := labelOrder[sortedLabels[i]]
			oj, okJ := labelOrder[sortedLabels[j]]
			if okI && okJ {
				return oi < oj
			}
			if okI {
				return true
			}
			if okJ {
				return false
			}
			return sortedLabels[i] < sortedLabels[j]
		})

		dataMap := make(map[string]map[string]int)
		for _, r := range flattened {
			if dataMap[r.day] == nil {
				dataMap[r.day] = make(map[string]int)
			}
			dataMap[r.day][r.label] = r.cnt
		}

		series := make([]chartSeries, 0, len(sortedLabels))
		for _, label := range sortedLabels {
			data := make([]int, 0, len(days))
			for _, day := range days {
				data = append(data, dataMap[day][label])
			}
			series = append(series, chartSeries{Name: label, Data: data})
		}

		tableRows := make([][]any, 0, len(days)*len(sortedLabels))
		for _, day := range days {
			for _, label := range sortedLabels {
				tableRows = append(tableRows, []any{day, label, dataMap[day][label]})
			}
		}

		chartTitle := "年龄趋势（" + typeName + "）"
		if len(sortedLabels) <= 3 {
			chartTitle = strings.Join(sortedLabels, "/") + "趋势"
		}

		chart := buildChart("line", chartTitle, days, series,
			[]string{"日期", tag, "人数"}, tableRows)

		totalByLabel := make(map[string]int)
		for _, r := range flattened {
			totalByLabel[r.label] += r.cnt
		}

		topLabel := ""
		topTotal := 0
		for label, total := range totalByLabel {
			if total > topTotal {
				topTotal = total
				topLabel = label
			}
		}

		answer := fmt.Sprintf("「%s」维度%s%s趋势：人数最多的是「%s」，共%d人。",
				tag, typeName, p.PeriodLabel(7), topLabel, topTotal)

		firstVal := 0
		lastVal := 0
		for i, day := range days {
			if v, ok := dataMap[day][topLabel]; ok {
				if i == 0 {
					firstVal = v
				}
				lastVal = v
			}
		}
		if firstVal > 0 {
			change := float64(lastVal-firstVal) / float64(firstVal) * 100
			if change > 5 {
				answer += fmt.Sprintf(" 「%s」呈上升趋势（+%.1f%%）。", topLabel, change)
			} else if change < -5 {
				answer += fmt.Sprintf(" 「%s」呈下降趋势（%.1f%%）。", topLabel, change)
			} else {
				answer += " 整体趋势平稳。"
			}
		}

		return answer, chart, nil
	}

	// Non-age path: original logic
	daySet := make(map[string]bool)
	labelSet := make(map[string]bool)
	for _, r := range records {
		daySet[r["day"].String()] = true
		labelSet[r["label"].String()] = true
	}

	days := make([]string, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sort.Strings(days)

	sortedLabels := make([]string, 0, len(labelSet))
	for l := range labelSet {
		sortedLabels = append(sortedLabels, l)
	}
	sort.Strings(sortedLabels)

	dataMap := make(map[string]map[string]int)
	for _, r := range records {
		day := r["day"].String()
		label := r["label"].String()
		cnt := r["cnt"].Int()
		if dataMap[day] == nil {
			dataMap[day] = make(map[string]int)
		}
		dataMap[day][label] = cnt
	}

	series := make([]chartSeries, 0, len(sortedLabels))
	for _, label := range sortedLabels {
		data := make([]int, 0, len(days))
		for _, day := range days {
			data = append(data, dataMap[day][label])
		}
		series = append(series, chartSeries{Name: label, Data: data})
	}

	tableRows := make([][]any, 0, len(days)*len(sortedLabels))
	for _, day := range days {
		for _, label := range sortedLabels {
			tableRows = append(tableRows, []any{day, label, dataMap[day][label]})
		}
	}

	chartTitle := tag + "趋势（" + typeName + "）"
	if len(sortedLabels) <= 3 {
		chartTitle = strings.Join(sortedLabels, "/") + "趋势"
	}

	chart := buildChart("line", chartTitle, days, series,
		[]string{"日期", tag, "人数"}, tableRows)

	totalByLabel := make(map[string]int)
	for _, r := range records {
		label := r["label"].String()
		totalByLabel[label] += r["cnt"].Int()
	}

	topLabel := ""
	topTotal := 0
	for label, total := range totalByLabel {
		if total > topTotal {
			topTotal = total
			topLabel = label
		}
	}

	answer := fmt.Sprintf("「%s」维度%s%s趋势：人数最多的是「%s」，共%d人。",
			tag, typeName, p.PeriodLabel(7), topLabel, topTotal)

	firstVal := 0
	lastVal := 0
	for i, day := range days {
		if v, ok := dataMap[day][topLabel]; ok {
			if i == 0 {
				firstVal = v
			}
			lastVal = v
		}
	}
	if firstVal > 0 {
		change := float64(lastVal-firstVal) / float64(firstVal) * 100
		if change > 5 {
			answer += fmt.Sprintf(" 「%s」呈上升趋势（+%.1f%%）。", topLabel, change)
		} else if change < -5 {
			answer += fmt.Sprintf(" 「%s」呈下降趋势（%.1f%%）。", topLabel, change)
		} else {
			answer += " 整体趋势平稳。"
		}
	}

	return answer, chart, nil
}
