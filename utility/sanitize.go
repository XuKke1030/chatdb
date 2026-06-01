package utility

import (
	"regexp"
	"strings"
)

// sanitizeRules 定义表名/字段名到业务化中文的映射
var sanitizeRules = []struct {
	pattern string
	replace string
}{
	// 表名
	{`case_list`, "案件记录"},
	{`grid_case_record`, "案件记录"},
	{`grid_metric_daily`, "案件统计"},
	{`grid_metric_monthly`, "案件统计"},
	{`population_tag_daily`, "人口统计"},
	{`traffic_gate_record`, "过车记录"},
	{`traffic_holiday`, "节假日配置"},
	// 字段名
	{`pending_step`, "当前环节"},
	{`report_time`, "上报时间"},
	{`update_time`, "更新时间"},
	{`case_source`, "案件来源"},
	{`case_type1`, "一级类别"},
	{`case_type2`, "二级类别"},
	{`case_type`, "案件类别"},
	{`case_status`, "处置状态"},
	{`metric_date`, "统计日期"},
	{`metric_month`, "统计月份"},
	{`responsibility_unit`, "责任单位"},
	{`community`, "社区"},
	{`region`, "区域"},
	{`label_cnt`, "标签人数"},
	{`label_type`, "标签类型"},
	{`gate_id`, "卡口编号"},
	{`plate_color`, "车牌颜色"},
	{`plate_number`, "车牌号"},
	{`pass_time`, "通行时间"},
	{`direction`, "方向"},
}

// bracketPattern 匹配中文/英文括号中仅含英文下划线标识符的内容，如（case_list）(pending_step)
var bracketPattern = regexp.MustCompile(`[（(]\s*[a-z_]+\s*[）)]`)

// SanitizeOutput 对 AI 输出文本做后置过滤，将表名/字段名替换为业务化中文，删除括号备注
func SanitizeOutput(text string) string {
	// 先删除括号备注形式，如（case_list）、(pending_step)
	text = bracketPattern.ReplaceAllString(text, "")

	// 再做标识符替换（按长度降序，避免短模式吞掉长模式的前缀）
	type rule struct {
		from string
		to   string
	}
	sorted := make([]rule, len(sanitizeRules))
	copy(sorted, func() []rule {
		r := make([]rule, len(sanitizeRules))
		for i, s := range sanitizeRules {
			r[i] = rule{s.pattern, s.replace}
		}
		return r
	}())
	// 按pattern长度降序排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[j].from) > len(sorted[i].from) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for _, r := range sorted {
		text = strings.ReplaceAll(text, r.from, r.to)
	}

	return text
}
