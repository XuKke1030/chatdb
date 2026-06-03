package utility

import (
	"fmt"
	"strings"
)

func PanicErr(err error) {
	if err != nil {
		panic(fmt.Errorf("发生 panic 错误: %s", err.Error()))
	}
}

// SafeUserErr 将内部错误映射为对用户安全的消息。
// 生产环境不应将数据库错误、网络错误、栈追踪等信息直接暴露给用户。
func SafeUserErr(err error) string {
	if err == nil {
		return "当前查询暂时未能完成，请稍后重试。"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	// 白名单：已知可安全暴露的错误模式
	switch {
	case strings.Contains(msg, "GraphRunError") || strings.Contains(lower, "max steps") || strings.Contains(lower, "exceeds max steps"):
		return "当前问题的自动分析步骤过多，暂时未能完成计算。请缩小时间范围或明确统计口径后重试。"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "context deadline") || strings.Contains(lower, "超时"):
		return "查询超时，请尝试简化问题或换一种问法。"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "i/o timeout"):
		return "数据库连接异常，请稍后重试。"
	case strings.Contains(lower, "too many connections") || strings.Contains(lower, "max_connections"):
		return "数据库连接数已满，请稍后重试。"
	case strings.Contains(lower, "access denied") || strings.Contains(lower, "permission denied"):
		return "数据库权限不足，请联系管理员。"
	case strings.Contains(lower, "sql syntax") || strings.Contains(lower, "syntax error") || strings.Contains(lower, "语法"):
		return "SQL 语法有误，AI 将自动修正，请重新提问。"
	case strings.Contains(lower, "no such table") || strings.Contains(lower, "table") && strings.Contains(lower, "doesn't exist") || strings.Contains(lower, "不存在"):
		return "查询的表不存在，AI 将自动修正，请重新提问。"
	case strings.Contains(lower, "unknown column") || strings.Contains(lower, "column") && strings.Contains(lower, "not found"):
		return "查询的列不存在，AI 将自动修正，请重新提问。"
	case strings.Contains(lower, "empty result") || strings.Contains(lower, "0 rows"):
		return "暂无数据。"
	}

	// 默认：不透传内部错误细节
	return "当前查询暂时未能完成，请稍后重试。"
}
