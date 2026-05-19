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

type FastPathInsightMeta struct {
	KeyPoints   []string `json:"keyPoints"`
	Impacts     []string `json:"impacts"`
	Suggestions []string `json:"suggestions"`
}

type FastPathFormatMeta struct {
	Version                 string               `json:"version"`
	Sections                []string             `json:"sections"`
	ChartRule               string               `json:"chartRule"`
	InsightCollapsedDefault bool                 `json:"insightCollapsedDefault"`
	NaturalLanguageOnly     bool                 `json:"naturalLanguageOnly"`
	InsightMeta             *FastPathInsightMeta `json:"insightMeta,omitempty"`
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
	// 参数提取：context 后续从 controller 层注入，这里用 background 仅做卡口名称匹配
	params := ExtractFastPathParams(context.Background(), req.Topic, req.Message)
	fp.params = params
	return &FastPathIntent{
		Intent:     fastPathIntentName(fp.kind),
		Topic:      req.Topic,
		Question:   req.Message,
		DatabaseId: req.DatabaseId,
		Cacheable:  true,
		Params:     paramsToMap(params),
		legacy:     fp,
	}, true
}

// paramsToMap 将 ExtractedParams 转为 map[string]any 用于缓存 key
func paramsToMap(p ExtractedParams) map[string]any {
	m := map[string]any{}
	if p.GateName != "" {
		m["gateName"] = p.GateName
	}
	if p.RegionName != "" {
		m["regionName"] = p.RegionName
	}
	if p.CompareRef != "" {
		m["compareRef"] = p.CompareRef
	}
	if p.Days > 0 && p.Days != 7 {
		m["days"] = p.Days
	}
	return m
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
				cached.Format = buildFastPathFormatMeta(intent.legacy, cached.Conclusion, cached.ChartData)
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
	format := buildFastPathFormatMeta(intent.legacy, conclusion, chartData)
	if format.InsightMeta != nil {
		consts.Logger.Infof(ctx, "AskNumberInsight intent=%s topic=%s keyPoints=%d impacts=%d suggestions=%d collapsed=%v",
			intent.Intent, intent.Topic,
			len(format.InsightMeta.KeyPoints), len(format.InsightMeta.Impacts), len(format.InsightMeta.Suggestions),
			format.InsightCollapsedDefault)
	}
	result := &FastPathResult{
		FastPath:   true,
		CacheHit:   false,
		Intent:     intent.Intent,
		Topic:      intent.Topic,
		Answer:     answer,
		Conclusion: conclusion,
		ChartData:  chartData,
		Format:     format,
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

func buildFastPathFormatMeta(fp *fastPath, conclusion string, chartData string) FastPathFormatMeta {
	sections := []string{"精准结论", "特征洞察", "洞察分析"}
	if strings.TrimSpace(chartData) != "" {
		sections = []string{"精准结论", "特征洞察", "可视化", "洞察分析"}
	}
	insightMeta := buildInsightMeta(fp, conclusion)
	insightCollapsedDefault := shouldCollapseInsight(insightMeta)
	return FastPathFormatMeta{
		Version:                 "ask-number-fastpath-v1",
		Sections:                sections,
		ChartRule:               fastPathChartRule(fp, chartData),
		InsightCollapsedDefault: insightCollapsedDefault,
		NaturalLanguageOnly:     true,
		InsightMeta:             insightMeta,
	}
}

func shouldCollapseInsight(meta *FastPathInsightMeta) bool {
	if meta == nil {
		return true
	}
	total := len(meta.KeyPoints) + len(meta.Impacts) + len(meta.Suggestions)
	if total <= 3 {
		allShort := true
		for _, s := range meta.KeyPoints {
			if len([]rune(s)) > 30 {
				allShort = false
				break
			}
		}
		if allShort {
			for _, s := range meta.Impacts {
				if len([]rune(s)) > 30 {
					allShort = false
					break
				}
			}
		}
		if allShort {
			for _, s := range meta.Suggestions {
				if len([]rune(s)) > 30 {
					allShort = false
					break
				}
			}
		}
		if allShort {
			return false
		}
	}
	return true
}

func buildInsightMeta(fp *fastPath, conclusion string) *FastPathInsightMeta {
	keyPoints, impacts, suggestions := buildKindInsight(fp, conclusion)
	if len(keyPoints) == 0 && len(impacts) == 0 && len(suggestions) == 0 {
		return nil
	}
	return &FastPathInsightMeta{
		KeyPoints:   keyPoints,
		Impacts:     impacts,
		Suggestions: suggestions,
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
	case fpTrafficTopGateToday, fpTrafficGateRank, fpTrafficForeignOrigin, fpPopRegionRank, fpGridRegionRank, fpTrafficDwellTop:
		return "bar_rank"
	case fpTrafficWeekendCompare, fpTrafficHolidayCompare:
		return "bar_compare"
	case fpPopHourlyTrend:
		return "line_trend"
	case fpGridCaseTypeDist:
		return "pie"
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
	case fpTrafficDwellTop:
		return "traffic.dwell.top"
	case fpPopWeekTrend:
		return "population.trend.recent_days"
	case fpPopHolidayCompare:
		return "population.compare.holiday"
	case fpPopRegionRank:
		return "population.rank.region"
	case fpPopHourlyTrend:
		return "population.trend.hourly"
	case fpGridCaseCount:
		return "grid.case.count"
	case fpGridCloseRate:
		return "grid.case.close_rate"
	case fpGridRegionRank:
		return "grid.rank.region"
	case fpGridCaseTypeDist:
		return "grid.case.type_dist"
	default:
		return "unknown"
	}
}
