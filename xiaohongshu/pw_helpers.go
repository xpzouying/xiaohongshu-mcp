package xiaohongshu

import (
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// 本文件集中放各 action 共用、但尚不足以单列文件的 playwright 适配小工具。

// logActionError 记录动作构造期的非致命错误（如首导航超时），不中断流程，
// 由后续读取/轮询兜底。与 go-rod 时代 Must* panic 被上层 recover 的语义对齐。
func logActionError(what string, err error) {
	if err != nil {
		logrus.Warnf("%s: %v", what, err)
	}
}

// evalString 在页面上下文执行表达式并取其字符串返回值；出错或值非字符串时返回 ""。
func evalString(page playwright.Page, expr string) string {
	res, err := page.Evaluate(expr)
	if err != nil {
		return ""
	}
	s, _ := res.(string)
	return s
}

// evalFloat 在页面上下文执行表达式并把返回值解析为 float64；出错或不可解析返回 ok=false。
func evalFloat(page playwright.Page, expr string) (float64, bool) {
	res, err := page.Evaluate(expr)
	if err != nil {
		return 0, false
	}
	switch v := res.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}
