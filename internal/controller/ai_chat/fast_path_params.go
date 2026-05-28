package ai_chat

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"ai-chat-sql/internal/model"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ExtractedParams 从用户消息中提取的结构化参数
type ExtractedParams struct {
	DateFrom    string   // 起始日期 Y-m-d
	DateTo      string   // 截止日期 Y-m-d
	GateName    string   // 卡口名称（如"拱北口岸"）
	RegionName  string   // 区域名称（如"香洲区"）
	Days        int      // 天数（近N天）
	CompareRef  string   // 对比基准：weekend/holiday/lastweek/yoy/mom
	HolidayName string   // 节假日名称（如"国庆"、"春节"）
	AllDates    bool     // 是否查询当前可用全部日期范围
	Question    string   // 当前用户问题
	Tag         string   // 人流标签类别：年龄/性别/省内城市来源/省外城市来源/省外来源
	Labels      []string // 具体标签值，如 ["(31,35]","(36,40]"]
	PopType     int      // 人流方向：0=不区分, 1=总, 2=进, 3=出
	TopN        int      // 排名数，默认5
	Area        string   // 区域筛选
	Community   string   // 社区名称（网格主题）
	GridName    string   // 网格名称（网格主题）
	CaseType    string   // 案件类型（网格主题）
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
	// 前天
		if containsAny(q, []string{"前天"}) {
			d := now.AddDate(0, 0, -2)
			return d.Format("Y-m-d"), d.Format("Y-m-d") + " 23:59:59", 1, true
		}
		// 大前天
		if containsAny(q, []string{"大前天"}) {
			d := now.AddDate(0, 0, -3)
			return d.Format("Y-m-d"), d.Format("Y-m-d") + " 23:59:59", 1, true
		}
		// N天前
		reDaysAgo := regexp.MustCompile(`(\d+)[天日]前`)
		if m := reDaysAgo.FindStringSubmatch(q); len(m) == 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= 365 {
				d := now.AddDate(0, 0, -n)
				return d.Format("Y-m-d"), d.Format("Y-m-d") + " 23:59:59", 1, true
			}
		}
	// 近N天 / 最近N天
	re := regexp.MustCompile(`(?:近|最近)(\d+)[天日]`)
	if m := re.FindStringSubmatch(q); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= 365 {
			from := now.AddDate(0, 0, -n+1).Format("Y-m-d")
			return from, now.Format("Y-m-d") + " 23:59:59", n, true
		}
	}
	// 一个月内 / 近一个月
	if containsAny(q, []string{"一个月内", "近一个月", "最近一个月", "近30天", "最近30天", "30天内"}) {
		from := now.AddDate(0, 0, -29).Format("Y-m-d")
		return from, now.Format("Y-m-d") + " 23:59:59", 30, true
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
	// 上个月
		if containsAny(q, []string{"上个月", "上月"}) {
			prevMonth := now.AddDate(0, -1, 0)
			firstOfPrevMonth := prevMonth.Format("Y-m") + "-01"
			lastOfPrevMonth := now.Format("Y-m") + "-01"
			return firstOfPrevMonth, lastOfPrevMonth, 30, true
		}
	// 某年 / N年（如"24年"、"2024年"、"今年"、"去年"）
	if containsAny(q, []string{"今年", "本年"}) {
		year := now.Format("2006")
		return year + "-01-01", now.Format("2006-01-02") + " 23:59:59", 365, true
	}
	if containsAny(q, []string{"去年", "上一年"}) {
		year := now.AddDate(-1, 0, 0).Format("2006")
		return year + "-01-01", year + "-12-31 23:59:59", 365, true
	}
	reYear := regexp.MustCompile(`(\d{4})年`)
	if m := reYear.FindStringSubmatch(q); len(m) == 2 {
		y := m[1]
		return y + "-01-01", y + "-12-31 23:59:59", 365, true
	}
	reYearShort := regexp.MustCompile(`(\d{2})年`)
	if m := reYearShort.FindStringSubmatch(q); len(m) == 2 {
		shortY, _ := strconv.Atoi(m[1])
		fullY := 2000 + shortY
		if fullY >= 2020 && fullY <= 2099 {
			y := strconv.Itoa(fullY)
			return y + "-01-01", y + "-12-31 23:59:59", 365, true
		}
	}
	// 默认7天
	if containsAny(q, []string{"近七", "近7", "最近七天", "最近7天", "一周", "7天", "近期", "近几天", "最近几天"}) {
		from := now.AddDate(0, 0, -6).Format("Y-m-d")
		return from, now.Format("Y-m-d") + " 23:59:59", 7, true
	}

	return
}

