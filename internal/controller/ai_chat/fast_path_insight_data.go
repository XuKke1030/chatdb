package ai_chat

import (
	"regexp"
	"strconv"
	"strings"
)

// extractInsightData 从结论文本和快问类型中提取关键数值，返回占位符→值映射
func extractInsightData(kind fastPathKind, conclusion string) map[string]string {
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
		data["level"] = trafficLevel(nums)
		data["peak"] = "高峰"
		data["pressure"] = "较大"
		data["action"] = "现场疏导"
		if len(nums) > 0 {
			total, _ := strconv.Atoi(nums[0])
			if total < trafficHighThreshold {
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
		data["hkCount"] = firstOr(nums, "0")
		if r, ok := data["hkRatio"]; ok {
			pct, _ := strconv.ParseFloat(r, 64)
			if pct > hkMacauHighRatioPct {
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
			if outside > provinceOutsideHighPct {
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
		} else if pctVal >= trafficYoYAlertPct {
			data["yoyLevel"] = "显著"
			data["needPlan"] = "需"
		} else {
			data["yoyLevel"] = "一般"
			data["needPlan"] = "视情况"
		}

	case fpTrafficMoM:
		data["momPct"] = extractPct(conclusion)
		pctVal, _ := strconv.ParseFloat(data["momPct"], 64)
		if pctVal >= trafficMoMAlertPct {
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
			if cr < closeRateTarget*0.5 {
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
		data["target"] = formatFloat(closeRateTarget)
		cr, _ := strconv.ParseFloat(data["closeRate"], 64)
		if cr >= closeRateTarget {
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

func extractQuotedName(s string) string {
	re := regexp.MustCompile(`「([^」]+)」`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
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

func trafficLevel(nums []string) string {
	if len(nums) > 0 {
		total, err := strconv.Atoi(nums[0])
		if err == nil && total >= trafficHighThreshold {
			return "高位"
		}
	}
	return "正常"
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
