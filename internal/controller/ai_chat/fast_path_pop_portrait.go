package ai_chat

import (
	"context"
	"fmt"

	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func fastPopPortraitQuery(ctx context.Context, p ExtractedParams) (string, string, error) {
	from, to := popEffectiveRange(7)

	charts := make([]string, 0, 5)

	// 1. Summary from pl_mobile_people_flow_data (latest day)
	db := g.DB("master")
	queryDate := gtime.Now().Format("Y-m-d")
	if p.DateFrom != "" {
		queryDate = p.DateFrom[:10]
	}

	actRecord, err := db.Ctx(ctx).Raw(
		"SELECT all_count, in_count, out_count, activation, base_line_value FROM pl_mobile_people_flow_data WHERE statistics_date = DATE(?)",
		queryDate,
	).One()
	if err != nil {
		return "查询人流画像时出错：" + err.Error(), "", nil
	}
	if actRecord == nil || actRecord.IsEmpty() {
		return p.PeriodLabel(1) + "暂无人流画像数据。", "", nil
	}

	allCount, inCount, outCount := 0, 0, 0
	act := 0.0
	baseline := 0.0
	if actRecord != nil && !actRecord.IsEmpty() {
		allCount = actRecord["all_count"].Int()
		inCount = actRecord["in_count"].Int()
		outCount = actRecord["out_count"].Int()
		act = actRecord["activation"].Float64()
		baseline = actRecord["base_line_value"].Float64()
	}

	// 1b. Metric card with key indicators
	metricRows := [][]any{
		{"总人流", allCount},
		{"进站", inCount},
		{"出站", outCount},
		{"活力指数", fmt.Sprintf("%.2f", act)},
		{"基线值", fmt.Sprintf("%.2f", baseline)},
	}
	charts = append(charts, buildChart("metric_card", "人流活力指标", []string{},
		[]chartSeries{},
		[]string{"指标", "值"}, metricRows))

	// 2. Activation + baseline trend line chart (7-day)
	actRecords, actErr := db.Ctx(ctx).Raw(
		"SELECT statistics_date, activation, base_line_value, all_count, in_count, out_count FROM pl_mobile_people_flow_data WHERE statistics_date >= DATE(?) AND statistics_date <= DATE(?) ORDER BY statistics_date",
		from, to,
	).All()
	if actErr == nil && actRecords != nil && len(actRecords) > 0 {
		xLabels := make([]string, 0, len(actRecords))
		actData := make([]int, 0, len(actRecords))
		baseData := make([]int, 0, len(actRecords))
		actRows := make([][]any, 0, len(actRecords))
		for _, r := range actRecords {
			day := r["statistics_date"].String()
			xLabels = append(xLabels, day)
			actData = append(actData, int(r["activation"].Float64()*100))
			baseData = append(baseData, int(r["base_line_value"].Float64()*100))
			actRows = append(actRows, []any{day, r["activation"].Float64(), r["base_line_value"].Float64(), r["all_count"].Int()})
		}
		if len(xLabels) > 0 {
			charts = append(charts, buildChart("line", p.PeriodLabel(7)+"活力与基线趋势", xLabels,
				[]chartSeries{{Name: "活力指数(x100)", Data: actData}, {Name: "基线值(x100)", Data: baseData}},
				[]string{"日期", "活力指数", "基线值", "总人流"}, actRows))
		}
	}

	// 3. In/out trend line chart (7-day)
	result, err := service.Population().Aggregate(ctx, model.PopulationAggregateQuery{
		DateFrom: from,
		DateTo:   to + " 23:59:59",
		GroupBy:  "day",
	})
	if err == nil && result != nil && len(result.Series) > 0 {
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
			charts = append(charts, buildChart("line", p.PeriodLabel(7)+"人流进出趋势", xLabels,
				[]chartSeries{{Name: "进入", Data: inData}, {Name: "离开", Data: outData}},
				[]string{"日期", "进入", "离开", "净流入", "流动人口"}, trendRows))
		}
	}

	// 4. Age distribution pie chart
	ageResult, ageErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to + " 23:59:59",
		Tag:      "年龄",
		Type:     1,
	})
	if ageErr == nil && ageResult != nil && len(ageResult.Items) > 0 {
		ageX := make([]string, 0, len(ageResult.Items))
		ageData := make([]int, 0, len(ageResult.Items))
		ageRows := make([][]any, 0, len(ageResult.Items))
		for _, item := range ageResult.Items {
			ageX = append(ageX, item.Label)
			ageData = append(ageData, item.Count)
			ageRows = append(ageRows, []any{item.Label, item.Count, item.Pct + "%"})
		}
		if len(ageX) > 0 {
			charts = append(charts, buildChart("pie", "年龄分布（总人数）", ageX,
				[]chartSeries{{Name: "人数", Data: ageData}},
				[]string{"年龄段", "人数", "占比"}, ageRows))
		}
	}

	// 5. Origin top5 bar_rank chart
	originResult, originErr := service.Population().TagDistribution(ctx, model.TagDistributionQuery{
		DateFrom: from,
		DateTo:   to + " 23:59:59",
		Tag:      "省外来源",
		Type:     1,
	})
	if originErr == nil && originResult != nil && len(originResult.Items) > 0 {
		topItems := originResult.Items
		if len(topItems) > 5 {
			topItems = topItems[:5]
		}
		originX := make([]string, 0, len(topItems))
		originData := make([]int, 0, len(topItems))
		originRows := make([][]any, 0, len(topItems))
		for i, item := range topItems {
			originX = append(originX, item.Label)
			originData = append(originData, item.Count)
			originRows = append(originRows, []any{fmt.Sprintf("%d", i+1), item.Label, item.Count, item.Pct + "%"})
		}
		if len(originX) > 0 {
			charts = append(charts, buildChart("bar_rank", "省外来源地排名Top5（总人数）", originX,
				[]chartSeries{{Name: "人数", Data: originData}},
				[]string{"排名", "来源地", "人数", "占比"}, originRows))
		}
	}

	// Build answer text
	answer := fmt.Sprintf("人流综合画像：%s总人流 %d 人（进 %d / 出 %d），活力指数 %.2f", p.PeriodLabel(1), allCount, inCount, outCount, act)
	if baseline > 0 {
		pct := (act - baseline) / baseline * 100
		if pct > 10 {
			answer += fmt.Sprintf("，显著高于基线(%.2f)", baseline)
		} else if pct < -10 {
			answer += fmt.Sprintf("，低于基线(%.2f)", baseline)
		} else {
			answer += fmt.Sprintf("，与基线(%.2f)基本持平", baseline)
		}
	}

	if ageResult != nil && len(ageResult.Items) > 0 {
		answer += fmt.Sprintf("；年龄分布最多为「%s」(占比%s%%)", ageResult.Items[0].Label, ageResult.Items[0].Pct)
	}
	if originResult != nil && len(originResult.Items) > 0 {
		answer += fmt.Sprintf("；省外来源第1为「%s」(占比%s%%)", originResult.Items[0].Label, originResult.Items[0].Pct)
	}
	answer += "。"

	// Combine charts
	chartData := ""
	for _, c := range charts {
		chartData += c + "\n\n"
	}
	return answer, chartData, nil
}