func allDatesParams(q string) bool {
	q = strings.TrimSpace(q)
	return containsAny(q, []string{"过去所有日期", "所有日期", "全部日期", "历史全部", "全量日期", "全部时间", "所有时间", "所有年份", "历史所有", "全部历史", "所有日期中"})
}

func explicitDateParams(q string, now *gtime.Time) (ExtractedParams, bool) {
	var p ExtractedParams
	if allDatesParams(q) {
		p.AllDates = true
		return p, true
	}
	if from, to, days, ok := dateRangeParams(q, now); ok {
		p.DateFrom = from
		p.DateTo = to
		p.Days = days
		return p, true
	}
	return p, false
}

func inheritDateParamsFromHistory(current string, history []model.ChatHistoryItem, now *gtime.Time) (ExtractedParams, bool) {
	if _, ok := explicitDateParams(current, now); ok {
		return ExtractedParams{}, false
	}
	for i := len(history) - 1; i >= 0; i-- {
		content := strings.TrimSpace(history[i].Content)
		if content == "" {
			continue
		}
		if p, ok := explicitDateParams(content, now); ok {
			return p, true
		}
		if strings.Contains(content, "当前可用全部日期范围") {
			return ExtractedParams{AllDates: true}, true
		}
	}
	return ExtractedParams{}, false
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
	if containsAny(q, []string{"上周", "上一周", "上周同期", "对比上周"}) {
		return "lastweek"
	}
	if containsAny(q, []string{"环比", "上月", "比上月", "比上月同期"}) {
		return "mom"
	}
	return ""
}

// ExtractFastPathParams 综合提取所有参数
func ExtractFastPathParams(ctx context.Context, topic, question string) ExtractedParams {
	return ExtractFastPathParamsWithHistory(ctx, topic, question, nil)
}

