package ai_chat

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestMatchFastPathTrafficPriority(t *testing.T) {
	tests := []struct {
		name     string
		question string
		want     fastPathKind
	}{
		{name: "top gate fuzzy today", question: "哪个地方车流大？", want: fpTrafficTopGateToday},
		{name: "holiday before weekday", question: "节假日车流和平日相比？", want: fpTrafficHolidayCompare},
		{name: "weekend weekday", question: "周末和平日车流有什么变化？", want: fpTrafficWeekendCompare},
		{name: "gate rank today", question: "车流量排名前十的卡口", want: fpTrafficGateRank},
		{name: "in out ratio", question: "进出车流占比多少？", want: fpTrafficInOutRatio},
		{name: "multi gate compare", question: "几个卡口对比一下", want: fpTrafficMultiGateCompare},
		{name: "hk macau yoy", question: "港澳车同比变化", want: fpTrafficHkMacauYoY},
		{name: "age proportion", question: "年龄占比多少", want: fpPopTagProportion},
		{name: "multi tag trend", question: "各年龄段趋势", want: fpPopMultiTagTrend},
		{name: "comprehensive analysis", question: "人流综合分析", want: fpPopComprehensive},
		{name: "multi tag compare", question: "年龄性别对比", want: fpPopMultiTagCompare},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := "traffic"
			if tt.want == fpPopTagProportion || tt.want == fpPopMultiTagTrend || tt.want == fpPopComprehensive || tt.want == fpPopMultiTagCompare {
				topic = "population"
			}
			fp := matchFastPath(topic, tt.question)
			if fp == nil {
				t.Fatalf("expected fast path, got nil")
			}
			if fp.kind != tt.want {
				t.Fatalf("expected kind %v, got %v", tt.want, fp.kind)
			}
		})
	}
}

func TestFormatFastPathAnswerSectionsAndChartRule(t *testing.T) {
	ctx := context.Background()
	ratioPath := &fastPath{kind: fpTrafficHkMacau, topic: "traffic"}
	ratioAnswer := formatFastPathAnswer(ctx, ratioPath, "港澳车占比 10%。", "")
	for _, section := range []string{"## 精准结论", "## 特征洞察", "## 洞察分析"} {
		if !strings.Contains(ratioAnswer, section) {
			t.Fatalf("expected section %s in answer", section)
		}
	}
	if strings.Contains(ratioAnswer, "## 可视化") {
		t.Fatalf("single ratio answer should not include visualization section")
	}
	ratioMeta := buildFastPathFormatMeta(ctx, ratioPath, "港澳车占比 10%。", "")
	if ratioMeta.ChartRule != "no_chart_single_value_or_ratio" {
		t.Fatalf("expected no-chart rule, got %s", ratioMeta.ChartRule)
	}
	if !ratioMeta.NaturalLanguageOnly {
		t.Fatalf("expected natural-language-only flag")
	}

	rankPath := &fastPath{kind: fpTrafficGateRank, topic: "traffic"}
	rankChart := "```chatdb-chart\n{}\n```"
	rankAnswer := formatFastPathAnswer(ctx, rankPath, "今日车流排名。", rankChart)
	if !strings.Contains(rankAnswer, "## 可视化") {
		t.Fatalf("rank answer should include visualization section")
	}
	rankMeta := buildFastPathFormatMeta(ctx, rankPath, "今日车流排名。", rankChart)
	if rankMeta.ChartRule != "bar_rank" {
		t.Fatalf("expected bar_rank rule, got %s", rankMeta.ChartRule)
	}
}

