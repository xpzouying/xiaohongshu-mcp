package xiaohongshu

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

const (
	creatorSessionCookie  = "galaxy_creator_session_id"
	creatorSessionTimeout = 60 * time.Second
)

// ensureCreatorSession 浏览器里没有创作者中心会话时，先打开创作者中心首页，等会话建立
func ensureCreatorSession(page *rod.Page) error {
	if hasCreatorSession(page) {
		return nil
	}

	if err := page.Navigate(creatorLoginURL()); err != nil {
		return errors.Wrap(err, "打开创作者中心失败")
	}

	deadline := time.Now().Add(creatorSessionTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if hasCreatorSession(page) {
			humanize.Delay(page.GetContext(), humanize.AfterNavigate)
			return nil
		}
	}
	if isRedNote() {
		return errors.New("RedNote 登录态已存在，但创作者中心没有完成 SSO；请先在 MCP 浏览器中打开创作者中心并完成登录")
	}
	return errors.New("创作者中心登录失效，请重新扫码登录")
}

func hasCreatorSession(page *rod.Page) bool {
	cks, err := page.Browser().GetCookies()
	if err == nil {
		for _, c := range cks {
			if c.Name == creatorSessionCookie {
				return true
			}
		}
	}
	// RedNote 的创作者中心可能完成 SSO 跳转，但不会向 MCP 暴露这个 Cookie。
	if isRedNote() {
		result, err := page.Eval(`() => location.hostname === "creator.rednote.com" && location.pathname.startsWith("/publish/")`)
		return err == nil && result.Value.Bool()
	}
	return false
}