func ExtractFastPathParamsWithHistory(ctx context.Context, topic, question string, history []model.ChatHistoryItem) ExtractedParams {
	now := gtime.Now()
	p := ExtractedParams{Question: strings.TrimSpace(question)}

	// 时间范围
	if inherited, ok := inheritDateParamsFromHistory(question, history, now); ok {
		p.DateFrom = inherited.DateFrom
		p.DateTo = inherited.DateTo
		p.Days = inherited.Days
		p.AllDates = inherited.AllDates
	} else if explicit, ok := explicitDateParams(question, now); ok {
		p.DateFrom = explicit.DateFrom
		p.DateTo = explicit.DateTo
		p.Days = explicit.Days
		p.AllDates = explicit.AllDates
	} else if from, to, days, ok := dateRangeParams(question, now); ok {
		p.DateFrom = from
		p.DateTo = to
		p.Days = days
	} else {
		// No explicit date — leave DateFrom/DateTo empty so fast path
		// functions can apply their own date-range fallback.
		p.Days = 7
	}

	// 卡口名称（仅车流主题）
	if topic == "traffic" {
		p.GateName = gateNameParams(ctx, question)
	}

	// 区域名称
	p.RegionName = regionNameParams(question)

	// 人流标签参数（仅人口主题）
	if topic == "population" {
		popTagParams(question, &p)
	}

	// 网格多维度参数（仅网格主题）
	if topic == "grid" {
		gridDimParams(ctx, question, &p)
	}

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

// ---- 人流标签参数提取 ----

func popTagParams(q string, p *ExtractedParams) {
	q = strings.TrimSpace(q)

	// 标签类别
	tagMap := []struct {
		keys []string
		tag  string
	}{
		{[]string{"年龄", "岁", "年龄段", "年龄段分布", "年龄分布", "年轻", "青年", "中年", "老年", "老人"}, "年龄"},
		{[]string{"性别", "男女", "男女性别", "性别分布", "男", "男性", "女"}, "性别"},
		{[]string{"省内来源", "省内城市", "省内城市来源", "省内来源分布"}, "省内城市来源"},
		{[]string{"省外城市来源", "省外城市", "外省城市"}, "省外城市来源"},
		{[]string{"省外来源", "省外", "外省", "省份来源", "省外来源分布", "来源", "来自"}, "省外来源"},
	}
	for _, m := range tagMap {
		for _, k := range m.keys {
			if strings.Contains(q, k) {
				p.Tag = m.tag
				break
			}
		}
		if p.Tag != "" {
			break
		}
	}

	// 人口方向 type
	if containsAny(q, []string{"进站", "进入", "入站", "入", "进站人流", "进入人流"}) {
		p.PopType = 2
	} else if containsAny(q, []string{"出站", "离开", "出", "出站人流", "离开人流"}) {
		p.PopType = 3
	}

	// 排名数
	topNRe := regexp.MustCompile(`(?:前|top|Top|TOP)(\d+)`)
	if m := topNRe.FindStringSubmatch(q); len(m) > 1 {
		if n, _ := strconv.Atoi(m[1]); n > 0 {
			p.TopN = n
		}
	}
	if p.TopN <= 0 && containsAny(q, []string{"排名", "排行", "前几", "最多", "最热门"}) {
		p.TopN = 5
	}

	// 年龄段标签合并
	if p.Tag == "年龄" {
		if containsAny(q, []string{"年轻", "青年", "年轻人"}) {
			p.Labels = []string{"(0,18]", "(19,22]", "(23,25]", "(26,30]"}
		} else if containsAny(q, []string{"中年"}) {
			p.Labels = []string{"(31,35]", "(36,40]", "(41,45]", "(46,50]"}
		} else if containsAny(q, []string{"老年", "老人"}) {
			p.Labels = []string{"(51,55]", "(56,60]", ">60"}
		}
	} else if p.Tag == "性别" {
		if containsAny(q, []string{"男", "男性"}) {
			p.Labels = []string{"男"}
		} else if containsAny(q, []string{"女", "女性"}) {
			p.Labels = []string{"女"}
		}
	}

	// 区域
	if p.RegionName != "" {
		p.Area = p.RegionName
	}

	// 默认：有标签但无方向时按总人数(type=1)查询
	if p.Tag != "" && p.PopType == 0 {
		p.PopType = 1
	}
}

// ---- 网格多维度参数 ----

var (
	communityNamesCache []string
	communityNamesOnce  sync.Once
	communityNamesMu    sync.RWMutex
)

func loadCommunityNames(ctx context.Context) []string {
	communityNamesOnce.Do(func() {
		refreshCommunityNames(ctx)
	})
	communityNamesMu.RLock()
	defer communityNamesMu.RUnlock()
	return communityNamesCache
}

func refreshCommunityNames(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {}
	}()
	records, err := g.DB("master").Model("grid_case_record").Ctx(ctx).
		Fields("DISTINCT community").
		Where("community <> ''").
		All()
	if err != nil {
		return
	}
	names := make([]string, 0, len(records))
	for _, r := range records {
		name := strings.TrimSpace(r["community"].String())
		if name != "" {
			names = append(names, name)
		}
	}
	communityNamesMu.Lock()
	communityNamesCache = names
	communityNamesMu.Unlock()
}

func gridDimParams(ctx context.Context, q string, p *ExtractedParams) {
	q = strings.TrimSpace(q)

	communities := loadCommunityNames(ctx)
	for _, c := range communities {
		if strings.Contains(q, c) && len(c) > len(p.Community) {
			p.Community = c
		}
	}

	caseTypeMap := []struct {
		keys []string
		ct   string
	}{
		{[]string{"城市管理", "城管"}, "城市管理"},
		{[]string{"环境卫生", "环卫"}, "环境卫生"},
		{[]string{"市容市貌"}, "市容市貌"},
		{[]string{"市场监管"}, "市场监管"},
		{[]string{"社区服务", "社区治理"}, "社区服务"},
	}
	for _, m := range caseTypeMap {
		for _, k := range m.keys {
			if strings.Contains(q, k) {
				p.CaseType = m.ct
				break
			}
		}
		if p.CaseType != "" {
			break
		}
	}
}

// ---- 工具函数 ----

// PeriodLabel 返回适合放在回答中的时间描述，如"5月27日"或"近7天"
func (p ExtractedParams) PeriodLabel(defaultDays int) string {
	days := p.Days
	if days <= 0 {
		days = defaultDays
	}
	if days <= 0 {
		days = 7
	}
	if days == 1 && p.DateFrom != "" {
		return p.DateFrom
	}
	return fmt.Sprintf("近%d天", days)
}