func TestBuildInsightMeta(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		topic      string
		kind       fastPathKind
		conclusion string
		wantKP     int
		wantImp    int
		wantSug    int
	}{
		{topic: "traffic", kind: fpTrafficToday, conclusion: "今日车流总计 12500 辆，其中进入 6200 辆，离开 6300 辆。港澳车 1200 辆，占比 9.6%。", wantKP: 1, wantImp: 1, wantSug: 1},
		{topic: "population", kind: fpPopWeekTrend, conclusion: "近七天人流 52000 人次，日均 7429 人次。", wantKP: 1, wantImp: 1, wantSug: 1},
		{topic: "grid", kind: fpGridCaseCount, conclusion: "本月案件 320 件，结案率 78.5%。", wantKP: 1, wantImp: 1, wantSug: 1},
	}
	for _, tc := range tests {
		fp := &fastPath{topic: tc.topic, kind: tc.kind}
		meta := buildInsightMeta(ctx, fp, tc.conclusion)
		if meta == nil {
			t.Fatalf("expected non-nil insightMeta for topic=%s kind=%v", tc.topic, tc.kind)
		}
		if len(meta.KeyPoints) < tc.wantKP {
			t.Fatalf("expected ≥%d keyPoints for topic=%s kind=%v, got %d (items=%v)", tc.wantKP, tc.topic, tc.kind, len(meta.KeyPoints), meta.KeyPoints)
		}
		if len(meta.Impacts) < tc.wantImp {
			t.Fatalf("expected ≥%d impacts for topic=%s kind=%v, got %d", tc.wantImp, tc.topic, tc.kind, len(meta.Impacts))
		}
		if len(meta.Suggestions) < tc.wantSug {
			t.Fatalf("expected ≥%d suggestions for topic=%s kind=%v, got %d", tc.wantSug, tc.topic, tc.kind, len(meta.Suggestions))
		}
	}
}

func TestConditionalInsightCollapse(t *testing.T) {
	ctx := context.Background()
	noTemplateFP := &fastPath{topic: "traffic", kind: fpTrafficForeignOrigin}
	noTemplateMeta := buildInsightMeta(ctx, noTemplateFP, "持平")
	if noTemplateMeta == nil {
		t.Fatal("expected non-nil insightMeta for no-template kind")
	}
	total := len(noTemplateMeta.KeyPoints) + len(noTemplateMeta.Impacts) + len(noTemplateMeta.Suggestions)
	if total <= 3 {
		if shouldCollapseInsight(noTemplateMeta) {
			t.Fatalf("≤3 short items should not be collapsed, got %d items", total)
		}
	}

	growthFP := &fastPath{topic: "traffic", kind: fpTrafficToday}
	growthMeta := buildInsightMeta(ctx, growthFP, "今日车流总计 12500 辆，较昨日增长 15%")
	if growthMeta == nil {
		t.Fatal("expected non-nil insightMeta for growth insight")
	}
	if !shouldCollapseInsight(growthMeta) {
		t.Fatal("growth insight with data should be collapsed")
	}

	if !shouldCollapseInsight(nil) {
		t.Fatal("nil insightMeta should default to collapsed")
	}
}

func TestInsightMetaInFormatMeta(t *testing.T) {
	ctx := context.Background()
	fp := &fastPath{kind: fpTrafficHkMacau, topic: "traffic"}
	meta := buildFastPathFormatMeta(ctx, fp, "港澳车占比 10%。", "")
	if meta.InsightMeta == nil {
		t.Fatal("expected InsightMeta to be populated in format meta")
	}
	if len(meta.InsightMeta.KeyPoints) == 0 {
		t.Fatal("expected at least 1 keyPoint in InsightMeta")
	}
}

