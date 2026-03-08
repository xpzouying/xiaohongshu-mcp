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
	logrus.Infof("打开用户主页进行关注: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	if err := checkPageAccessible(page); err != nil {
		return err
	}

	followStatus := page.MustEval(`() => {
		const btn = document.querySelector('.reds-button-new.follow-button, .follow-button');
		if (!btn) return 'not_found';
		const text = btn.textContent.trim();
		if (text.includes('已关注') || text.includes('互相关注')) return 'following';
		if (text.includes('回关') || text.includes('关注')) return 'not_following';
		return text;
	}`).String()

	logrus.Infof("当前关注状态: %s", followStatus)

	if followStatus == "following" {
		return fmt.Errorf("已经关注了该用户")
	}
	if followStatus == "not_found" {
		return fmt.Errorf("未找到关注按钮")
	}

	// 获取所有可能的按钮元素信息
	btnInfo := page.MustEval(`() => {
		const selectors = [
			'.user-actions .follow-button',
			'.info-part .follow-button', 
			'button.follow-btn',
			'.follow-state',
			'.user-actions button',
			'.info-part button',
			'button[class*="follow"]',
			'div[class*="follow"]',
			'.reds-button',
		];
		const results = [];
		for (const sel of selectors) {
			const els = document.querySelectorAll(sel);
			els.forEach(el => {
				results.push({
					selector: sel,
					tag: el.tagName,
					className: el.className,
					text: el.textContent.trim().substring(0, 50),
					html: el.outerHTML.substring(0, 200),
				});
			});
		}
		return JSON.stringify(results);
	}`).String()
	logrus.Infof("页面按钮信息: %s", btnInfo)

	followBtn, err := page.Element(".reds-button-new.follow-button, .follow-button")
	if err != nil {
		return fmt.Errorf("未找到关注按钮: %w", err)
	}

	// Log exact button info
	btnHTML, _ := followBtn.HTML()
	logrus.Infof("即将点击的按钮: %s", func() string { if len(btnHTML) > 200 { return btnHTML[:200] } else { return btnHTML } }())

	// 先用 JavaScript 触发 click（处理 Vue 事件）
	_, evalErr := page.Eval(`() => {
		const btn = document.querySelector('.reds-button-new.follow-button, .follow-button');
		if (btn) {
			btn.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true, view: window}));
			return true;
		}
		return false;
	}`)
	if evalErr != nil {
		logrus.Warnf("JS click failed: %v, falling back to rod click", evalErr)
		if err := followBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return fmt.Errorf("点击关注按钮失败: %w", err)
		}
	}

	time.Sleep(2 * time.Second)

	newStatus := page.MustEval(`() => {
		const btn = document.querySelector('.reds-button-new.follow-button, .follow-button');
		if (!btn) return 'unknown';
		const text = btn.textContent.trim();
		if (text.includes('已关注') || text.includes('互相关注')) return 'following';
		if (text.includes('关注')) return 'following';
		return text;
	}`).String()

	if newStatus == "following" {
		logrus.Infof("关注用户成功: %s", userID)
		return nil
	}
	return fmt.Errorf("关注可能未成功，当前状态: %s", newStatus)
}

func (f *FollowAction) UnfollowUser(ctx context.Context, userID, xsecToken string) error {
	page := f.page.Context(ctx)
	url := makeUserProfileURL(userID, xsecToken)
	logrus.Infof("打开用户主页取消关注: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	if err := checkPageAccessible(page); err != nil {
		return err
	}

	followStatus := page.MustEval(`() => {
		const btn = document.querySelector('.reds-button-new.follow-button, .follow-button');
		if (!btn) return 'not_found';
		const text = btn.textContent.trim();
		if (text.includes('已关注') || text.includes('互相关注')) return 'following';
		return 'not_following';
	}`).String()

	if followStatus == "not_following" {
		return fmt.Errorf("当前未关注该用户")
	}
	if followStatus == "not_found" {
		return fmt.Errorf("未找到关注按钮")
	}

	followBtn, err := page.Element(".reds-button-new.follow-button, .follow-button")
	if err != nil {
		return fmt.Errorf("未找到关注按钮: %w", err)
	}

	if err := followBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击取消关注失败: %w", err)
	}

	time.Sleep(1 * time.Second)

	// 确认弹窗
	confirmBtn, err := page.Timeout(3*time.Second).Element(".modal-confirm, .confirm-btn, .unfollow-confirm")
	if err == nil && confirmBtn != nil {
		logrus.Info("检测到确认弹窗")
		confirmBtn.Click(proto.InputMouseButtonLeft, 1)
		time.Sleep(1 * time.Second)
	}

	logrus.Infof("取消关注操作完成: %s", userID)
	return nil
}
