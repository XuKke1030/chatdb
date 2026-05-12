package ai_chat

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/os/gtime"
)

func TestDateRangeParams(t *testing.T) {
	now := gtime.NewFromStrFormat("2026-05-12", "Y-m-d")

	tests := []struct {
		query    string
		wantDays int
		wantOk   bool
	}{
		{"今天车流", 1, true},
		{"今日车流量", 1, true},
		{"昨天车流", 1, true},
		{"近3天车流", 3, true},
		{"最近7天车流", 7, true},
		{"近30天趋势", 30, true},
		{"近七天车流趋势", 7, true},
		{"本周车流", 2, true},
		{"上周车流", 7, true},
		{"本月车流", 12, true},
		{"车流怎么样", 0, false},
	}

	for _, tt := range tests {
		_, _, days, ok := dateRangeParams(tt.query, now)
		if ok != tt.wantOk {
			t.Errorf("dateRangeParams(%q) ok=%v, want %v", tt.query, ok, tt.wantOk)
			continue
		}
		if ok && days != tt.wantDays {
			t.Errorf("dateRangeParams(%q) days=%d, want %d", tt.query, days, tt.wantDays)
		}
	}
}

func TestRegionNameParams(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"香洲区案件情况", "香洲区"},
		{"斗门区车流", "斗门区"},
		{"金湾区人流趋势", "金湾区"},
		{"横琴新区车流", "横琴新区"},
		{"全市车流", ""},
		{"今天车流怎么样", ""},
	}

	for _, tt := range tests {
		got := regionNameParams(tt.query)
		if got != tt.want {
			t.Errorf("regionNameParams(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestCompareRefParams(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"节假日车流对比", "holiday"},
		{"周末车流和平日对比", "weekend"},
		{"上周车流环比", "lastweek"},
		{"今天车流", ""},
	}

	for _, tt := range tests {
		got := compareRefParams(tt.query)
		if got != tt.want {
			t.Errorf("compareRefParams(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestExtractFastPathParams_DefaultRange(t *testing.T) {
	p := ExtractFastPathParams(context.Background(), "traffic", "车流怎么样")
	if p.Days != 7 {
		t.Fatalf("expected default days=7, got %d", p.Days)
	}
	if p.DateFrom == "" || p.DateTo == "" {
		t.Fatal("expected non-empty date range")
	}
}

func TestExtractFastPathParams_Today(t *testing.T) {
	p := ExtractFastPathParams(context.Background(), "traffic", "今天车流怎么样")
	if p.Days != 1 {
		t.Fatalf("expected days=1, got %d", p.Days)
	}
}

func TestExtractFastPathParams_RegionName(t *testing.T) {
	p := ExtractFastPathParams(context.Background(), "traffic", "香洲区车流怎么样")
	if p.RegionName != "香洲区" {
		t.Fatalf("expected regionName=香洲区, got %q", p.RegionName)
	}
}

func TestExtractFastPathParams_NDays(t *testing.T) {
	p := ExtractFastPathParams(context.Background(), "traffic", "近3天车流趋势")
	if p.Days != 3 {
		t.Fatalf("expected days=3, got %d", p.Days)
	}
}
