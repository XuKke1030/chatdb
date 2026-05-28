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
	}{
		{"今天车流", 1},
		{"今日车流量", 1},
		{"近3天车流", 3},
		{"最近7天车流", 7},
		{"近30天趋势", 30},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			_, _, days, ok := dateRangeParams(tt.query, now)
			if !ok {
				t.Fatalf("dateRangeParams(%q) ok=false, want true", tt.query)
			}
			if days != tt.wantDays {
				t.Fatalf("dateRangeParams(%q) days=%d, want %d", tt.query, days, tt.wantDays)
			}
		})
	}
}

func TestExtractFastPathParams_Region(t *testing.T) {
	p := ExtractFastPathParams(context.Background(), "traffic", "香洲区车流趋势")
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

func TestPopTagParams_TagExtraction(t *testing.T) {
	tests := []struct {
		query      string
		wantTag    string
		wantType   int
		wantLabels []string
	}{
		{"年龄分布", "年龄", 1, nil},
		{"年龄段占比", "年龄", 1, nil},
		{"进站年龄分布", "年龄", 2, nil},
		{"出站性别分布", "性别", 3, nil},
		{"年轻人群的进站趋势", "年龄", 2, []string{"(0,18]", "(19,22]", "(23,25]", "(26,30]"}},
		{"中年人出站变化", "年龄", 3, []string{"(31,35]", "(36,40]", "(41,45]", "(46,50]"}},
		{"老年人流趋势", "年龄", 1, []string{"(51,55]", "(56,60]", ">60"}},
		{"省外来源排名前5", "省外来源", 1, nil},
		{"省内城市来源分布", "省内城市来源", 1, nil},
		{"来源地排名", "省外来源", 1, nil},
		{"来自哪里", "省外来源", 1, nil},
		{"男性进站人流", "性别", 2, []string{"男"}},
		{"女性出站人流", "性别", 3, []string{"女"}},
		{"女的占比", "性别", 1, []string{"女"}},
		{"男性占比", "性别", 1, []string{"男"}},
		{"男的进站人流", "性别", 2, []string{"男"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			p := &ExtractedParams{}
			popTagParams(tt.query, p)
			if p.Tag != tt.wantTag {
				t.Fatalf("tag=%q, want %q", p.Tag, tt.wantTag)
			}
			if p.PopType != tt.wantType {
				t.Fatalf("type=%d, want %d", p.PopType, tt.wantType)
			}
			if tt.wantLabels != nil {
				if len(p.Labels) != len(tt.wantLabels) {
					t.Fatalf("labels=%v, want %v", p.Labels, tt.wantLabels)
				}
				for i, l := range p.Labels {
					if l != tt.wantLabels[i] {
						t.Fatalf("labels[%d]=%q, want %q", i, l, tt.wantLabels[i])
					}
				}
			}
		})
	}
}

func TestPopTagParams_TopN(t *testing.T) {
	tests := []struct {
		query    string
		wantTopN int
	}{
		{"来源地排名前5", 5},
		{"年龄段排名前10", 10},
		{"排名前3", 3},
		{"来源地排名", 5},
		{"排行", 5},
		{"top3来源", 3},
		{"Top5年龄", 5},
		{"TOP10", 10},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			p := &ExtractedParams{}
			popTagParams(tt.query, p)
			if p.TopN != tt.wantTopN {
				t.Fatalf("topN=%d, want %d", p.TopN, tt.wantTopN)
			}
		})
	}
}

func TestPopTagParams_PopTypeDefault(t *testing.T) {
	tests := []struct {
		query    string
		wantType int
	}{
		{"年龄分布", 1},
		{"性别占比", 1},
		{"进站年龄分布", 2},
		{"出站性别占比", 3},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			p := &ExtractedParams{}
			popTagParams(tt.query, p)
			if p.PopType != tt.wantType {
				t.Fatalf("popType=%d, want %d", p.PopType, tt.wantType)
			}
		})
	}
}

func TestPopTagParams_Area(t *testing.T) {
	p := &ExtractedParams{RegionName: "香洲区"}
	popTagParams("年龄分布", p)
	if p.Area != "香洲区" {
		t.Fatalf("expected area=香洲区, got %q", p.Area)
	}
}
