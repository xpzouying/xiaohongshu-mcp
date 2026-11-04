package xiaohongshu

import (
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// 小红书把「可疑会话」重定向到这些路径。
const (
	// captchaPath 安全验证页：需要用手机 App 扫码做身份核验。
	captchaPath = "/website-login/captcha"
	// loginPath 登录页：未登录或登录态已失效。
	loginPath = "/website-login"
)

// classifyBlockedPath 按路径判断页面是否被风控拦截。
//
// 只认路径、不看查询串：登录页会带 redirectPath=/website-login/captcha 这类参数，
// 若对整条 URL 做子串匹配，就会把登录页误判成验证页。
//
// 未知的 /website-login/<sub> 一律归为登录页：对调用方来说「请重新登录」
// 是更安全的指引，而验证页的路径是固定且已知的。
func classifyBlockedPath(path string) error {
	switch {
	case path == captchaPath, strings.HasPrefix(path, captchaPath+"/"):
		return errors.ErrVerificationRequired
	case path == loginPath, strings.HasPrefix(path, loginPath+"/"):
		return errors.ErrLoginRequired
	}
	return nil
}

// classifyBlockedURL 先解析再按路径判断；解析失败视为未拦截。
//
// 用 EscapedPath 而不是 Path：Path 会把 %2F 解码成 /，而 JS 侧的
// location.pathname 保持编码形式，两者不一致会导致同一页面判定不同。
func classifyBlockedURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	return classifyBlockedPath(u.EscapedPath())
}

// CheckPageBlocked 读取主框架地址，判断是否被风控拦截。
//
// 未命中、或读不到地址都返回 nil——判定只是附加信息，取不到时不应反过来
// 影响主流程。
func CheckPageBlocked(page *rod.Page) error {
	// 用 CDP 的 Page.getFrameTree 读主框架地址：纯协议调用、不注入 JS，
	// 且受 page 自身的超时与取消约束——这一点是选用它而不是 page.Info() 的原因：
	// page.Info() 走 browser session，不受 page.Timeout 约束，页面被取消后
	// 仍会再发一次 CDP 调用，无法给它设上限。
	frameTree, err := (proto.PageGetFrameTree{}).Call(page.Timeout(3 * time.Second))
	if err != nil {
		logrus.Debugf("读取主框架地址失败，跳过风控页判定: %v", err)
		return nil
	}
	if frameTree == nil || frameTree.FrameTree == nil || frameTree.FrameTree.Frame == nil {
		return nil
	}
	return classifyBlockedURL(frameTree.FrameTree.Frame.URL)
}
