package xiaohongshu

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

const (
	exploreURL                 = "https://www.xiaohongshu.com/explore"
	securityVerificationPath   = "/website-login/captcha"
	loginNavigationTimeout     = 30 * time.Second
	loginQrcodeTimeout         = 4 * time.Minute
	verificationQrcodeTimeout  = time.Minute
	loggedInSelector           = ".main-container .user .link-wrapper .channel"
	loginQrcodeSelector        = ".login-container .qrcode-img"
	verificationQrcodeSelector = ".qrcode-img"
)

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

// navigate 只等待浏览器接受导航，不等待页面所有长连接结束。
func (a *LoginAction) navigate(ctx context.Context, targetURL string) (*rod.Page, error) {
	navigationCtx, cancel := context.WithTimeout(ctx, loginNavigationTimeout)
	defer cancel()

	if err := a.page.Context(navigationCtx).Navigate(targetURL); err != nil {
		return nil, errors.Wrap(err, "navigate to login page failed")
	}

	return a.page.Context(ctx), nil
}

func waitForInitialRender(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func qrcodePageConfig(pageURL string) (string, time.Duration) {
	if strings.Contains(pageURL, securityVerificationPath) {
		return verificationQrcodeSelector, verificationQrcodeTimeout
	}
	return loginQrcodeSelector, loginQrcodeTimeout
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	pp, err := a.navigate(ctx, exploreURL)
	if err != nil {
		return false, err
	}

	if err := waitForInitialRender(ctx, time.Second); err != nil {
		return false, errors.Wrap(err, "wait for login page render failed")
	}

	exists, _, err := pp.Has(loggedInSelector)
	if err != nil {
		return false, errors.Wrap(err, "check login status failed")
	}

	if !exists {
		return false, nil
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
	pp, err := a.navigate(ctx, exploreURL)
	if err != nil {
		return err
	}

	// 等待一小段时间让页面完全加载
	if err := waitForInitialRender(ctx, 2*time.Second); err != nil {
		return errors.Wrap(err, "wait for login page render failed")
	}

	// 检查是否已经登录
	if exists, _, _ := pp.Has(loggedInSelector); exists {
		// 已经登录，直接返回
		return nil
	}

	// 等待扫码成功提示或者登录完成
	// 这里我们等待登录成功的元素出现，这样更简单可靠
	if _, err := pp.Element(loggedInSelector); err != nil {
		return errors.Wrap(err, "wait for login failed")
	}

	return nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, time.Duration, error) {
	pp, err := a.navigate(ctx, exploreURL)
	if err != nil {
		return "", false, 0, err
	}

	// 等待一小段时间让页面完全加载
	if err := waitForInitialRender(ctx, 2*time.Second); err != nil {
		return "", false, 0, errors.Wrap(err, "wait for login page render failed")
	}

	// 检查是否已经登录
	if exists, _, _ := pp.Has(loggedInSelector); exists {
		return "", true, 0, nil
	}

	info, err := pp.Info()
	if err != nil {
		return "", false, 0, errors.Wrap(err, "get login page info failed")
	}
	selector, qrcodeTimeout := qrcodePageConfig(info.URL)

	// 获取二维码图片
	qrcodeCtx, cancel := context.WithTimeout(ctx, loginNavigationTimeout)
	defer cancel()

	qrcodeElement, err := pp.Context(qrcodeCtx).Element(selector)
	if err != nil {
		return "", false, 0, errors.Wrap(err, "find qrcode element failed")
	}

	src, err := qrcodeElement.Attribute("src")
	if err != nil {
		return "", false, 0, errors.Wrap(err, "get qrcode src failed")
	}
	if src == nil || len(*src) == 0 {
		return "", false, 0, errors.New("qrcode src is empty")
	}
	if !strings.HasPrefix(*src, "data:image/png;base64,") {
		return "", false, 0, errors.New("invalid qrcode image source")
	}

	return *src, false, qrcodeTimeout, nil
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
			el, err := pp.Element(loggedInSelector)
			if err == nil && el != nil {
				return true
			}
		}
	}
}
