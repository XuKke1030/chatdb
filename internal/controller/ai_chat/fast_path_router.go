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
	Hybrid     bool           `json:"hybrid"`
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
	// 参数提取：缺省时间优先继承会话上一轮明确时间，避免追问时回退到默认周期。
	params := ExtractFastPathParamsWithHistory(context.Background(), req.Topic, req.Message, req.History)
	fp.params = params
	hybrid := isHybridFastPath(fp.kind)
	return &FastPathIntent{
		Intent:     fastPathIntentName(fp.kind),
		Topic:      req.Topic,
		Question:   req.Message,
		DatabaseId: req.DatabaseId,
		Cacheable:  !hybrid,
		Hybrid:     hybrid,
		Params:     paramsToMap(params),
		legacy:     fp,
	}, true
}

// isHybridFastPath returns true for fast path kinds that should use LLM to
// generate the natural language answer instead of a fixed template.
func isHybridFastPath(kind fastPathKind) bool {
	switch kind {
	case fpTrafficGateRank, fpTrafficTopGateToday,
		fpTrafficHkMacau, fpTrafficProvinceInside,
		fpTrafficWeek, fpTrafficHoliday, fpTrafficHolidayCompare, fpTrafficWeekendCompare,
		fpTrafficMultiGateCompare, fpTrafficHkMacauYoY, fpTrafficHkMacauStay,
		fpPopWeekTrend, fpPopRegionRank, fpPopHourlyTrend,
		fpPopTagDistribution, fpPopTagTopN, fpPopTagTrend,
		fpPopTagProportion, fpPopMultiTagTrend, fpPopComprehensive, fpPopMultiTagCompare,
		fpPopActivationSummary, fpPopActivationTrend,
		fpPopPortrait, fpPopOverview, fpTrafficOverview,
		fpGridCaseCount, fpGridCloseRate, fpGridRegionRank, fpGridCaseTypeDist, fpGridOverview, fpGridAvgHandle:
		return true
	}
	return false
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
	if p.DateFrom != "" {
		m["dateFrom"] = p.DateFrom
	}
	if p.DateTo != "" {
		m["dateTo"] = p.DateTo
	}
	if p.AllDates {
		m["allDates"] = true
	}
	if p.Tag != "" {
		m["tag"] = p.Tag
	}
	if len(p.Labels) > 0 {
		m["labels"] = strings.Join(p.Labels, ",")
	}
	if p.PopType > 0 {
		m["popType"] = p.PopType
	}
	if p.TopN > 0 {
		m["topN"] = p.TopN
	}
	if p.Area != "" {
		m["area"] = p.Area
	}
	if p.Community != "" {
		m["community"] = p.Community
	}
	if p.GridName != "" {
		m["gridName"] = p.GridName
	}
	if p.CaseType != "" {
		m["caseType"] = p.CaseType
	}
	if p.HolidayName != "" {
		m["holidayName"] = p.HolidayName
	}
	return m
}

