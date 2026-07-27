package xiaohongshu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

const (
	followActionTimeout = 60 * time.Second
	pageStableWait      = 2 * time.Second
	clickEffectWait     = 1 * time.Second
)

// FollowAction handles follow/unfollow operations via browser automation.
type FollowAction struct {
	page *rod.Page
}

// NewFollowAction creates a new FollowAction with the given page.
func NewFollowAction(page *rod.Page) *FollowAction {
	return &FollowAction{page: page.Timeout(followActionTimeout)}
}

// FollowUser navigates to a user's profile and clicks the follow button.
func (f *FollowAction) FollowUser(ctx context.Context, userID, xsecToken string) error {
	return f.doFollowAction(ctx, userID, xsecToken, true)
}

// UnfollowUser navigates to a user's profile and clicks the unfollow button.
func (f *FollowAction) UnfollowUser(ctx context.Context, userID, xsecToken string) error {
	return f.doFollowAction(ctx, userID, xsecToken, false)
}

// doFollowAction is the shared implementation for follow and unfollow.
func (f *FollowAction) doFollowAction(ctx context.Context, userID, xsecToken string, follow bool) error {
	page := f.page.Context(ctx)
	url := makeUserProfileURL(userID, xsecToken)

	action := "关注"
	if !follow {
		action = "取消关注"
	}
	logrus.Infof("%s用户: %s", action, userID)

	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("打开用户主页失败: %w", err)
	}
	page.MustWaitDOMStable()
	time.Sleep(pageStableWait)

	// Find the follow/unfollow button in the user profile header area
	btn, err := findFollowButtonInProfile(page, follow)
	if err != nil {
		return fmt.Errorf("未找到%s按钮: %w", action, err)
	}

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击%s按钮失败: %w", action, err)
	}
	time.Sleep(clickEffectWait)

	// For unfollow, handle confirmation dialog
	if !follow {
		if err := dismissConfirmDialog(page); err != nil {
			logrus.Warnf("取消关注确认弹窗处理失败: %v", err)
		}
		time.Sleep(clickEffectWait)
	}

	// Verify the action succeeded
	if err := verifyFollowState(page, follow); err != nil {
		logrus.Warnf("%s操作可能未生效: %v", action, err)
	}

	logrus.Infof("%s用户成功: %s", action, userID)
	return nil
}

// findFollowButtonInProfile locates the follow/unfollow button within the
// user profile header area, avoiding buttons from recommendation lists.
func findFollowButtonInProfile(page *rod.Page, wantFollow bool) (*rod.Element, error) {
	// Try to find the button within the profile header first
	profileSelectors := []string{
		".user-info button",
		"[class*='profile'] button",
		"[class*='header'] button",
	}

	for _, selector := range profileSelectors {
		btn, err := findButtonByText(page, selector, wantFollow)
		if err == nil {
			return btn, nil
		}
	}

	// Fallback: search all buttons but prefer the first match
	// (the profile follow button typically appears before recommendation buttons)
	return findButtonByText(page, "button", wantFollow)
}

// findButtonByText searches for a follow/unfollow button matching the given selector and state.
func findButtonByText(page *rod.Page, selector string, wantFollow bool) (*rod.Element, error) {
	buttons, err := page.Elements(selector)
	if err != nil {
		return nil, err
	}

	for _, btn := range buttons {
		text, err := btn.Text()
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)

		if wantFollow && (text == "关注" || text == "+ 关注") {
			return btn, nil
		}
		if !wantFollow && (text == "已关注" || strings.Contains(text, "已关注")) {
			return btn, nil
		}
	}

	return nil, fmt.Errorf("未找到匹配的按钮")
}

// dismissConfirmDialog handles the unfollow confirmation popup.
func dismissConfirmDialog(page *rod.Page) error {
	buttons, err := page.Elements("button")
	if err != nil {
		return err
	}
	for _, btn := range buttons {
		text, err := btn.Text()
		if err != nil {
			continue
		}
		if strings.Contains(text, "确认") || strings.Contains(text, "确定") {
			return btn.Click(proto.InputMouseButtonLeft, 1)
		}
	}
	return nil
}

// verifyFollowState checks that the button state changed after the action.
func verifyFollowState(page *rod.Page, didFollow bool) error {
	buttons, err := page.Elements("button")
	if err != nil {
		return err
	}
	for _, btn := range buttons {
		text, err := btn.Text()
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if didFollow && (text == "已关注" || strings.Contains(text, "已关注")) {
			return nil
		}
		if !didFollow && (text == "关注" || text == "+ 关注") {
			return nil
		}
	}
	return fmt.Errorf("操作后未检测到预期的按钮状态变化")
}
