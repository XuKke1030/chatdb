package ai_chat

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ExtractedParams 从用户消息中提取的结构化参数
type ExtractedParams struct {
	DateFrom     string  // 起始日期 Y-m-d
	DateTo       string  // 截止日期 Y-m-d
	GateName     string  // 卡口名称（如"拱北口岸"）
	RegionName   string  // 区域名称（如"香洲区"）
	Days         int     // 天数（近N天）
	CompareRef   string  // 对比基准：weekend/holiday/lastweek/yoy/mom
	HolidayName  string  // 节假日名称（如"国庆"、"春节"）
}

// dateRangeParams 尝试从问题中提取时间范围，返回 (from, to, days, ok)
func dateRangeParams(q string, now *gtime.Time) (from, to string, days int, ok bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}

	// 今天
	if containsAny(q, []string{"今天", "今日", "当前", "目前"}) {
		return now.Format("Y-m-d"), now.Format("Y-m-d") + " 23:59:59", 1, true
	}
	// 昨天
	if containsAny(q, []string{"昨天", "昨日"}) {
		y := now.AddDate(0, 0, -1)
		return y.Format("Y-m-d"), y.Format("Y-m-d") + " 23:59:59", 1, true
	}
	// 近N天 / 最近N天
	re := regexp.MustCompile(`(?:近|最近)(\d+)[天日]`)
	if m := re.FindStringSubmatch(q); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= 365 {
			from := now.AddDate(0, 0, -n+1).Format("Y-m-d")
			return from, now.Format("Y-m-d") + " 23:59:59", n, true
		}
	}
	// 本周
	if containsAny(q, []string{"本周", "这周", "这一周"}) {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -weekday+1)
		return monday.Format("Y-m-d"), now.Format("Y-m-d") + " 23:59:59", weekday, true
	}
	// 上周
	if containsAny(q, []string{"上周", "上一周"}) {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		lastMonday := now.AddDate(0, 0, -weekday-6)
		lastSunday := now.AddDate(0, 0, -weekday)
		return lastMonday.Format("Y-m-d"), lastSunday.Format("Y-m-d") + " 23:59:59", 7, true
	}
	// 本月
	if containsAny(q, []string{"本月", "这个月", "当月"}) {
		firstOfMonth := now.Format("Y-m") + "-01"
		return firstOfMonth, now.Format("Y-m-d") + " 23:59:59", now.Day(), true
	}
	// 默认7天
	if containsAny(q, []string{"近七", "近7", "最近七天", "最近7天", "一周", "7天", "近期", "近几天", "最近几天"}) {
		from := now.AddDate(0, 0, -6).Format("Y-m-d")
		return from, now.Format("Y-m-d") + " 23:59:59", 7, true
	}

	return
}

// gateNameParams 尝试从问题中匹配卡口名称
func gateNameParams(ctx context.Context, q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	devices := loadGateDeviceNames(ctx)
	if len(devices) == 0 {
		return ""
	}
	// 最长匹配优先
	best := ""
	for _, name := range devices {
		if name != "" && strings.Contains(q, name) && len(name) > len(best) {
			best = name
		}
	}
	return best
}

// regionNameParams 尝试从问题中匹配区域名称
func regionNameParams(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	// 常见珠海行政区域
	zhuhaiRegions := []string{"香洲区", "斗门区", "金湾区", "横琴新区", "横琴粤澳深度合作区", "高新区", "万山区", "保税区", "高栏港经济区", "富山工业区"}
	best := ""
	for _, r := range zhuhaiRegions {
		if strings.Contains(q, r) && len(r) > len(best) {
			best = r
		}
	}
	return best
}

// compareRefParams 从问题中提取对比基准
func compareRefParams(q string) string {
	q = strings.TrimSpace(q)
	if containsAny(q, []string{"节假日", "节日", "假日", "假期", "法定假"}) {
		return "holiday"
	}
	if containsAny(q, []string{"周末", "周六周日", "周末和平日", "周末和工作日"}) {
		return "weekend"
	}
	if containsAny(q, []string{"同比", "去年同期", "去年", "比去年同期"}) {
		return "yoy"
	}
	if containsAny(q, []string{"环比", "上月", "比上月", "比上月同期"}) {
		return "mom"
	}
	if containsAny(q, []string{"上周", "上一周", "环比", "上周同期", "对比上周"}) {
		return "lastweek"
	}
	return ""
}

// ExtractFastPathParams 综合提取所有参数
func ExtractFastPathParams(ctx context.Context, topic, question string) ExtractedParams {
	now := gtime.Now()
	var p ExtractedParams

	// 时间范围
	from, to, days, ok := dateRangeParams(question, now)
	if ok {
		p.DateFrom = from
		p.DateTo = to
		p.Days = days
	} else {
		// 默认7天
		p.DateFrom = now.AddDate(0, 0, -6).Format("Y-m-d")
		p.DateTo = now.Format("Y-m-d") + " 23:59:59"
		p.Days = 7
	}

	// 卡口名称（仅车流主题）
	if topic == "traffic" {
		p.GateName = gateNameParams(ctx, question)
	}

	// 区域名称
	p.RegionName = regionNameParams(question)

	// 对比基准
	p.CompareRef = compareRefParams(question)

	// 节假日名称提取
	p.HolidayName = holidayNameParams(question)

	return p
}

// holidayNameParams 从问题中提取节假日名称
func holidayNameParams(q string) string {
	q = strings.TrimSpace(q)
	pairs := []struct {
		key   string
		value string
	}{
		{"国庆", "国庆"}, {"春节", "春节"}, {"元旦", "元旦"}, {"清明", "清明"},
		{"劳动节", "劳动"}, {"五一", "劳动"}, {"端午", "端午"}, {"中秋", "中秋"},
	}
	for _, p := range pairs {
		if strings.Contains(q, p.key) {
			return p.value
		}
	}
	return ""
}

// ---- 卡口名称缓存 ----

var (
	gateDeviceNamesCache []string
	gateDeviceNamesOnce  sync.Once
	gateDeviceNamesMu    sync.RWMutex
)

func loadGateDeviceNames(ctx context.Context) []string {
	defer func() {
		if r := recover(); r != nil {
			// 测试环境下可能没有数据库驱动，忽略 panic
		}
	}()
	gateDeviceNamesOnce.Do(func() {
		refreshGateDeviceNames(ctx)
	})
	gateDeviceNamesMu.RLock()
	defer gateDeviceNamesMu.RUnlock()
	return gateDeviceNamesCache
}

func refreshGateDeviceNames(ctx context.Context) {
	records, err := g.DB("master").Model("traffic_gate_device").Ctx(ctx).
		Fields("device_name").
		Where("enabled = ?", 1).
		Where("device_name <> ''").
		All()
	if err != nil {
		return
	}
	names := make([]string, 0, len(records))
	for _, r := range records {
		name := strings.TrimSpace(r["device_name"].String())
		if name != "" {
			names = append(names, name)
		}
	}
	gateDeviceNamesMu.Lock()
	gateDeviceNamesCache = names
	gateDeviceNamesMu.Unlock()
}

// RefreshGateDeviceNamesCache 可在设备变更时外部调用刷新
func RefreshGateDeviceNamesCache(ctx context.Context) {
	refreshGateDeviceNames(ctx)
	// 重置 once 以允许下次重新加载
	gateDeviceNamesOnce = sync.Once{}
}

// ---- 工具函数 ----
