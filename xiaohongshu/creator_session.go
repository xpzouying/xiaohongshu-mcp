package xiaohongshu

import (
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

const (
	creatorSessionCookie = "galaxy_creator_session_id"
	// 海外站创作者中心直接沿用主站 .rednote.com 的 web_session，没有单独的创作者会话 cookie。
	intlSessionCookie     = "web_session"
	creatorSessionTimeout = 15 * time.Second
)

// ensureCreatorSession 浏览器里没有创作者中心会话时，先打开创作者中心首页，等会话建立
func ensureCreatorSession(page *rod.Page) error {
	if hasCreatorSession(page) {
		return nil
	}

	if err := page.Navigate(configs.CreatorURL("/?source=official")); err != nil {
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
	intl := configs.IsIntl()
	for _, c := range cks {
		if c.Name == creatorSessionCookie {
			return true
		}
		if intl && c.Name == intlSessionCookie && strings.HasSuffix(c.Domain, "rednote.com") {
			return true
		}
	}
	return false
}
