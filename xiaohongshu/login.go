package xiaohongshu

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	time.Sleep(1 * time.Second)

	exists, _, err := pp.Has(`.main-container .user .link-wrapper .channel`)
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}

	if !exists {
		return false, errors.Wrap(err, "login status element not found")
	}

	return true, nil
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists {
		// 已经登录，直接返回
		return nil
	}

	// 等待扫码成功提示或者登录完成
	// 这里我们等待登录成功的元素出现，这样更简单可靠
	pp.MustElement(".main-container .user .link-wrapper .channel")

	return nil
}

func (a *LoginAction) LoginQrcodeImg(ctx context.Context, b *headless_browser.Browser) (string, string, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists {
		// 已经登录，直接返回
		_ = pp.Close()
		b.Close()
		return "已经在登录状态", "0", nil
	}

	// 获取二维码图片
	attribute, err := pp.MustElement(".login-container .qrcode-img").Attribute("src")
	if err != nil || attribute == nil || len(*attribute) == 0 {
		_ = pp.Close()
		b.Close()
		return "", "0", errors.Wrap(err, "login qrcode failed")
	}

	const timeout = 4 * time.Minute
	const poll = 750 * time.Millisecond

	// 这里我们用 goroutine 等待登录成功的元素出现，二维码图片直接返回
	go func() {
		defer func() { // 防止 rod 的 Must* panic 影响进程
			if r := recover(); r != nil {
				logrus.Errorf("[xhs-login] goroutine recovered: %v", r)
			}
		}()

		// 单独用 Background，避免调用方 cancel 影响这里
		p := a.page.Context(context.Background())
		deadline := time.Now().Add(timeout)

		selector := ".main-container .user .link-wrapper .channel"

		for time.Now().Before(deadline) {
			// 用带超时的非 Must 查找，避免阻塞 & panic
			if el, err := p.Timeout(3 * time.Second).Element(selector); err == nil && el != nil {
				if err := saveCookies(p); err != nil {
					logrus.Errorf("failed to save cookies: %v", err)
				}
				_ = p.Close()
				b.Close()
				return
			}
			time.Sleep(poll)
		}

		// 超时
		_ = p.Close()
		b.Close()
	}()

	return *attribute, timeout.String(), nil
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}
