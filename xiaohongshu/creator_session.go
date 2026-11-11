package xiaohongshu

import (
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

const (
	urlOfCreatorHome = `https://creator.xiaohongshu.com/?source=official`

	creatorSessionCookie = "galaxy_creator_session_id"
	// 2026-11 起小红书不再下发 galaxy_creator_session_id，
	// 创作者中心改为复用主站 web_session 走 SSO，这里作为兜底判断。
	creatorSessionWebSessionCookie = "web_session"
	creatorSessionTimeout          = 15 * time.Second
)

// ensureCreatorSession 浏览器里没有创作者中心会话时，先打开创作者中心首页，等会话建立
func ensureCreatorSession(page *rod.Page) error {
	if hasCreatorSession(page) {
		return nil
	}

	if err := page.Navigate(urlOfCreatorHome); err != nil {
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
	return errors.New("创作者中心登录失效，请重新扫码登录")
}

func hasCreatorSession(page *rod.Page) bool {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return false
	}
	return hasCreatorSessionCookie(cks)
}

// hasCreatorSessionCookie 判断 cookie 中是否包含创作者中心登录态。
// 优先认 galaxy_creator_session_id；该 cookie 不再下发时，
// 主站 web_session 仍可通过 SSO 打开创作者中心。
func hasCreatorSessionCookie(cks []*proto.NetworkCookie) bool {
	for _, c := range cks {
		if c.Name == creatorSessionCookie {
			return true
		}
		if c.Name == creatorSessionWebSessionCookie && strings.Contains(c.Domain, "xiaohongshu.com") {
			return true
		}
	}
	return false
}
