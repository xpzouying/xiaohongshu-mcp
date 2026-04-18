package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

// LoginStatusResult 登录状态结果
type LoginStatusResult struct {
	IsLoggedIn bool
	Nickname   string
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (*LoginStatusResult, error) {
	pp := a.page.Context(ctx)
	if err := pp.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return nil, errors.Wrap(err, "navigate to explore failed")
	}
	_ = pp.WaitLoad()

	time.Sleep(1 * time.Second)

	exists, _, err := pp.Has(`.main-container .user .link-wrapper .channel`)
	if err != nil {
		return nil, errors.Wrap(err, "check login status failed")
	}

	if !exists {
		return &LoginStatusResult{IsLoggedIn: false}, nil
	}

	// 点击侧边栏"我"导航到个人主页，从 __INITIAL_STATE__ 获取昵称
	nickname := ""
	profileLink, err := pp.Element(`div.main-container li.user.side-bar-component a.link-wrapper span.channel`)
	if err == nil && profileLink != nil {
		if clickErr := profileLink.Click(proto.InputMouseButtonLeft, 1); clickErr == nil {
			_ = pp.WaitLoad()
			time.Sleep(1 * time.Second)

			result, evalErr := pp.Eval(`() => {
				try {
					const user = window.__INITIAL_STATE__?.user;
					if (!user) return "";
					const data = user.userPageData?.value?.basicInfo ||
					             user.userPageData?._value?.basicInfo;
					return data?.nickname || "";
				} catch(e) { return ""; }
			}`)
			if evalErr == nil && result != nil {
				nickname = result.Value.String()
			}
		}
	}

	return &LoginStatusResult{IsLoggedIn: true, Nickname: nickname}, nil
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	if err := pp.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return errors.Wrap(err, "navigate to explore failed")
	}
	_ = pp.WaitLoad()

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

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	if err := pp.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return "", false, errors.Wrap(err, "navigate to explore failed")
	}
	_ = pp.WaitLoad()

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
