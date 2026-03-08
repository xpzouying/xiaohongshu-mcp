package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

type FollowAction struct {
	page *rod.Page
}

func NewFollowAction(page *rod.Page) *FollowAction {
	pp := page.Timeout(60 * time.Second)
	return &FollowAction{page: pp}
}

func (f *FollowAction) FollowUser(ctx context.Context, userID, xsecToken string) error {
	page := f.page.Context(ctx)
	url := makeUserProfileURL(userID, xsecToken)
	logrus.Infof("关注用户: %s", userID)

	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("打开用户主页失败: %w", err)
	}
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// Find and click follow button
	btn, err := page.Element(`button.follow-btn, button[class*="follow"], .user-actions button`)
	if err != nil {
		return fmt.Errorf("未找到关注按钮: %w", err)
	}

	text, _ := btn.Text()
	logrus.Debugf("按钮文本: %s", text)

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击关注按钮失败: %w", err)
	}

	time.Sleep(2 * time.Second)
	logrus.Infof("关注用户成功: %s", userID)
	return nil
}

func (f *FollowAction) UnfollowUser(ctx context.Context, userID, xsecToken string) error {
	page := f.page.Context(ctx)
	url := makeUserProfileURL(userID, xsecToken)
	logrus.Infof("取消关注用户: %s", userID)

	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("打开用户主页失败: %w", err)
	}
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// Find the "已关注" button
	btn, err := page.Element(`button.followed-btn, button[class*="follow"], .user-actions button`)
	if err != nil {
		return fmt.Errorf("未找到取消关注按钮: %w", err)
	}

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击取消关注按钮失败: %w", err)
	}
	time.Sleep(1 * time.Second)

	// Handle confirmation dialog
	confirmBtn, err := page.Element(`button.confirm, .dialog button:last-child, [class*="confirm"]`)
	if err == nil {
		_ = confirmBtn.Click(proto.InputMouseButtonLeft, 1)
		time.Sleep(1 * time.Second)
	}

	logrus.Infof("取消关注用户成功: %s", userID)
	return nil
}

