package ai_chat

import (
	"context"
	"strings"
)

// 业务阈值常量
const (
	trafficHighThreshold   = 50000
	trafficYoYAlertPct     = 20.0
	trafficMoMAlertPct     = 15.0
	closeRateTarget        = 85.0
	stayLongBucket         = "4h+"
	hkMacauHighRatioPct    = 15.0
	popHighThreshold       = 100000
	provinceOutsideHighPct = 40.0
)

// insightTemplate 描述单个 kind 的洞察条目模板
type insightTemplate struct {
	keyPoints   []string
	impacts     []string
	suggestions []string
}

// kindInsightTemplates 按 kind 定义洞察模板
// 占位符 {key} 在运行时被 extractInsightData 返回的实际数值替换
// 无法替换的占位符会导致该条目被跳过，确保前端不会收到残缺文本
var kindInsightTemplates = map[fastPathKind]insightTemplate{
	fpTrafficToday: {
		keyPoints:   []string{"当天车流{total}辆", "车流处于{level}水平"},
		impacts:     []string{"车流{level}时{peak}时段承载压力{pressure}"},
		suggestions: []string{"建议关注高峰卡口，必要时加强{action}"},
	},
	fpTrafficTopGateToday: {
		keyPoints:   []string{"最大卡口「{topGate}」车流{topCount}辆", "需关注该卡口通行效率和拥堵风险"},
		impacts:     []string{"单卡口集中度过高时，周边道路承载压力增大"},
		suggestions: []string{"建议对「{topGate}」增加现场疏导力量"},
	},
	fpTrafficWeek: {
		keyPoints:   []string{"近{days}天日均{dailyAvg}辆", "港澳车占比{hkRatio}%"},
		impacts:     []string{"趋势数据可辅助判断周期性波动和异常峰值"},
		suggestions: []string{"建议关注趋势中的异常波动日，排查数据或业务原因"},
	},
	fpTrafficHkMacau: {
		keyPoints:   []string{"港澳车{hkCount}辆，占比{hkRatio}%"},
		impacts:     []string{"港澳车占比{hkLevel}，口岸及通关资源需相应调配"},
		suggestions: []string{"占比持续偏高时建议评估通关通道容量"},
	},
	fpTrafficGateRank: {
		keyPoints:   []string{"排名首位卡口「{topGate}」车流{topCount}辆"},
		impacts:     []string{"卡口集中度反映路网负荷分布"},
		suggestions: []string{"建议对 Top3 卡口制定差异化的疏导方案"},
	},
	fpTrafficWeekendCompare: {
		keyPoints:   []string{"周末与工作日日均车流差异{diffPct}%"},
		impacts:     []string{"周末差异可辅助调配值班和巡检资源"},
		suggestions: []string{"差异较大时建议周末增派疏导力量"},
	},
	fpTrafficHolidayCompare: {
		keyPoints:   []string{"节假日与工作日日均车流差异{diffPct}%"},
		impacts:     []string{"节假日高峰需提前部署保障力量"},
		suggestions: []string{"建议节前3天启动高峰应对预案"},
	},
	fpTrafficProvinceInside: {
		keyPoints:   []string{"省内车{insideCount}辆（占大陆车{insidePct}%），省外车{outsideCount}辆"},
		impacts:     []string{"省外车占比{outsideLevel}，需关注外地车涌入带来的管理压力"},
		suggestions: []string{"省外车集中时可加强重点卡口外地车引导"},
	},
	fpTrafficYoY: {
		keyPoints:   []string{"同比{dir}{yoyPct}%"},
		impacts:     []string{"同比变化{yoyLevel}，{needPlan}调整年度通行预案"},
		suggestions: []string{"建议结合节假日和天气因素深入分析变化原因"},
	},
	fpTrafficMoM: {
		keyPoints:   []string{"环比{dir}{momPct}%"},
		impacts:     []string{"环比变化{momLevel}，短期波动需区分趋势与噪声"},
		suggestions: []string{"建议持续观察3个周期确认趋势方向"},
	},
	fpTrafficHoliday: {
		keyPoints:   []string{"{holidayName}期间车流{total}辆，日均{dailyAvg}辆"},
		impacts:     []string{"节假日车流集中，保障压力高于平日"},
		suggestions: []string{"建议节前发布出行提示，节中加强卡口疏导"},
	},
	fpTrafficStayDistribution: {
		keyPoints:   []string{"停留集中在{topBucket}时段", "共有{totalVehicles}辆车有停留记录"},
		impacts:     []string{"长停留车辆可能影响区域交通和停车资源"},
		suggestions: []string{"建议对长停留车辆核查停留原因"},
	},
	fpTrafficOriginByProvince: {
		keyPoints:   []string{"省外车主要来自「{topOrigin}」，共{topCount}辆"},
		impacts:     []string{"来源集中度可辅助跨区域协作研判"},
		suggestions: []string{"来源地集中时可加强与对应省份的联动管理"},
	},
	fpTrafficOverview: {
		keyPoints:   []string{"近{days}天车流{total}辆，日均{dailyAvg}辆", "港澳车占比{hkRatio}%，省内车占大陆车{insidePct}%"},
		impacts:     []string{"多维汇总可辅助全局态势感知和资源调度"},
		suggestions: []string{"建议对关键指标设立阈值告警，实现主动预警"},
	},
	fpPopWeekTrend: {
		keyPoints:   []string{"近{days}天人流{total}人次，日均{dailyAvg}人次"},
		impacts:     []string{"人流趋势可辅助公共服务保障和场地容量评估"},
		suggestions: []string{"关注趋势中的异常峰值日，排查活动或突发事件"},
	},
	fpPopRegionRank: {
		keyPoints:   []string{"人流最大区域「{topRegion}」，共{topCount}人次"},
		impacts:     []string{"区域集中度反映公共服务资源分布压力"},
		suggestions: []string{"建议对 Top3 区域提前部署服务保障力量"},
	},
	fpPopHourlyTrend: {
		keyPoints:   []string{"人流高峰时段{peakHour}，峰值{peakCount}人次"},
		impacts:     []string{"小时级分布可辅助安保和值班排班"},
		suggestions: []string{"建议在高峰时段前30分钟启动人流疏导"},
	},
	fpPopOverview: {
		keyPoints:   []string{"近{days}天人流{total}人次，日均{dailyAvg}人次"},
		impacts:     []string{"综合汇总可辅助人防物防资源规划"},
		suggestions: []string{"建议对人流超阈值区域启动分级响应"},
	},
	fpGridCaseCount: {
		keyPoints:   []string{"本月案件{total}件，结案率{closeRate}%"},
		impacts:     []string{"案件规模{caseLevel}，治理压力{pressureLevel}"},
		suggestions: []string{"建议对未结案件加快督办，复盘高发原因"},
	},
	fpGridCloseRate: {
		keyPoints:   []string{"结案率{closeRate}%，{comparedTo}目标值{target}%"},
		impacts:     []string{"结案率反映基层治理效能和督办力度"},
		suggestions: []string{"低于目标时建议启动专项督办机制"},
	},
	fpGridRegionRank: {
		keyPoints:   []string{"案件最多区域「{topRegion}」，共{topCount}件"},
		impacts:     []string{"区域集中度反映重点治理区域"},
		suggestions: []string{"建议对 Top3 区域制定专项治理方案"},
	},
	fpGridCaseTypeDist: {
		keyPoints:   []string{"案件类型以「{topType}」为主，占比{topPct}%"},
		impacts:     []string{"类型分布可辅助精准配置执法资源"},
		suggestions: []string{"建议对高发类型加强前端预防和巡查"},
	},
	fpGridAvgHandle: {
		keyPoints:   []string{"平均处理时长最长为{top_type}，{avg_hours}小时", "最短为{bottom_type}，{bottom_hours}小时"},
		impacts:     []string{"处理时长影响案件流转效率和市民满意度"},
		suggestions: []string{"建议对处理时长较长案件类型优化流程"},
	},
	fpTrafficDwellTop: {
		keyPoints:   []string{"驻留时长最长的车辆为{top_plate}，停留{dwell_hours}小时"},
		impacts:     []string{"长时间驻留可能涉及非法营运或走私风险"},
		suggestions: []string{"建议调取该车辆轨迹进行深度分析"},
	},
	fpTrafficForeignOrigin: {
		keyPoints:   []string{"外地车来源排名首位为{top_province}，共{top_count}辆"},
		impacts:     []string{"来源集中度高，可聚焦重点省份布控"},
		suggestions: []string{"建议关注Top3来源地的车辆类型与停留时长"},
	},
	fpTrafficHkMacauStay: {
		keyPoints:   []string{"港澳车停留{bucket}时段占比最高", "平均停留{avgStay}分钟"},
		impacts:     []string{"港澳车停留时长影响口岸区域资源占用"},
		suggestions: []string{"建议根据停留时长分布优化口岸周边停车位配置"},
	},
	fpPopHolidayCompare: {
		keyPoints:   []string{"节假日日均{holiday_avg}人，平日日均{workday_avg}人"},
		impacts:     []string{"节假日人流变化{change_pct}，{direction}方向显著"},
		suggestions: []string{"建议根据节假日增幅调配安保资源"},
	},
	fpPopYoY: {
		keyPoints:   []string{"今年日均{current_val}人，去年同期{prior_val}人"},
		impacts:     []string{"同比变化{change_pct}，{direction}趋势明显"},
		suggestions: []string{"结合政策变化分析同比增长驱动因素"},
	},
	fpPopMultiRegionCompare: {
		keyPoints:   []string{"人流最密集区域为{top_region}，日均{top_val}人"},
		impacts:     []string{"区域间差异达{max_diff}人，资源分配需差异化"},
		suggestions: []string{"建议在Top区域加强人流监控和应急部署"},
	},
	fpPopTagDistribution: {
		keyPoints:   []string{"「{tag}」分布最多为{top_label}，占比{top_pct}%", "{top_label}较上月{change_dir}"},
		impacts:     []string{"标签分布可辅助公共服务精准配置"},
		suggestions: []string{"建议对占比较高标签群体定向优化服务"},
	},
	fpPopTagTopN: {
		keyPoints:   []string{"「{tag}」排名第1为{top_label}，人数{top_count}，占比{top_pct}%", "Top3合计占比{top3_pct}%"},
		impacts:     []string{"标签TopN排名可指导资源优先投向"},
		suggestions: []string{"建议对排名靠前标签群体加大服务力度"},
	},
	fpPopTagTrend: {
		keyPoints:   []string{"「{top_label}」趋势{trend_dir}，变化幅度{change_pct}%"},
		impacts:     []string{"标签趋势变化可辅助预测性资源配置"},
		suggestions: []string{"建议关注持续上升的标签群体，提前调整服务供给"},
	},
	fpPopActivationSummary: {
		keyPoints:   []string{"活力指数{activation}，偏离基线{deviation_pct}%，{deviation_desc}", "{is_anomaly}"},
		impacts:     []string{"活力偏离基线可提示人流异常活跃或低迷"},
		suggestions: []string{"建议结合活力偏离度判断是否需要增减服务供给"},
	},
	fpPopActivationTrend: {
		keyPoints:   []string{"活力趋势{trend_dir}，最新值{activation_latest}，基线{baseline_latest}", "{crossover_point}"},
		impacts:     []string{"活力持续高于基线可能预示常态化人流增长"},
		suggestions: []string{"建议在活力持续高于基线时段预置更多服务资源"},
	},
	fpPopPortrait: {
		keyPoints:   []string{"总量{all_count}人，活力{activation}{deviation_desc}", "年龄占比最高{age_top_label}({age_top_pct}%)", "省外来源第1「{origin_top_label}」({origin_top_pct}%)"},
		impacts:     []string{"画像综合呈现可支持一站式决策"},
		suggestions: []string{"建议结合活力偏离和来源地构成优化跨区域协同"},
	},
	fpPopFloatingAnomaly: {
		keyPoints:   []string{"流动人口异常区域{anomaly_count}个，增幅最大为{top_region}({top_pct}%)"},
		impacts:     []string{"异常增长可能关联治安或流动人口管理风险"},
		suggestions: []string{"建议对增幅超20%区域启动专项排查"},
	},
	fpTrafficInOutRatio: {
		keyPoints:   []string{"进入车辆{inCount}辆（占比{inPct}%），离开{outCount}辆（占比{outPct}%）"},
		impacts:     []string{"进出方向失衡可能影响区域交通负荷和停车资源分配"},
		suggestions: []string{"建议对进入集中时段加强入场疏导和车位指引"},
	},
	fpTrafficMultiGateCompare: {
		keyPoints:   []string{"Top3卡口合计占总量{top3Pct}%"},
		impacts:     []string{"卡口集中度过高时，局部路网承载压力增大"},
		suggestions: []string{"建议对集中度最高的卡口制定差异化疏导方案"},
	},
	fpTrafficHkMacauYoY: {
		keyPoints:   []string{"港澳车同比{dir}{yoyPct}%"},
		impacts:     []string{"港澳车同比变化{yoyLevel}，口岸通关资源需相应调整"},
		suggestions: []string{"建议根据同比变化趋势调整口岸通道开放策略"},
	},
	fpPopTagProportion: {
		keyPoints:   []string{"「{tag}」占比最高为{topLabel}（{topPct}%）"},
		impacts:     []string{"占比结构可辅助公共服务资源精准配置"},
		suggestions: []string{"建议对占比较高标签群体定向优化服务供给"},
	},
	fpPopMultiTagTrend: {
		keyPoints:   []string{"「{topLabel}」趋势{trendDir}，变化幅度{changePct}%"},
		impacts:     []string{"多标签趋势变化可辅助预测性资源配置"},
		suggestions: []string{"建议关注持续上升的标签群体，提前调整服务供给"},
	},
	fpPopComprehensive: {
		keyPoints:   []string{"近{days}天人流{total}人次，区域最多「{topRegion}」", "年龄占比最高{ageTopLabel}({ageTopPct}%)"},
		impacts:     []string{"综合画像可支持一站式决策和全局态势感知"},
		suggestions: []string{"建议对关键指标设立阈值告警，实现主动预警"},
	},
	fpPopMultiTagCompare: {
		keyPoints:   []string{"年龄占比最高{ageTopLabel}({ageTopPct}%)", "性别：男{malePct}%/女{femalePct}%", "省外来源第1「{originTopLabel}」({originTopPct}%)"},
		impacts:     []string{"多维标签对比可辅助跨维度交叉研判"},
		suggestions: []string{"建议结合标签交叉特征优化服务组合策略"},
	},
	fpGridOverview: {
		keyPoints:   []string{"本月案件总量{total_cases}件，结案率{close_rate}%"},
		impacts:     []string{"结案率{close_rate_direction}，处置效率{efficiency_direction}"},
		suggestions: []string{"建议对未结案件加快督办，复盘高发原因"},
	},
}

// buildKindInsight 按 kind 模板和结论文本生成数据化洞察条目
// 当 kind 无模板时，降级到 topic 级通用洞察
func buildKindInsight(ctx context.Context, fp *fastPath, conclusion string) (keyPoints, impacts, suggestions []string) {
	tmpl, ok := kindInsightTemplates[fp.kind]
	if !ok {
		return buildInsightItems(fp, conclusion)
	}

	data := extractInsightData(ctx, fp.kind, conclusion)

	keyPoints = renderTemplateItems(tmpl.keyPoints, data)
	impacts = renderTemplateItems(tmpl.impacts, data)
	suggestions = renderTemplateItems(tmpl.suggestions, data)
	return keyPoints, impacts, suggestions
}

func renderTemplateItems(templates []string, data map[string]string) []string {
	result := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		s := tmpl
		for k, v := range data {
			s = replacePlaceholder(s, k, v)
		}
		if containsPlaceholder(s) {
			continue
		}
		result = append(result, s)
	}
	return result
}

func replacePlaceholder(s, key, value string) string {
	return strings.ReplaceAll(s, "{"+key+"}", value)
}

func containsPlaceholder(s string) bool {
	return strings.Contains(s, "{") && strings.Contains(s, "}")
}