// TestKindInsightQuality 验证每个有模板的 kind 至少产生 1 条洞察
func TestKindInsightQuality(t *testing.T) {
	ctx := context.Background()
	conclusions := map[fastPathKind]string{
		fpTrafficToday:            "今日车流总计 12500 辆，其中进入 6200 辆，离开 6300 辆。港澳车 1200 辆，占比 9.6%。",
		fpTrafficTopGateToday:     "今日车流量最大的卡口为「皇岗口岸」，共 3500 辆。",
		fpTrafficWeek:             "近7天车流总计 85000 辆，日均 12143 辆。港澳车占比 8.5%。",
		fpTrafficHkMacau:          "近七天车流中，港澳车 5200 辆，占总车流 8.5%；内地车 56200 辆，占比 91.5%。",
		fpTrafficGateRank:         "排名首位卡口「皇岗口岸」车流 3500 辆。",
		fpTrafficWeekendCompare:   "周末与工作日日均车流差异 18.5%。",
		fpTrafficHolidayCompare:   "节假日与工作日日均车流差异 25.3%。",
		fpTrafficProvinceInside:   "近七天车流中，省内车 35000 辆（占大陆车62.5%），省外车 21000 辆。",
		fpTrafficYoY:              "近七天车流同比增长 22.5%。",
		fpTrafficMoM:              "近一个月车流环比增长 12.5%。",
		fpTrafficHoliday:          "国庆期间车流总计 28000 辆，日均 4000 辆。",
		fpTrafficStayDistribution: "近七天共有 8500 辆车有停留记录，主要集中在0-30min时段。",
		fpTrafficOriginByProvince: "近七天省外车主要来自「湖南」，共 5200 辆。",
		fpTrafficOverview:         "近七天车流总计 85000 辆，日均 12143 辆。港澳车占比 8.5%，省内车占大陆车 62.5%。",
		fpPopWeekTrend:            "近七天人流 52000 人次，日均 7429 人次。",
		fpPopRegionRank:           "人流最大区域「罗湖区」，共 15000 人次。",
		fpPopHourlyTrend:          "人流高峰时段14:00，峰值 3200 人次。",
		fpPopOverview:             "近七天人流 52000 人次，日均 7429 人次。",
		fpGridCaseCount:           "本月案件 320 件，结案率 78.5%。",
		fpGridCloseRate:           "结案率 78.5%，低于目标值85%。",
		fpGridRegionRank:          "案件最多区域「XX社区」，共 85 件。",
		fpGridCaseTypeDist:        "案件类型以「城市管理」为主，占比 35.2%。",
		fpPopTagDistribution:      "「年龄」维度分布：最多的是「30-50」，共7567人，占比18.3%。",
		fpPopTagTopN:              "「省外来源」维度排名前5：第1名为「湖南」，共5200人，占比22.5%。",
		fpPopTagTrend:             "「年龄」维度总人数近七天趋势：人数最多的是「30-50」，共7567人。 「30-50」呈上升趋势（+8.5%）。",
		fpPopActivationSummary:    "今日人流总计 276450 人（进 53385 / 出 35327），活力指数 8.02，显著高于基线(5.61)，偏离+43.0%。",
		fpPopActivationTrend:      "近七天活力指数趋势上升，最新值 8.02（基线 5.61），5天高于基线。",
		fpPopPortrait:             "人流综合画像：今日总人流 276450 人（进 53385 / 出 35327），活力指数 8.02，显著高于基线(5.61)；年龄分布最多为「30-50」(占比18.3%)；省外来源第1为「湖南」(占比22.5%)。",
		fpTrafficInOutRatio:       "近七天车流中，进入 52000 辆（占比 48.5%），离开 55000 辆（占比 51.2%），方向不明 500 辆。",
		fpTrafficMultiGateCompare: "近七天车流量最大的卡口为「皇岗口岸」（3500辆）。",
		fpTrafficHkMacauYoY:       "近七天港澳车同比增长 15.3%。",
		fpPopTagProportion:        "「年龄」维度占比最高为「30-50」（35.2%）。",
		fpPopMultiTagTrend:        "「30-50」趋势上升，变化幅度 8.5%。",
		fpPopComprehensive:        "近七天人流 52000 人次，区域最多「罗湖区」。年龄占比最高「30-50」(35.2%)。省外来源第1「湖南」(22.5%)。",
		fpPopMultiTagCompare:      "多维标签对比：年龄占比最高「30-50」(35.2%)；性别：男52.3%/女47.7%；省外来源第1「湖南」(22.5%)。",
	}

	for kind, conclusion := range conclusions {
		t.Run(fastPathIntentName(kind), func(t *testing.T) {
			fp := &fastPath{kind: kind, topic: kindTopic(kind)}
			meta := buildInsightMeta(ctx, fp, conclusion)
			if meta == nil {
				t.Fatalf("kind=%v: expected non-nil insightMeta", kind)
			}
			total := len(meta.KeyPoints) + len(meta.Impacts) + len(meta.Suggestions)
			if total == 0 {
				t.Fatalf("kind=%v: expected ≥1 insight items, got 0", kind)
			}
		})
	}
}

// TestInsightTextContainsNumbers 验证洞察文本包含实际数值
func TestInsightTextContainsNumbers(t *testing.T) {
	ctx := context.Background()
	numRe := regexp.MustCompile(`\d+`)
	fp := &fastPath{kind: fpTrafficToday, topic: "traffic"}
	answer := formatFastPathAnswer(ctx, fp, "今日车流总计 12500 辆。", "")
	insightSection := strings.Split(answer, "## 洞察分析")
	if len(insightSection) < 2 {
		t.Fatal("expected 洞察分析 section in answer")
	}
	if !numRe.MatchString(insightSection[1]) {
		t.Fatal("expected insight section to contain numeric values")
	}
}