func executeFastPathIntent(ctx context.Context, intent *FastPathIntent) (*FastPathResult, error) {
	totalStart := time.Now()
	var cacheKey string
	var ttl time.Duration

	// Check cache first
	if intent.Cacheable {
		days := paramDays(intent.Params)
		cacheKey = BuildCacheKey(intent.Topic, intent.Intent, intent.DatabaseId, days, intent.Params)
		ttl = intentTTL(intent.Intent)
		if cached, hit := GetCache(ctx, cacheKey); hit {
			cached.CacheHit = true
			cached.QueryMs = 0
			cached.FormatMs = 0
			cached.TotalMs = time.Since(totalStart).Milliseconds()
			if cached.Format.Version == "" {
				cached.Format = buildFastPathFormatMeta(ctx, intent.legacy, cached.Conclusion, cached.ChartData)
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
	answer := formatFastPathAnswer(ctx, intent.legacy, conclusion, chartData)
	formatMs := time.Since(formatStart).Milliseconds()
	format := buildFastPathFormatMeta(ctx, intent.legacy, conclusion, chartData)
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
			cacheKey = BuildCacheKey(intent.Topic, intent.Intent, intent.DatabaseId, paramDays(intent.Params), intent.Params)
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
		cacheKey = BuildCacheKey(intent.Topic, intent.Intent, intent.DatabaseId, paramDays(intent.Params), intent.Params)
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
		"chartData":  result.ChartData,
		"allDates":   intent.Params["allDates"] != nil,
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

func buildFastPathFormatMeta(ctx context.Context, fp *fastPath, conclusion string, chartData string) FastPathFormatMeta {
	sections := []string{"精准结论", "特征洞察", "洞察分析"}
	if strings.TrimSpace(chartData) != "" {
		sections = []string{"精准结论", "特征洞察", "可视化", "洞察分析"}
	}
	insightMeta := buildInsightMeta(ctx, fp, conclusion)
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

func buildInsightMeta(ctx context.Context, fp *fastPath, conclusion string) *FastPathInsightMeta {
	keyPoints, impacts, suggestions := buildKindInsight(ctx, fp, conclusion)
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
	case fpPopRegionProportion:
		return "pie"
	case fpTrafficWeekendCompare, fpTrafficHolidayCompare:
		return "bar_compare"
	case fpPopHourlyTrend:
		return "line_trend"
	case fpPopYoY:
		return "bar_compare"
	case fpPopMultiRegionCompare:
		return "line_trend"
	case fpPopFloatingAnomaly:
		return "bar_rank"
	case fpPopTagDistribution:
		return "pie"
	case fpPopTagTopN:
		return "bar_rank"
	case fpPopTagTrend:
		return "line_trend"
	case fpTrafficHkMacau:
		return "pie"
	case fpTrafficInOutRatio:
		return "pie"
	case fpTrafficHkMacauStay:
		return "bar_rank"
	case fpTrafficMultiGateCompare:
		return "line_trend"
	case fpTrafficHkMacauYoY:
		return "bar_compare"
	case fpPopTagProportion:
		return "pie"
	case fpPopMultiTagTrend:
		return "line_trend"
	case fpPopComprehensive:
		return "line_trend,bar_rank,pie"
	case fpPopMultiTagCompare:
		return "pie,bar_rank"
	case fpPopActivationSummary:
		return "metric_card"
	case fpPopActivationTrend:
		return "line_trend"
	case fpPopPortrait:
		return "metric_card,line_trend,pie,bar_rank"
	case fpGridCaseTypeDist:
		return "pie"
	case fpTrafficOverview:
		return "line_trend,bar_rank,pie"
	case fpPopOverview:
		return "line_trend,bar_rank"
	case fpGridOverview:
		return "bar_rank,pie"
	case fpGridAvgHandle:
		return "metric_card"
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
	case fpPopRegionProportion:
		return "population.proportion.region"
	case fpPopHourlyTrend:
		return "population.trend.hourly"
	case fpPopYoY:
		return "population.compare.yoy"
	case fpPopMultiRegionCompare:
		return "population.compare.multi_region"
	case fpPopFloatingAnomaly:
		return "population.floating.anomaly"
	case fpPopTagDistribution:
		return "population.tag.distribution"
	case fpPopTagTopN:
		return "population.tag.topn"
	case fpPopTagTrend:
		return "population.tag.trend"
	case fpPopActivationSummary:
		return "population.activation.summary"
	case fpPopActivationTrend:
		return "population.activation.trend"
	case fpPopPortrait:
		return "population.portrait"
	case fpGridCaseCount:
		return "grid.case.count"
	case fpGridCloseRate:
		return "grid.case.close_rate"
	case fpGridRegionRank:
		return "grid.rank.region"
	case fpGridCaseTypeDist:
		return "grid.case.type_dist"
	case fpTrafficProvinceInside:
		return "traffic.ratio.province_inside"
	case fpTrafficYoY:
		return "traffic.compare.yoy"
	case fpTrafficMoM:
		return "traffic.compare.mom"
	case fpTrafficHoliday:
		return "traffic.holiday"
	case fpTrafficStayDistribution:
		return "traffic.dwell.distribution"
	case fpTrafficHkMacauStay:
		return "traffic.dwell.hk_macau_distribution"
	case fpTrafficOriginByProvince:
		return "traffic.rank.origin_province"
	case fpTrafficInOutRatio:
		return "traffic.ratio.in_out"
	case fpTrafficMultiGateCompare:
		return "traffic.compare.multi_gate"
	case fpTrafficHkMacauYoY:
		return "traffic.compare.hk_macau_yoy"
	case fpTrafficOverview:
		return "traffic.overview"
	case fpPopOverview:
		return "population.overview"
	case fpPopTagProportion:
		return "population.tag.proportion"
	case fpPopMultiTagTrend:
		return "population.tag.multi_trend"
	case fpPopComprehensive:
		return "population.comprehensive"
	case fpPopMultiTagCompare:
		return "population.compare.multi_tag"
	case fpGridOverview:
		return "grid.overview"
	case fpGridAvgHandle:
		return "grid.handle.avg"
	default:
		return "unknown"
	}
}

func paramDays(params map[string]any) int {
	if v, ok := params["days"]; ok {
		switch d := v.(type) {
		case int:
			return d
		case float64:
			return int(d)
		}
	}
	return 7
}
