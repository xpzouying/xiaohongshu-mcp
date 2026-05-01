package xiaohongshu

import (
	"context"
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

// isLoggedIn 检查登录状态，兼容 xiaohongshu.com 和 rednote.com
// 通过检查侧边栏是否存在"登录按钮"来判断：有登录按钮=未登录，无登录按钮=已登录
func isLoggedIn(pp *rod.Page) (bool, error) {
	// 先确认页面已加载侧边栏
	exists, _, err := pp.Has(`.side-bar`)
	if err != nil {
		return false, errors.Wrap(err, "check sidebar failed")
	}
	if !exists {
		// 侧边栏未渲染，视为未登录（页面可能还在加载中）
		return false, nil
	}

	// 登录按钮存在 = 未登录
	hasLoginBtn, _, err := pp.Has(`.side-bar .login-btn`)
	if err != nil {
		return false, errors.Wrap(err, "check login button failed")
	}

	return !hasLoginBtn, nil
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	time.Sleep(1 * time.Second)

	return isLoggedIn(pp)
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if loggedIn, _ := isLoggedIn(pp); loggedIn {
		return nil
	}

	// 等待二维码弹窗出现，再开始轮询登录状态
	qrEl, err := pp.Timeout(30 * time.Second).Element(".login-container .qrcode-img")
	if err != nil || qrEl == nil {
		return errors.Wrap(err, "qrcode popup did not appear")
	}

	// 轮询等待登录完成（登录按钮消失表示登录成功）
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "login cancelled")
		case <-timeout:
			return errors.New("login timeout after 5 minutes")
		case <-ticker.C:
			if loggedIn, _ := isLoggedIn(pp); loggedIn {
				return nil
			}
		}
	}
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	loggedIn, err := isLoggedIn(pp)
	if err != nil {
		return "", false, errors.Wrap(err, "check login status failed")
	}
	if loggedIn {
		return "", true, nil
	}

	// 获取二维码图片
	src, err := pp.MustElement(".login-container .qrcode-img").Attribute("src")
	if err != nil {
		return "", false, errors.Wrap(err, "get qrcode src failed")
	}
	if src == nil || len(*src) == 0 {
		return "", false, errors.New("qrcode src is empty")
	}

	return *src, false, nil
}

func (a *LoginAction) WaitForLogin(ctx context.Context) bool {
	pp := a.page.Context(ctx)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if loggedIn, _ := isLoggedIn(pp); loggedIn {
				return true
			}
		}
	}
}
