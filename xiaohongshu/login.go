package xiaohongshu

import (
	"context"
	"fmt"
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

	ok, err := a.isLoggedIn(ctx)
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}
	return ok, nil
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists && !hasLoginGate(pp) {
		// 已经登录，直接返回
		return nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("等待登录超时，请在弹出的浏览器中完成扫码登录")
		case <-ticker.C:
			ok, err := a.isLoggedIn(waitCtx)
			if err != nil {
				// 扫码登录过程中页面会刷新或重建，短暂会话抖动直接重试。
				continue
			}
			if ok {
				return nil
			}
		}
	}
}

func hasLoginGate(page *rod.Page) bool {
	result, err := page.Eval(`() => {
		const bodyText = document.body ? document.body.innerText : "";
		const input = document.querySelector('#search-input');
		const placeholder = input ? (input.getAttribute('placeholder') || '') : '';
		return bodyText.includes('获取验证码') ||
			bodyText.includes('登录后查看搜索结果') ||
			placeholder.includes('登录探索更多内容');
	}`)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}

func (a *LoginAction) isLoggedIn(ctx context.Context) (bool, error) {
	pp := a.page.Context(ctx)
	if hasLoginGate(pp) {
		return false, nil
	}

	exists, _, err := pp.Has(`.main-container .user .link-wrapper .channel`)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	pp.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists {
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
			el, err := pp.Element(".main-container .user .link-wrapper .channel")
			if err == nil && el != nil {
				return true
			}
		}
	}
}
