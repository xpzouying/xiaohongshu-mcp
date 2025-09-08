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

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
	pp := a.page.Context(ctx)

	// 导航到小红书探索页
	if err := pp.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return false, errors.Wrap(err, "导航到探索页失败")
	}

	// 等待页面加载完成
	if err := pp.WaitLoad(); err != nil {
		return false, errors.Wrap(err, "等待页面加载失败")
	}

	time.Sleep(1 * time.Second)

	exists, _, err := pp.Has(`.main-container .user .link-wrapper .channel`)
	if err != nil {
		return false, errors.Wrap(err, "检查登录状态元素失败")
	}

	if !exists {
		return false, nil // 未登录，但不是错误
	}

	return true, nil
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)

	// 导航到小红书首页，这会触发二维码弹窗
	if err := pp.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return errors.Wrap(err, "导航到探索页失败")
	}

	if err := pp.WaitLoad(); err != nil {
		return errors.Wrap(err, "等待页面加载失败")
	}

	// 等待一小段时间让页面完全加载
	time.Sleep(2 * time.Second)

	// 检查是否已经登录
	if exists, _, _ := pp.Has(".main-container .user .link-wrapper .channel"); exists {
		// 已经登录，直接返回
		return nil
	}

	// 等待扫码成功提示或者登录完成
	// 这里我们等待登录成功的元素出现，这样更简单可靠
	loginElem, err := pp.Element(".main-container .user .link-wrapper .channel")
	if err != nil {
		return errors.Wrap(err, "等待登录元素出现失败，可能是扫码超时或页面异常")
	}

	// 这里可以进一步检查元素是否可见
	if loginElem == nil {
		return errors.New("登录元素为空，登录可能失败")
	}

	return nil
}
