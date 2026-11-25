package xiaohongshu

import (
	"context"
	"encoding/json"
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

// loginPageURL 登录与查登录态都用的页面。
//
// 不能用 /explore：海外 IP 访问 www.xiaohongshu.com/explore（以及 /explore/<id>、/user/profile/...）
// 会被 302 到 www.rednote.com，那边是另一套登录态，cookies 里的小红书会话在那儿不算登录，
// 扫码也会登进 rednote（设备管理里显示为「未知」设备）。搜索结果页不会被重定向，
// 未登录时同样弹出小红书的扫码框。
const loginPageURL = "https://www.xiaohongshu.com/search_result?keyword=%E5%B0%8F%E7%BA%A2%E4%B9%A6&source=web_explore_feed"

const loggedInSelector = ".main-container .user .link-wrapper .channel"

// loggedInJS 读页面自己的登录态；侧栏「我」的入口在部分页面上不存在，只作兜底。
const loggedInJS = `(selector) => {
	const unwrap = v => v?.value !== undefined ? v.value : (v?._value !== undefined ? v._value : v);
	const user = window.__INITIAL_STATE__?.user;
	const info = unwrap(user?.userInfo);
	return unwrap(user?.loggedIn) === true || !!(info && !info.guest && (info.userId || info.user_id)) ||
		!!document.querySelector(selector);
}`

// isLoggedIn 页面正在跳转或尚未注水时读不到，按未登录处理，由调用方继续轮询。
func isLoggedIn(page *rod.Page) bool {
	res, err := page.Eval(loggedInJS, loggedInSelector)
	return err == nil && res.Value.Bool()
}

// openLoginPage 打开 loginPageURL，并等页面状态注水（最多 hydrate），期间已登录就提前返回 true。
func openLoginPage(page *rod.Page, hydrate time.Duration) (bool, error) {
	if err := page.Navigate(loginPageURL); err != nil {
		return false, errors.Wrap(err, "open login page failed")
	}
	deadline := time.Now().Add(hydrate)
	for time.Now().Before(deadline) {
		if isLoggedIn(page) {
			return true, nil
		}
		if err := page.GetContext().Err(); err != nil {
			return false, err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false, nil
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	// 加超时保护：只是查登录态的快速检查，不应无限挂（登录扫码的等待在 Login/WaitForLogin 里）
	pp := a.page.Context(ctx).Timeout(30 * time.Second)
	loggedIn, err := openLoginPage(pp, 5*time.Second)
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}
	return loggedIn, nil
}

// CurrentUser 当前登录用户的基础信息。
type CurrentUser struct {
	Nickname string `json:"nickname"`
	UserID   string `json:"userId"`
}

// CurrentUser 从当前页面的 __INITIAL_STATE__ 读取登录用户信息。
// 需在 CheckLoginStatus 之后调用：复用已加载的 loginPageURL 页，不做额外导航。
func (a *LoginAction) CurrentUser(ctx context.Context) (*CurrentUser, error) {
	pp := a.page.Context(ctx).Timeout(10 * time.Second)

	res, err := pp.Eval(`() => {
		const u = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user;
		const info = u && u.userInfo && u.userInfo.value !== undefined ? u.userInfo.value : (u && u.userInfo);
		if (!info || info.guest) return "";
		return JSON.stringify({nickname: info.nickname, userId: info.userId || info.user_id});
	}`)
	if err != nil {
		return nil, errors.Wrap(err, "read current user state failed")
	}

	raw := res.Value.String()
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
	pp := a.page.Context(ctx)

	// 未登录时页面会弹出二维码
	if loggedIn, err := openLoginPage(pp, 2*time.Second); err != nil || loggedIn {
		return err
	}
	if !a.WaitForLogin(ctx) {
		return errors.Wrap(ctx.Err(), "wait for login failed")
	}
	return nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)

	// 未登录时页面会弹出二维码
	if loggedIn, err := openLoginPage(pp, 2*time.Second); err != nil || loggedIn {
		return "", loggedIn, err
	}

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
			if isLoggedIn(pp) {
				return true
			}
		}
	}
}
