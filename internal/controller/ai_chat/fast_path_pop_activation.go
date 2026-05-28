package ai_chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func fastPopActivationSummaryQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	queryDate := gtime.Now().Format("Y-m-d")
	if p.DateFrom != "" {
		queryDate = p.DateFrom[:10]
	}

	record, err := db.Ctx(ctx).Raw(
		"SELECT statistics_date, all_count, in_count, out_count, activation, base_line_value FROM pl_mobile_people_flow_data WHERE statistics_date = DATE(?)",
		queryDate,
	).One()
	if err != nil {
		return "查询人流活力指数时出错：" + err.Error(), "", nil
	}

	if record == nil || record.IsEmpty() {
		return p.PeriodLabel(1) + "暂无人流活力数据。", "", nil
	}

	dateLabel := record["statistics_date"].String()
	if dateLabel == "" {
		dateLabel = queryDate
	}
	act := record["activation"].Float64()
	baseline := record["base_line_value"].Float64()
	allCount := record["all_count"].Int()
	inCount := record["in_count"].Int()
	outCount := record["out_count"].Int()

	deviation := ""
	if baseline > 0 {
		pct := (act - baseline) / baseline * 100
		if pct > 10 {
			deviation = fmt.Sprintf("，显著高于基线(%.2f)，偏离+%.1f%%", baseline, pct)
		} else if pct < -10 {
			deviation = fmt.Sprintf("，低于基线(%.2f)，偏离%.1f%%", baseline, pct)
		} else {
			deviation = fmt.Sprintf("，与基线(%.2f)基本持平", baseline)
		}
	}

	answer := fmt.Sprintf(dateLabel + "人流总计 %d 人（进 %d / 出 %d），活力指数 %.2f%s。",
		allCount, inCount, outCount, act, deviation)

	// Build metric-card style chart with key indicators
	tableRows := [][]any{
		{"总人流", allCount},
		{"进站", inCount},
		{"出站", outCount},
		{"活力指数", fmt.Sprintf("%.2f", act)},
		{"基线值", fmt.Sprintf("%.2f", baseline)},
	}
	chart := buildChart("metric_card", "人流活力指标", []string{},
		[]chartSeries{},
		[]string{"指标", "值"}, tableRows)

	return answer, chart, nil
}

func fastPopActivationTrendQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	db := g.DB("master")
	days := p.Days
	if days <= 0 {
		days = 7
	}
	from, to := popEffectiveRange(days)
	if p.Days > 0 && p.DateFrom != "" {
		from = p.DateFrom[:10]
		to = p.DateTo[:10]
	}

	records, err := db.Ctx(ctx).Raw(
		"SELECT statistics_date, activation, base_line_value, all_count, in_count, out_count FROM pl_mobile_people_flow_data WHERE statistics_date >= DATE(?) AND statistics_date <= DATE(?) ORDER BY statistics_date",
		from, to,
	).All()
	if err != nil {
		return "查询人流活力趋势时出错：" + err.Error(), "", nil
	}
	if len(records) == 0 {
		return p.PeriodLabel(7) + "暂无人流活力趋势数据。", "", nil
	}

	xLabels := make([]string, 0, len(records))
	actData := make([]int, 0, len(records))
	baseData := make([]int, 0, len(records))
	tableRows := make([][]any, 0, len(records))

	for _, r := range records {
		day := r["statistics_date"].String()
		xLabels = append(xLabels, day)
		act := r["activation"].Float64()
		base := r["base_line_value"].Float64()
		// Scale by 100 for chart display (chart uses int)
		actData = append(actData, int(act*100))
		baseData = append(baseData, int(base*100))
		tableRows = append(tableRows, []any{day, fmt.Sprintf("%.2f", act), fmt.Sprintf("%.2f", base), r["all_count"].Int()})
	}

	chart := buildChart("line", p.PeriodLabel(7)+"活力指数与基线对比", xLabels,
		[]chartSeries{
			{Name: "活力指数(×100)", Data: actData},
			{Name: "基线值(×100)", Data: baseData},
		},
		[]string{"日期", "活力指数", "基线值", "总人流"}, tableRows)

	// Trend direction from first to last
	firstAct := records[0]["activation"].Float64()
	lastAct := records[len(records)-1]["activation"].Float64()
	direction := "平稳"
	if lastAct > firstAct*1.05 {
		direction = "上升"
	} else if lastAct < firstAct*0.95 {
		direction = "下降"
	}

	// Count days above baseline and detect crossover points
	aboveBaseline := 0
	crossovers := make([]string, 0)
	for i, r := range records {
		act := r["activation"].Float64()
		base := r["base_line_value"].Float64()
		if act > base {
			aboveBaseline++
		}
		if i > 0 {
			prevAct := records[i-1]["activation"].Float64()
			prevBase := records[i-1]["base_line_value"].Float64()
			prevDiff := prevAct - prevBase
			currDiff := act - base
			if (prevDiff < 0 && currDiff > 0) || (prevDiff > 0 && currDiff < 0) {
				day := r["statistics_date"].String()
				crossDir := "上穿基线"
				if currDiff < 0 {
					crossDir = "下穿基线"
				}
				crossovers = append(crossovers, fmt.Sprintf("%s%s", day, crossDir))
			}
		}
	}

	answer := fmt.Sprintf("%s活力指数趋势%s，最新值 %.2f（基线 %.2f），%d天高于基线。",
			p.PeriodLabel(7), direction, lastAct, records[len(records)-1]["base_line_value"].Float64(), aboveBaseline)
	if len(crossovers) > 0 {
		answer += fmt.Sprintf(" 交叉点：%s。", strings.Join(crossovers, "、"))
	}

	return answer, chart, nil
}
