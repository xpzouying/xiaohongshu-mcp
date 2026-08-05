package xiaohongshu

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/playwright-community/playwright-go"
)

type LoginAction struct {
	page playwright.Page
}

func NewLogin(page playwright.Page) *LoginAction {
	return &LoginAction{page: page}
}

// loginChannelSelector 登录后侧边栏「我」频道的标识元素。
const loginChannelSelector = `.main-container .user .link-wrapper .channel`

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	if _, err := a.page.Goto("https://www.xiaohongshu.com/explore", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30_000),
	}); err != nil {
		return false, errors.Wrap(err, "check login status navigate failed")
	}

	time.Sleep(1 * time.Second)

	el, err := a.page.QuerySelector(loginChannelSelector)
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}
	if el == nil {
		return false, errors.New("login status element not found")
	}
	return true, nil
}

// CurrentUser 当前登录用户的基础信息。
type CurrentUser struct {
	Nickname string `json:"nickname"`
	UserID   string `json:"userId"`
}

// CurrentUser 从当前页面的 __INITIAL_STATE__ 读取登录用户信息。
// 需在 CheckLoginStatus 之后调用：复用已加载的 explore 页，不做额外导航。
func (a *LoginAction) CurrentUser(ctx context.Context) (*CurrentUser, error) {
	res, err := a.page.Evaluate(`() => {
		const u = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user;
		const info = u && u.userInfo && u.userInfo.value !== undefined ? u.userInfo.value : (u && u.userInfo);
		if (!info || info.guest) return "";
		return JSON.stringify({nickname: info.nickname, userId: info.userId || info.user_id});
	}`)
	if err != nil {
		return nil, errors.Wrap(err, "read current user state failed")
	}

	raw, _ := res.(string)
	if raw == "" {
		return nil, errors.New("current user not found in page state")
	}

	var user CurrentUser
	if err := json.Unmarshal([]byte(raw), &user); err != nil {
		return nil, errors.Wrap(err, "unmarshal current user failed")
	}
	return &user, nil
}

func (a *LoginAction) Login(ctx context.Context) error {
	if _, err := a.page.Goto("https://www.xiaohongshu.com/explore", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60_000),
	}); err != nil {
		return errors.Wrap(err, "navigate to explore failed")
	}

	time.Sleep(2 * time.Second)

	if el, _ := a.page.QuerySelector(loginChannelSelector); el != nil {
		return nil
	}

	// 等待登录元素出现（扫码完成后）
	_, err := a.page.WaitForSelector(loginChannelSelector, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(120_000),
	})
	return err
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	if _, err := a.page.Goto("https://www.xiaohongshu.com/explore", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60_000),
	}); err != nil {
		return "", false, errors.Wrap(err, "navigate to explore failed")
	}

	time.Sleep(2 * time.Second)

	if el, _ := a.page.QuerySelector(loginChannelSelector); el != nil {
		return "", true, nil
	}

	qr, err := a.page.QuerySelector(".login-container .qrcode-img")
	if err != nil || qr == nil {
		return "", false, errors.New("qrcode image element not found")
	}
	src, err := qr.GetAttribute("src")
	if err != nil {
		return "", false, errors.Wrap(err, "get qrcode src failed")
	}
	if src == "" {
		return "", false, errors.New("qrcode src is empty")
	}
	return src, false, nil
}

func (a *LoginAction) WaitForLogin(ctx context.Context) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			el, err := a.page.QuerySelector(loginChannelSelector)
			if err == nil && el != nil {
				return true
			}
		}
	}
}
