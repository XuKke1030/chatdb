package ai_chat

import (
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := matchFastPath("traffic", tt.question)
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
	ratioPath := &fastPath{kind: fpTrafficHkMacau, topic: "traffic"}
	ratioAnswer := formatFastPathAnswer(ratioPath, "港澳车占比 10%。", "")
	for _, section := range []string{"## 精准结论", "## 特征洞察", "## 洞察分析"} {
		if !strings.Contains(ratioAnswer, section) {
			t.Fatalf("expected section %s in answer", section)
		}
	}
	if strings.Contains(ratioAnswer, "## 可视化") {
		t.Fatalf("single ratio answer should not include visualization section")
	}
	ratioMeta := buildFastPathFormatMeta(ratioPath, "")
	if ratioMeta.ChartRule != "no_chart_single_value_or_ratio" {
		t.Fatalf("expected no-chart rule, got %s", ratioMeta.ChartRule)
	}
	if !ratioMeta.InsightCollapsedDefault || !ratioMeta.NaturalLanguageOnly {
		t.Fatalf("expected collapsed insight and natural-language-only flags")
	}

	rankPath := &fastPath{kind: fpTrafficGateRank, topic: "traffic"}
	rankChart := "```chatdb-chart\n{}\n```"
	rankAnswer := formatFastPathAnswer(rankPath, "今日车流排名。", rankChart)
	if !strings.Contains(rankAnswer, "## 可视化") {
		t.Fatalf("rank answer should include visualization section")
	}
	rankMeta := buildFastPathFormatMeta(rankPath, rankChart)
	if rankMeta.ChartRule != "bar_rank" {
		t.Fatalf("expected bar_rank rule, got %s", rankMeta.ChartRule)
	}
}
