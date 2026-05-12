package ai_chat

import (
	"context"
	"strings"
	"time"

	v1 "ai-chat-sql/api/ai_chat/v1"
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
)

type FastPathIntent struct {
	Intent     string         `json:"intent"`
	Topic      string         `json:"topic"`
	Question   string         `json:"question"`
	DatabaseId int            `json:"databaseId"`
	Cacheable  bool           `json:"cacheable"`
	Params     map[string]any `json:"params,omitempty"`
	legacy     *fastPath
}

type FastPathResult struct {
	FastPath   bool               `json:"fastPath"`
	CacheHit   bool               `json:"cacheHit"`
	Intent     string             `json:"intent"`
	Topic      string             `json:"topic"`
	Answer     string             `json:"answer"`
	Conclusion string             `json:"conclusion"`
	ChartData  string             `json:"chartData,omitempty"`
	Format     FastPathFormatMeta `json:"format"`
	QueryMs    int64              `json:"queryMs"`
	FormatMs   int64              `json:"formatMs"`
	TotalMs    int64              `json:"totalMs"`
}

type FastPathFormatMeta struct {
	Version                 string   `json:"version"`
	Sections                []string `json:"sections"`
	ChartRule               string   `json:"chartRule"`
	InsightCollapsedDefault bool     `json:"insightCollapsedDefault"`
	NaturalLanguageOnly     bool     `json:"naturalLanguageOnly"`
}

type FastPathRouter struct{}

func NewFastPathRouter() *FastPathRouter {
	return &FastPathRouter{}
}

func (r *FastPathRouter) Match(req *v1.ChatReq) (*FastPathIntent, bool) {
	if req == nil {
		return nil, false
	}
	fp := matchFastPath(req.Topic, req.Message)
	if fp == nil {
		return nil, false
	}
	return &FastPathIntent{
		Intent:     fastPathIntentName(fp.kind),
		Topic:      req.Topic,
		Question:   req.Message,
		DatabaseId: req.DatabaseId,
		Cacheable:  true,
		Params:     map[string]any{},
		legacy:     fp,
	}, true
}

func executeFastPathIntent(ctx context.Context, intent *FastPathIntent) (*FastPathResult, error) {
	totalStart := time.Now()
	var cacheKey string
	var ttl time.Duration

	// Check cache first
	if intent.Cacheable {
		cacheKey = BuildCacheKey(intent.Topic, intent.Intent, intent.DatabaseId, intent.Params)
		ttl = intentTTL(intent.Intent)
		if cached, hit := GetCache(ctx, cacheKey); hit {
			cached.CacheHit = true
			cached.QueryMs = 0
			cached.FormatMs = 0
			cached.TotalMs = time.Since(totalStart).Milliseconds()
			if cached.Format.Version == "" {
				cached.Format = buildFastPathFormatMeta(intent.legacy, cached.ChartData)
			}
			consts.Logger.Infof(ctx, "AskNumberCache hit key=%s ttl=%s intent=%s topic=%s totalMs=%d", cacheKey, ttl, intent.Intent, intent.Topic, cached.TotalMs)
			return cached, nil
		}
		consts.Logger.Infof(ctx, "AskNumberCache miss key=%s ttl=%s intent=%s topic=%s", cacheKey, ttl, intent.Intent, intent.Topic)
	}

	queryStart := time.Now()
	conclusion, chartData, err := executeFastPath(ctx, intent.legacy)
	queryMs := time.Since(queryStart).Milliseconds()
	if err != nil {
		return nil, err
	}
	formatStart := time.Now()
	answer := formatFastPathAnswer(intent.legacy, conclusion, chartData)
	formatMs := time.Since(formatStart).Milliseconds()
	result := &FastPathResult{
		FastPath:   true,
		CacheHit:   false,
		Intent:     intent.Intent,
		Topic:      intent.Topic,
		Answer:     answer,
		Conclusion: conclusion,
		ChartData:  chartData,
		Format:     buildFastPathFormatMeta(intent.legacy, chartData),
		QueryMs:    queryMs,
		FormatMs:   formatMs,
		TotalMs:    time.Since(totalStart).Milliseconds(),
	}

	// Write result to cache
	if intent.Cacheable {
		if cacheKey == "" {
			cacheKey = BuildCacheKey(intent.Topic, intent.Intent, intent.DatabaseId, intent.Params)
		}
		SetCache(ctx, cacheKey, result)
	}

	return result, nil
}

