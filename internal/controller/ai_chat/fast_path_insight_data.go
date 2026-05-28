package ai_chat

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"ai-chat-sql/internal/service"
)

// extractInsightData 从结论文本和快问类型中提取关键数值，返回占位符→值映射
func extractInsightData(ctx context.Context, kind fastPathKind, conclusion string) map[string]string {
	data := map[string]string{}

	if conclusion == "" {
		return data
	}

	nums := extractNumbers(conclusion)

	dir := "持平"
	if strings.Contains(conclusion, "增长") || strings.Contains(conclusion, "上升") {
		dir = "增长"
	} else if strings.Contains(conclusion, "下降") || strings.Contains(conclusion, "减少") {
		dir = "下降"
	}
	data["dir"] = dir

	// 通用占比提取
	ratioRe := regexp.MustCompile(`占比\s*([\d.]+)%`)
	if m := ratioRe.FindStringSubmatch(conclusion); len(m) > 1 {
		data["hkRatio"] = m[1]
	}

	switch kind {
	case fpTrafficToday:
		data["total"] = firstOr(nums, "0")
		data["level"] = trafficLevel(ctx, nums)
		data["peak"] = "高峰"
		data["pressure"] = "较大"
		data["action"] = "现场疏导"
		if len(nums) > 0 {
			total, _ := strconv.Atoi(nums[0])
			if total < int(metricThreshold(ctx, "traffic", "traffic_daily_total", float64(trafficHighThreshold))) {
				data["level"] = "正常"
				data["pressure"] = "一般"
				data["peak"] = "常规"
			}
		}

	case fpTrafficTopGateToday, fpTrafficGateRank:
		data["topGate"] = extractQuotedName(conclusion)
		data["topCount"] = firstOr(nums, "0")

	case fpTrafficWeek:
		data["days"] = "7"
		data["dailyAvg"] = firstOr(nums, "0")

	case fpTrafficHkMacau:
		// nums: [total, hkCount, hkPct, mainlandCount, mainlandPct, ...]
		if len(nums) >= 2 {
			data["hkCount"] = nums[1]
		} else {
			data["hkCount"] = firstOr(nums, "0")
		}
		if r, ok := data["hkRatio"]; ok {
			pct, _ := strconv.ParseFloat(r, 64)
			if pct > metricThreshold(ctx, "traffic", "hk_macau_ratio_pct", hkMacauHighRatioPct) {
				data["hkLevel"] = "偏高"
			} else {
				data["hkLevel"] = "正常"
			}
		} else {
			data["hkLevel"] = "正常"
		}

	case fpTrafficWeekendCompare, fpTrafficHolidayCompare:
		data["diffPct"] = extractPct(conclusion)

	case fpTrafficProvinceInside:
		data["insideCount"] = firstOr(nums, "0")
		data["outsideCount"] = nthOr(nums, 2, "0")
		insideRe := regexp.MustCompile(`占大陆车\s*([\d.]+)%`)
		if m := insideRe.FindStringSubmatch(conclusion); len(m) > 1 {
			data["insidePct"] = m[1]
			pct, _ := strconv.ParseFloat(m[1], 64)
			outside := 100.0 - pct
			data["outsideLevel"] = "正常"
			if outside > metricThreshold(ctx, "traffic", "province_outside_pct", provinceOutsideHighPct) {
				data["outsideLevel"] = "偏高"
			}
		} else if r, ok := data["hkRatio"]; ok {
			data["insidePct"] = r
		}

	case fpTrafficYoY:
		data["yoyPct"] = extractPct(conclusion)
		pctVal, _ := strconv.ParseFloat(data["yoyPct"], 64)
		if pctVal >= 0 && pctVal < 1 {
			data["yoyLevel"] = "微小"
			data["needPlan"] = "暂无需"
		} else if pctVal >= metricThreshold(ctx, "traffic", "yoy_change_pct", trafficYoYAlertPct) {
			data["yoyLevel"] = "显著"
			data["needPlan"] = "需"
		} else {
			data["yoyLevel"] = "一般"
			data["needPlan"] = "视情况"
		}

	case fpTrafficMoM:
		data["momPct"] = extractPct(conclusion)
		pctVal, _ := strconv.ParseFloat(data["momPct"], 64)
		if pctVal >= metricThreshold(ctx, "traffic", "mom_change_pct", trafficMoMAlertPct) {
			data["momLevel"] = "显著"
		} else {
			data["momLevel"] = "一般"
		}

	case fpTrafficHoliday:
		data["holidayName"] = extractHolidayName(conclusion)
		data["total"] = firstOr(nums, "0")
		data["dailyAvg"] = nthOr(nums, 1, firstOr(nums, "0"))

	case fpTrafficStayDistribution:
		data["topBucket"] = extractStayBucket(conclusion)
		data["totalVehicles"] = firstOr(nums, "0")

	case fpTrafficOriginByProvince:
		data["topOrigin"] = extractQuotedName(conclusion)
		data["topCount"] = firstOr(nums, "0")

	case fpTrafficOverview:
		data["total"] = firstOr(nums, "0")
		data["dailyAvg"] = nthOr(nums, 1, firstOr(nums, "0"))
		insideRe := regexp.MustCompile(`占大陆车\s*([\d.]+)%`)
		if m := insideRe.FindStringSubmatch(conclusion); len(m) > 1 {
			data["insidePct"] = m[1]
		}

	case fpPopWeekTrend:
		data["days"] = "7"
		data["total"] = firstOr(nums, "0")
		data["dailyAvg"] = nthOr(nums, 1, firstOr(nums, "0"))

	case fpPopRegionRank:
		data["topRegion"] = extractQuotedName(conclusion)
		data["topCount"] = firstOr(nums, "0")

	case fpPopHourlyTrend:
		data["peakHour"] = extractHourRange(conclusion)
		data["peakCount"] = firstOr(nums, "0")

	case fpPopOverview:
		data["total"] = firstOr(nums, "0")
		data["dailyAvg"] = nthOr(nums, 1, firstOr(nums, "0"))

	case fpGridCaseCount:
		data["total"] = firstOr(nums, "0")
		data["closeRate"] = extractPct(conclusion)
		if cr, err := strconv.ParseFloat(data["closeRate"], 64); err == nil {
			if cr < metricThreshold(ctx, "grid", "close_rate_pct", closeRateTarget)*0.5 {
				data["caseLevel"] = "高"
				data["pressureLevel"] = "较大"
			} else {
				data["caseLevel"] = "正常"
				data["pressureLevel"] = "可控"
			}
		} else {
			data["caseLevel"] = "正常"
			data["pressureLevel"] = "可控"
		}

	case fpGridCloseRate:
		data["closeRate"] = extractPct(conclusion)
		data["target"] = formatFloat(metricThreshold(ctx, "grid", "close_rate_pct", closeRateTarget))
		cr, _ := strconv.ParseFloat(data["closeRate"], 64)
		if cr >= metricThreshold(ctx, "grid", "close_rate_pct", closeRateTarget) {
			data["comparedTo"] = "达到"
		} else {
			data["comparedTo"] = "低于"
		}

	case fpGridRegionRank:
		data["topRegion"] = extractQuotedName(conclusion)
		data["topCount"] = firstOr(nums, "0")

	case fpGridCaseTypeDist:
		data["topType"] = extractQuotedName(conclusion)
		data["topPct"] = extractPct(conclusion)

	case fpGridAvgHandle:
		data["top_type"] = extractQuotedName(conclusion)
		data["avg_hours"] = firstOr(nums, "0")
		// Extract last quoted name for bottom_type
		quoteRe := regexp.MustCompile(`「([^」]+)」`)
		allQuotes := quoteRe.FindAllStringSubmatch(conclusion, -1)
		if len(allQuotes) > 1 {
			data["bottom_type"] = allQuotes[len(allQuotes)-1][1]
		}
		if len(nums) > 1 {
			data["bottom_hours"] = nums[len(nums)-1]
		}

	case fpTrafficDwellTop:
		data["top_plate"] = extractQuotedName(conclusion)
		dwellRe := regexp.MustCompile(`(\d+)\s*小时`)
		if m := dwellRe.FindStringSubmatch(conclusion); len(m) > 1 {
			data["dwell_hours"] = m[1]
		} else {
			data["dwell_hours"] = firstOr(nums, "0")
		}

	case fpTrafficForeignOrigin:
		data["top_province"] = extractQuotedName(conclusion)
		data["top_count"] = firstOr(nums, "0")

	case fpTrafficHkMacauStay:
		data["bucket"] = extractQuotedName(conclusion)
		data["avgStay"] = firstOr(nums, "0")

	case fpPopHolidayCompare:
		data["holiday_avg"] = firstOr(nums, "0")
		data["workday_avg"] = nthOr(nums, 1, firstOr(nums, "0"))
		data["change_pct"] = extractPct(conclusion)
		data["direction"] = dir

	case fpPopYoY:
		data["current_val"] = firstOr(nums, "0")
		data["prior_val"] = nthOr(nums, 1, firstOr(nums, "0"))
		data["change_pct"] = extractPct(conclusion)
		data["direction"] = dir

	case fpPopMultiRegionCompare:
		data["top_region"] = extractQuotedName(conclusion)
		data["top_val"] = firstOr(nums, "0")
		if len(nums) >= 2 {
			v1, _ := strconv.Atoi(nums[0])
			v2, _ := strconv.Atoi(nums[len(nums)-1])
			diff := v1 - v2
			if diff < 0 {
				diff = -diff
			}
			data["max_diff"] = fmt.Sprintf("%d", diff)
		} else {
			data["max_diff"] = "—"
		}

	case fpPopTagDistribution:
		data["tag"] = "年龄"
		if strings.Contains(conclusion, "性别") {
			data["tag"] = "性别"
		} else if strings.Contains(conclusion, "来源") {
			data["tag"] = "来源地"
		}
		data["top_label"] = extractQuotedName(conclusion)
		data["top_pct"] = extractPct(conclusion)
		data["change_dir"] = "持平"
		if strings.Contains(conclusion, "增长") || strings.Contains(conclusion, "上升") {
			data["change_dir"] = "上升"
		} else if strings.Contains(conclusion, "下降") || strings.Contains(conclusion, "减少") {
			data["change_dir"] = "下降"
		}

	case fpPopTagTopN:
		data["tag"] = "省外来源"
		if strings.Contains(conclusion, "年龄") {
			data["tag"] = "年龄"
		} else if strings.Contains(conclusion, "性别") {
			data["tag"] = "性别"
		} else if strings.Contains(conclusion, "来源") {
			data["tag"] = "来源地"
		}
		data["top_label"] = extractQuotedName(conclusion)
		data["top_count"] = firstOr(nums, "0")
		data["top_pct"] = extractPct(conclusion)
		// Top3合计占比：取所有百分比数值，前三相加
		allPcts := extractAllPcts(conclusion)
		top3Sum := 0.0
		for i := 0; i < 3 && i < len(allPcts); i++ {
			v, _ := strconv.ParseFloat(allPcts[i], 64)
			top3Sum += v
		}
		if top3Sum > 0 {
			data["top3_pct"] = formatFloat(top3Sum)
		}

	case fpPopTagTrend:
		data["top_label"] = extractQuotedName(conclusion)
		data["trend_dir"] = "平稳"
		if strings.Contains(conclusion, "上升") {
			data["trend_dir"] = "上升"
		} else if strings.Contains(conclusion, "下降") {
			data["trend_dir"] = "下降"
		}
		data["change_pct"] = extractPct(conclusion)

	case fpPopActivationSummary:
		data["activation"] = firstOr(nums, "0")
		data["deviation_desc"] = "与基线基本持平"
		data["is_anomaly"] = "活力处于正常范围"
		data["deviation_pct"] = "0"
		devRe := regexp.MustCompile(`偏离([+-]?[\d.]+)%`)
		if m := devRe.FindStringSubmatch(conclusion); len(m) > 1 {
			data["deviation_pct"] = m[1]
		}
		if strings.Contains(conclusion, "显著高于") {
			data["deviation_desc"] = "显著高于基线"
			data["is_anomaly"] = "活力异常偏高，需关注"
		} else if strings.Contains(conclusion, "低于") {
			data["deviation_desc"] = "低于基线"
			data["is_anomaly"] = "活力偏低，需排查原因"
		}

	case fpPopActivationTrend:
		data["trend_dir"] = "平稳"
		if strings.Contains(conclusion, "上升") {
			data["trend_dir"] = "上升"
		} else if strings.Contains(conclusion, "下降") {
			data["trend_dir"] = "下降"
		}
		data["activation_latest"] = firstOr(nums, "0")
		data["baseline_latest"] = firstOr(nums[1:], "0")
		// Extract crossover point from conclusion text
		crossRe := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})(上穿基线|下穿基线)`)
		if m := crossRe.FindStringSubmatch(conclusion); len(m) > 2 {
			data["crossover_point"] = m[1] + m[2]
		} else {
			data["crossover_point"] = "近期无基线交叉"
		}

	case fpPopPortrait:
		data["all_count"] = firstOr(nums, "0")
		data["activation"] = firstOr(nums[1:], "0")
		data["deviation_desc"] = "与基线基本持平"
		if strings.Contains(conclusion, "显著高于") {
			data["deviation_desc"] = "显著高于基线"
		} else if strings.Contains(conclusion, "低于") {
			data["deviation_desc"] = "低于基线"
		}
		data["age_top_label"] = extractQuotedName(conclusion)
		data["age_top_pct"] = extractPct(conclusion)
		// Extract second quoted name for origin top
		rest := conclusion
		if idx := strings.Index(conclusion, "占比"); idx > 0 {
			rest = conclusion[idx+6:]
		}
		data["origin_top_label"] = extractQuotedName(rest)
		data["origin_top_pct"] = extractPct(rest)

	case fpPopFloatingAnomaly:
		data["anomaly_count"] = firstOr(nums, "0")
		data["top_region"] = extractQuotedName(conclusion)
		data["top_pct"] = extractPct(conclusion)

	case fpTrafficInOutRatio:
		data["inCount"] = firstOr(nums, "0")
		data["outCount"] = nthOr(nums, 1, "0")
		inVal, _ := strconv.Atoi(data["inCount"])
		outVal, _ := strconv.Atoi(data["outCount"])
		total := inVal + outVal
		if total > 0 {
			data["inPct"] = fmt.Sprintf("%.1f", float64(inVal)/float64(total)*100)
			data["outPct"] = fmt.Sprintf("%.1f", float64(outVal)/float64(total)*100)
		} else {
			data["inPct"] = "0"
			data["outPct"] = "0"
		}

	case fpTrafficMultiGateCompare:
		data["top3Pct"] = extractPct(conclusion)

	case fpTrafficHkMacauYoY:
		data["yoyPct"] = extractPct(conclusion)
		if pctVal, err := strconv.ParseFloat(data["yoyPct"], 64); err == nil {
			if pctVal >= metricThreshold(ctx, "traffic", "hk_macau_yoy_pct", 20.0) {
				data["yoyLevel"] = "显著"
			} else if pctVal < 1 {
				data["yoyLevel"] = "微小"
			} else {
				data["yoyLevel"] = "一般"
			}
		} else {
			data["yoyLevel"] = "一般"
		}

	case fpPopTagProportion:
		data["tag"] = "年龄"
		if strings.Contains(conclusion, "性别") {
			data["tag"] = "性别"
		} else if strings.Contains(conclusion, "来源") {
			data["tag"] = "来源地"
		}
		data["topLabel"] = extractQuotedName(conclusion)
		data["topPct"] = extractPct(conclusion)

	case fpPopMultiTagTrend:
		data["topLabel"] = extractQuotedName(conclusion)
		data["trendDir"] = "平稳"
		if strings.Contains(conclusion, "上升") || strings.Contains(conclusion, "增长") {
			data["trendDir"] = "上升"
		} else if strings.Contains(conclusion, "下降") || strings.Contains(conclusion, "减少") {
			data["trendDir"] = "下降"
		}
		data["changePct"] = extractPct(conclusion)

	case fpPopComprehensive:
		data["total"] = firstOr(nums, "0")
		data["topRegion"] = extractQuotedName(conclusion)
		data["ageTopLabel"] = extractQuotedName(conclusion)
		data["ageTopPct"] = extractPct(conclusion)
		rest := conclusion
		if idx := strings.Index(conclusion, "占比"); idx > 0 {
			rest = conclusion[idx+6:]
		}
		data["originTopLabel"] = extractQuotedName(rest)
		data["originTopPct"] = extractPct(rest)

	case fpPopMultiTagCompare:
		data["ageTopLabel"] = extractQuotedName(conclusion)
		data["ageTopPct"] = extractPct(conclusion)
		maleRe := regexp.MustCompile(`男\s*([\d.]+)%`)
		if m := maleRe.FindStringSubmatch(conclusion); len(m) > 1 {
			data["malePct"] = m[1]
		} else {
			data["malePct"] = "0"
		}
		femaleRe := regexp.MustCompile(`女\s*([\d.]+)%`)
		if m := femaleRe.FindStringSubmatch(conclusion); len(m) > 1 {
			data["femalePct"] = m[1]
		} else {
			data["femalePct"] = "0"
		}
		rest := conclusion
		if idx := strings.Index(conclusion, "省外"); idx > 0 {
			rest = conclusion[idx:]
		}
		data["originTopLabel"] = extractQuotedName(rest)
		data["originTopPct"] = extractPct(rest)

	case fpGridOverview:
		data["total_cases"] = firstOr(nums, "0")
		data["close_rate"] = extractPct(conclusion)
		if cr, err := strconv.ParseFloat(data["close_rate"], 64); err == nil {
			if cr < metricThreshold(ctx, "grid", "close_rate_pct", closeRateTarget) {
				data["close_rate_direction"] = "偏低"
				data["efficiency_direction"] = "待提升"
			} else {
				data["close_rate_direction"] = "达标"
				data["efficiency_direction"] = "良好"
			}
		} else {
			data["close_rate_direction"] = "—"
			data["efficiency_direction"] = "—"
		}
	}

	return data
}

func extractNumbers(s string) []string {
	re := regexp.MustCompile(`\b(\d+)\b`)
	matches := re.FindAllStringSubmatch(s, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			result = append(result, m[1])
		}
	}
	return result
}

func extractPct(s string) string {
	re := regexp.MustCompile(`([\d.]+)%`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return "0"
}

func extractAllPcts(s string) []string {
	re := regexp.MustCompile(`([\d.]+)%`)
	matches := re.FindAllStringSubmatch(s, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			result = append(result, m[1])
		}
	}
	return result
}

func extractQuotedName(s string) string {
	re := regexp.MustCompile(`「([^」]+)」`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) > 1 {
		return matches[len(matches)-1][1]
	}
	if len(matches) > 0 {
		return matches[0][1]
	}
	return "—"
}

func extractHolidayName(s string) string {
	for _, name := range []string{"国庆", "春节", "清明", "劳动节", "中秋", "端午", "元旦"} {
		if strings.Contains(s, name) {
			return name
		}
	}
	return "节假日"
}

func extractStayBucket(s string) string {
	re := regexp.MustCompile(`([\d]+-[\d]+(?:min|h|分钟|小时)[\w]*|[\d]+h\+|4h\+)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return "0-30min"
}

func extractHourRange(s string) string {
	re := regexp.MustCompile(`(\d{1,2}:\d{2}|\d{1,2}点)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return "高峰"
}

func firstOr(nums []string, fallback string) string {
	if len(nums) > 0 {
		return nums[0]
	}
	return fallback
}

func nthOr(nums []string, n int, fallback string) string {
	if len(nums) > n {
		return nums[n]
	}
	return fallback
}

func metricThreshold(ctx context.Context, topic, metricName string, fallback float64) float64 {
	if service.Metric() != nil {
		v, _, found := service.Metric().GetThreshold(ctx, topic, metricName)
		if found {
			return v
		}
	}
	return fallback
}

func trafficLevel(ctx context.Context, nums []string) string {
	if len(nums) > 0 {
		total, err := strconv.Atoi(nums[0])
		if err == nil && total >= int(metricThreshold(ctx, "traffic", "traffic_daily_total", float64(trafficHighThreshold))) {
			return "高位"
		}
	}
	return "正常"
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
