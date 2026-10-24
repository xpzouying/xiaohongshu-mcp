package xiaohongshu

import (
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

const creatorSessionTimeout = 60 * time.Second

// ensureCreatorSession 只在目标站点的发布表单可用后继续。
func ensureCreatorSession(page *rod.Page) error {
	page = page.Timeout(creatorSessionTimeout)
	if hasCreatorSession(page) {
		return nil
	}
	if err := page.Navigate(creatorURL("/publish/publish?source=official")); err != nil {
		return errors.Wrap(err, "打开创作者中心失败")
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if hasCreatorSession(page) {
			return nil
		}
		select {
		case <-page.GetContext().Done():
			return errors.Wrap(page.GetContext().Err(), "创作者中心登录未完成；请在所选站点完成登录后重试")
		case <-ticker.C:
		}
	}
}

// isCreatorPage 同时校验协议、域名和路径，不能把登录重定向当成成功。
func isCreatorPage(rawURL, path string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	base, _ := url.Parse(creatorURL(""))
	return u.Scheme == "https" && u.Host == base.Host && u.Path == path
}

func hasCreatorSession(page *rod.Page) bool {
	info, err := page.Info()
	if err != nil || !isCreatorPage(info.URL, "/publish/publish") {
		return false
	}
	// Cookie 可能过期或属于另一个站点，必须看到实际的上传控件。
	has, _, err := page.Has(`input[type="file"]`)
	return err == nil && has
}