func (c *ControllerV1) streamFastPathIntentAnswer(ctx context.Context, intent *FastPathIntent, req *v1.ChatReq, userId int, sessionId string) (*v1.ChatRes, error) {
	result, err := executeFastPathIntent(ctx, intent)
	if err != nil {
		return nil, err
	}
	cacheKey := ""
	ttl := time.Duration(0)
	if intent.Cacheable {
		cacheKey = BuildCacheKey(intent.Topic, intent.Intent, intent.DatabaseId, intent.Params)
		ttl = intentTTL(intent.Intent)
	}
	consts.Logger.Infof(ctx, "perf ask_number_fast_path intent=%s topic=%s databaseId=%d cacheHit=%v cacheKey=%s ttl=%s queryMs=%d formatMs=%d totalMs=%d chart=%v formatVersion=%s",
		result.Intent, result.Topic, intent.DatabaseId, result.CacheHit, cacheKey, ttl, result.QueryMs, result.FormatMs, result.TotalMs, result.ChartData != "", result.Format.Version)

	r := g.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")

	sseData := g.Map{
		"fastPath":   true,
		"cacheHit":   result.CacheHit,
		"intent":     result.Intent,
		"topic":      result.Topic,
		"queryMs":    result.QueryMs,
		"formatMs":   result.FormatMs,
		"totalMs":    result.TotalMs,
		"databaseId": intent.DatabaseId,
		"format":     result.Format,
	}
	if intent.Cacheable {
		sseData["cacheKey"] = cacheKey
		sseData["ttl"] = ttl.String()
	}

	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "start", Data: g.Map{"sessionId": sessionId}})
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "fast_path", Data: sseData})
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "message", Role: "assistant", Content: result.Answer})
	writeChatSSE(ctx, r, model.ChatOutDataItem{Event: "end", Data: g.Map{"sessionId": sessionId}})

	if saveErr := appendAskNumberMessage(ctx, userId, sessionId, req.Topic, "assistant", result.Answer); saveErr != nil {
		consts.Logger.Errorf(ctx, "保存快路径回答失败: %s", saveErr.Error())
	}
	return &v1.ChatRes{}, nil
}

func buildFastPathFormatMeta(fp *fastPath, chartData string) FastPathFormatMeta {
	sections := []string{"精准结论", "特征洞察", "洞察分析"}
	if strings.TrimSpace(chartData) != "" {
		sections = []string{"精准结论", "特征洞察", "可视化", "洞察分析"}
	}
	return FastPathFormatMeta{
		Version:                 "ask-number-fastpath-v1",
		Sections:                sections,
		ChartRule:               fastPathChartRule(fp, chartData),
		InsightCollapsedDefault: true,
		NaturalLanguageOnly:     true,
	}
}

func fastPathChartRule(fp *fastPath, chartData string) string {
	if strings.TrimSpace(chartData) == "" {
		return "no_chart_single_value_or_ratio"
	}
	if fp == nil {
		return "chart"
	}
	switch fp.kind {
	case fpTrafficWeek, fpPopWeekTrend:
		return "line_trend"
	case fpTrafficTopGateToday, fpTrafficGateRank, fpTrafficForeignOrigin, fpPopRegionRank, fpGridRegionRank:
		return "bar_rank"
	case fpTrafficWeekendCompare, fpTrafficHolidayCompare:
		return "bar_compare"
	default:
		return "chart"
	}
}

func fastPathIntentName(kind fastPathKind) string {
	switch kind {
	case fpTrafficToday:
		return "traffic.summary.today"
	case fpTrafficTopGateToday:
		return "traffic.top_gate.today"
	case fpTrafficWeek:
		return "traffic.trend.recent_days"
	case fpTrafficHkMacau:
		return "traffic.ratio.hk_macau"
	case fpTrafficGateRank:
		return "traffic.rank.gate"
	case fpTrafficWeekendCompare:
		return "traffic.compare.weekend_workday"
	case fpTrafficHolidayCompare:
		return "traffic.compare.holiday_workday"
	case fpTrafficForeignOrigin:
		return "traffic.rank.foreign_origin"
	case fpPopWeekTrend:
		return "population.trend.recent_days"
	case fpPopHolidayCompare:
		return "population.compare.holiday"
	case fpPopRegionRank:
		return "population.rank.region"
	case fpGridCaseCount:
		return "grid.case.count"
	case fpGridCloseRate:
		return "grid.case.close_rate"
	case fpGridRegionRank:
		return "grid.rank.region"
	default:
		return "unknown"
	}
}
