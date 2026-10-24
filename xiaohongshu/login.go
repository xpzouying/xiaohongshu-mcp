package xiaohongshu

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	// 加超时保护：只是查登录态的快速检查，不应无限挂（登录扫码的等待在 Login/WaitForLogin 里）
	pp := a.page.Context(ctx).Timeout(30 * time.Second)
	if err := pp.Navigate(webURL("/explore")); err != nil {
		return false, errors.Wrap(err, "navigate to explore page failed")
	}
	if err := pp.WaitLoad(); err != nil {
		return false, errors.Wrap(err, "wait for explore page failed")
	}
	return loginPageShowsSignedInUser(pp), nil
}

// CurrentUser 当前登录用户的基础信息。
type CurrentUser struct {
	Nickname string `json:"nickname"`
	UserID   string `json:"userId"`
}

// CurrentUser 从当前页面的 __INITIAL_STATE__ 读取登录用户信息。
// 需在 CheckLoginStatus 之后调用：复用已加载的 explore 页，不做额外导航。
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

	if err := pp.Navigate(webURL("/explore")); err != nil {
		return err
	}
	if err := pp.WaitLoad(); err != nil {
		return err
	}
	if loginPageShowsSignedInUser(pp) {
		return nil
	}
	if err := clickLoginControl(pp); err != nil {
		// 国内站可能已经自动打开登录弹窗。
		if exists, _, _ := pp.Has(".login-container .qrcode-img"); !exists {
			return err
		}
	}
	if !a.WaitForLogin(ctx) {
		return errors.Wrap(ctx.Err(), "等待登录结束")
	}
	return nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx).Timeout(30 * time.Second)

	if err := pp.Navigate(webURL("/explore")); err != nil {
		return "", false, errors.Wrap(err, "navigate to explore page failed")
	}
	if err := pp.WaitLoad(); err != nil {
		return "", false, errors.Wrap(err, "wait for explore page failed")
	}

	if loginPageShowsSignedInUser(pp) {
		return "", true, nil
	}

	qrSelector := ".login-container .qrcode-img"
	qr, err := pp.Timeout(2 * time.Second).Element(qrSelector)
	if err != nil {
		if loginPageShowsSignedInUser(pp) {
			return "", true, nil
		}
		// RedNote 不会自动打开登录弹窗，先点击登录。
		if err := clickLoginControl(pp); err != nil {
			return "", false, errors.Wrap(err, "login modal did not open")
		}
		qr, err = pp.Timeout(10 * time.Second).Element(qrSelector)
		if err != nil {
			return "", false, errors.Wrap(err, "login QR did not appear after opening login modal")
		}
	}

	src, err := qr.Attribute("src")
	if err != nil {
		return "", false, errors.Wrap(err, "get qrcode src failed")
	}
	if src == nil || len(*src) == 0 {
		return "", false, errors.New("qrcode src is empty")
	}

	return *src, false, nil
}

func loginPageShowsSignedInUser(page *rod.Page) bool {
	info, err := page.Info()
	if err != nil {
		return false
	}
	current, err := url.Parse(info.URL)
	if err != nil {
		return false
	}
	selected, _ := url.Parse(webURL(""))
	if current.Scheme != "https" || (current.Host != selected.Host && current.Host != strings.TrimPrefix(selected.Host, "www.")) {
		return false
	}
	if !isRedNote() {
		exists, _, err := page.Timeout(time.Second).Has(".main-container .user .link-wrapper .channel")
		return err == nil && exists
	}
	res, err := page.Eval(`() => {
		const user = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user;
		const flag = user && user.loggedIn;
		const loggedIn = flag && typeof flag === 'object' && 'value' in flag ? flag.value : flag;
		const info = user && user.userInfo;
		const value = info && info.value !== undefined ? info.value : info;
		return loggedIn !== false && !!(value && value.guest !== true &&
			typeof (value.userId || value.user_id) === 'string' &&
			(value.userId || value.user_id).trim());
	}`)
	return err == nil && res.Value.Bool()
}

func clickLoginControl(page *rod.Page) error {
	elems, err := page.Timeout(5 * time.Second).Elements("button, [role=button]")
	if err != nil {
		return err
	}
	for _, elem := range elems {
		text, err := elem.Text()
		if err != nil {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(text))
		if (label == "登录" || label == "登录/注册" || label == "log in" || label == "sign in") && humanize.Visible(elem) {
			return humanize.Click(elem)
		}
	}
	return errors.New("login control not found")
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
			if loginPageShowsSignedInUser(pp) {
				return true
			}
		}
	}
}
