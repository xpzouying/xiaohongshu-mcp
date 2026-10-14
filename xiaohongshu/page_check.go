package xiaohongshu

import (
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// 小红书把"可疑会话"重定向到这些路径。
//
// 命中时页面上不会出现 __INITIAL_STATE__，而业务流程等的是这个变量，
// 于是只能干等到 deadline 耗尽，最后报一句 "context deadline exceeded"。
// 从日志上完全看不出发生了什么——既像加载慢，又像选择器失效，
// 定位成本极高。这里把这种情况识别出来，直接给出可据以行动的错误。
const (
	// captchaPath 安全验证页：需要手机 App 扫码做身份核验。
	captchaPath = "/website-login/captcha"
	// loginPath 登录页：未登录或登录态失效。
	loginPath = "/website-login"
)

// classifyBlockedURL 按地址判断页面是否被风控拦截。
//
// 单独抽成纯函数（不依赖浏览器），是为了能脱离网络做单元测试——
// 这类判定一旦写错，会把正常页面误判成被拦截，比不判定更糟。
func classifyBlockedURL(rawURL string) error {
	switch {
	case strings.Contains(rawURL, captchaPath):
		return errors.ErrVerificationRequired
	case strings.Contains(rawURL, loginPath):
		return errors.ErrLoginRequired
	}
	return nil
}

// CheckPageBlocked 读取当前页面地址，判断是否被风控拦截。
//
// 未命中返回 nil；读不到地址也返回 nil——判定只是"锦上添花"，
// 取不到信息时不应该反过来影响主流程。
func CheckPageBlocked(page *rod.Page) error {
	// 用 CDP 的 Page.getFrameTree 读主框架地址：纯协议调用，不注入 JS。
	//
	// 注意：不要改用 page.Info() 或 Target.getTargetInfo——实测两者的 URL 字段
	// 都是空字符串（rod v0.116.2），拿它判定会永远落空，而且不报错、极难发现。
	// 只是取一次地址，给个短超时，避免它自身吃掉主流程的预算。
	frameTree, err := (proto.PageGetFrameTree{}).Call(page.Timeout(3 * time.Second))
	if err != nil || frameTree == nil || frameTree.FrameTree == nil {
		return nil
	}
	return classifyBlockedURL(frameTree.FrameTree.Frame.URL)
}
