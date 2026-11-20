package xiaohongshu

import (
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

const (
	urlOfCreatorHome = `https://creator.xiaohongshu.com/?source=official`

	creatorSessionCookie  = "galaxy_creator_session_id"
	creatorSessionTimeout = 15 * time.Second
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

	// 创作者中心有时能直接打开发布页，而不下发旧版会话 Cookie。
	if err := page.Navigate(urlOfPublic); err != nil {
		return errors.Wrap(err, "打开创作者中心发布页失败")
	}
	probe := page.Timeout(creatorSessionTimeout)
	defer probe.CancelTimeout()
	for probe.GetContext().Err() == nil {
		switch creatorPublishPageState(probe) {
		case "ready":
			return nil
		case "login":
			return errors.New("创作者中心要求登录，请重新扫码登录")
		case "verification":
			return errors.New("创作者中心要求安全验证，请在浏览器中完成验证")
		}
		select {
		case <-probe.GetContext().Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	return errors.New("创作者中心发布页未就绪，未找到可用的上传入口")
}

func creatorPublishPageState(page *rod.Page) string {
	info, err := page.Info()
	if err != nil {
		return ""
	}
	u, err := url.Parse(info.URL)
	if err != nil || u.Scheme != "https" || u.Host != "creator.xiaohongshu.com" {
		return ""
	}
	if u.Path == "/login" || strings.HasPrefix(u.Path, "/login/") {
		return "login"
	}
	dialogs, err := page.Elements(`[role="dialog"], [aria-modal="true"]`)
	if err != nil {
		return ""
	}
	for _, dialog := range dialogs {
		if visible, err := dialog.Visible(); err != nil || !visible {
			continue
		}
		text, err := dialog.Text()
		if err != nil {
			return ""
		}
		switch {
		case strings.Contains(text, "安全验证"), strings.Contains(text, "请完成验证"),
			strings.Contains(text, "滑动验证"), strings.Contains(text, "访问异常"):
			return "verification"
		case strings.Contains(text, "短信登录"), strings.Contains(text, "扫码登录"),
			strings.Contains(text, "请先登录"):
			return "login"
		}
	}
	if strings.TrimSuffix(u.Path, "/") != "/publish/publish" {
		return ""
	}
	panels, err := page.Elements("div.upload-content")
	if err != nil || len(panels) == 0 {
		return ""
	}
	if visible, err := panels[0].Visible(); err != nil || !visible {
		return ""
	}
	tabs, err := page.Elements("div.creator-tab")
	if err != nil {
		return ""
	}
	var imageTab, videoTab bool
	for _, tab := range tabs {
		if visible, err := tab.Visible(); err != nil || !visible {
			continue
		}
		text, err := tab.Text()
		if err != nil {
			return ""
		}
		switch strings.TrimSpace(text) {
		case "上传图文":
			imageTab = true
		case "上传视频":
			videoTab = true
		}
	}
	if imageTab && videoTab {
		return "ready"
	}
	return ""
}

func hasCreatorSession(page *rod.Page) bool {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return false
	}
	for _, c := range cks {
		if c.Name == creatorSessionCookie {
			return true
		}
	}
	return false
}