// kindTopic 返回 kind 对应的 topic，用于测试
func kindTopic(kind fastPathKind) string {
	switch {
	case kind >= fpTrafficToday && kind <= fpTrafficOverview,
		kind == fpTrafficInOutRatio, kind == fpTrafficMultiGateCompare, kind == fpTrafficHkMacauYoY:
		return "traffic"
	case kind >= fpPopWeekTrend && kind <= fpPopPortrait,
		kind == fpPopTagProportion, kind == fpPopMultiTagTrend, kind == fpPopComprehensive, kind == fpPopMultiTagCompare:
		return "population"
	case kind >= fpGridCaseCount && kind <= fpGridOverview:
		return "grid"
	default:
		return ""
	}
}

// TestNewInsightDataPoints 验证任务8新增的洞察数据提取
func TestNewInsightDataPoints(t *testing.T) {
	ctx := context.Background()

	t.Run("tag_distribution_change_dir", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopTagDistribution, "「年龄」分布最多为「(31,35]」，占比18.3%，较上月增长")
		if data["change_dir"] != "上升" {
			t.Fatalf("expected change_dir=上升, got %q", data["change_dir"])
		}
		data2 := extractInsightData(ctx, fpPopTagDistribution, "「年龄」分布最多为「(31,35]」，占比18.3%，较上月下降")
		if data2["change_dir"] != "下降" {
			t.Fatalf("expected change_dir=下降, got %q", data2["change_dir"])
		}
	})

	t.Run("tag_topn_top3_pct", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopTagTopN, "「省外来源」维度排名前5：第1名为「湖南」，共5200人，占比22.5%。第2名15.3%。第3名12.1%")
		if data["top3_pct"] == "" {
			t.Fatal("expected top3_pct to be set")
		}
	})

	t.Run("activation_summary_is_anomaly", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopActivationSummary, "今日人流总计 276450 人，活力指数 8.02，显著高于基线(5.61)，偏离+43.0%")
		if data["is_anomaly"] != "活力异常偏高，需关注" {
			t.Fatalf("expected is_anomaly=活力异常偏高，需关注, got %q", data["is_anomaly"])
		}
		data2 := extractInsightData(ctx, fpPopActivationSummary, "活力指数 3.02，低于基线(5.61)")
		if data2["is_anomaly"] != "活力偏低，需排查原因" {
			t.Fatalf("expected is_anomaly=活力偏低，需排查原因, got %q", data2["is_anomaly"])
		}
	})

	t.Run("activation_trend_crossover", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopActivationTrend, "近七天活力指数趋势上升，最新值 8.02（基线 5.61），5天高于基线 2026-05-18上穿基线")
		if data["crossover_point"] == "" {
			t.Fatal("expected crossover_point to be set")
		}
		// Also test no crossover
		data2 := extractInsightData(ctx, fpPopActivationTrend, "近七天活力指数趋势平稳，最新值 5.61（基线 5.61），7天高于基线")
		if data2["crossover_point"] != "近期无基线交叉" {
			t.Fatalf("expected crossover_point=近期无基线交叉, got %q", data2["crossover_point"])
		}

	t.Run("tag_trend_change_pct", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopTagTrend, "「(31,35]」呈上升趋势（+8.5%）")
		if data["change_pct"] != "8.5" {
			t.Fatalf("expected change_pct=8.5, got %q", data["change_pct"])
		}
		if data["trend_dir"] != "上升" {
			t.Fatalf("expected trend_dir=上升, got %q", data["trend_dir"])
		}
	})

	t.Run("activation_summary_deviation_pct", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopActivationSummary, "活力指数 8.02，显著高于基线(5.61)，偏离+43.0%")
		if data["deviation_pct"] != "+43.0" {
			t.Fatalf("expected deviation_pct=+43.0, got %q", data["deviation_pct"])
		}
	})

	t.Run("portrait_data", func(t *testing.T) {
		data := extractInsightData(ctx, fpPopPortrait, "人流综合画像：今日总人流 276450 人（进 53385 / 出 35327），活力指数 8.02，显著高于基线(5.61)；年龄分布最多为「(31,35]」(占比18.3%)；省外来源第1为「湖南」(占比22.5%)")
		if data["all_count"] == "" {
			t.Fatal("expected all_count to be set")
		}
		if data["age_top_label"] == "" {
			t.Fatal("expected age_top_label to be set")
		}
		if data["origin_top_label"] == "" {
			t.Fatal("expected origin_top_label to be set")
		}
	})	})
}
