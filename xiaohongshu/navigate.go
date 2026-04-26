package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

type NavigateAction struct {
	page *rod.Page
}

func NewNavigate(page *rod.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	logrus.Info("[Navigate] 导航到探索页面")
	page := n.page.Context(ctx)

	page.MustNavigate("https://www.xiaohongshu.com/explore").
		MustWaitLoad().
		MustElement(`div#app`)

	logrus.Info("[Navigate] 探索页面加载完成")
	return nil
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	logrus.Info("[Navigate] 开始导航到个人主页")
	page := n.page.Context(ctx)

	// First navigate to explore page
	logrus.Info("[Navigate] 步骤1: 先导航到探索页面")
	if err := n.ToExplorePage(ctx); err != nil {
		logrus.Errorf("[Navigate] 导航到探索页面失败: %v", err)
		return err
	}

	page.MustWaitStable()

	// 等待页面完全加载
	logrus.Info("[Navigate] 等待页面完全加载")
	time.Sleep(2 * time.Second)

	// 尝试多种选择器查找个人主页链接
	logrus.Info("[Navigate] 步骤2: 查找个人主页链接")
	selectors := []string{
		`div.main-container li.user.side-bar-component a.link-wrapper span.channel`,
		`[data-testid="profile-link"], [href*="/user/profile"], a[href*="/user/"], .user-profile-link`,
		`a[href*="/user"], button[class*="user"], span[class*="user"]`, // 更具体的用户相关元素
	}

	var profileLink *rod.Element
	for i, selector := range selectors {
		logrus.Infof("[Navigate] 尝试选择器 %d: %s", i+1, selector)
		if elem, err := page.Element(selector); err == nil && elem != nil {
			profileLink = elem
			logrus.Infof("[Navigate] 使用选择器 %d 找到个人主页链接", i+1)
			break
		}
		logrus.Warnf("[Navigate] 选择器 %d 未找到元素", i+1)
	}

	if profileLink == nil {
		logrus.Error("[Navigate] 所有选择器都未能找到个人主页链接")

		// 调试：截图当前页面状态
		if screenshot, err := page.Screenshot(false, nil); err == nil {
			logrus.Infof("[Navigate] 当前页面截图大小: %d bytes", len(screenshot))
		}

		// 获取页面HTML进行调试
		if html, err := page.HTML(); err == nil {
			logrus.Infof("[Navigate] 页面HTML长度: %d characters", len(html))
			// 记录关键部分用于调试
			if len(html) > 500 {
				logrus.Debugf("[Navigate] 页面头部HTML: %s", html[:500])
			}
		}

		return fmt.Errorf("profile link not found")
	}

	logrus.Info("[Navigate] 点击个人主页链接")
	profileLink.MustClick()

	// Wait for navigation to complete with longer timeout
	logrus.Info("[Navigate] 等待个人主页加载完成")
	page.MustWaitLoad()
	page.MustWaitStable()
	time.Sleep(3 * time.Second) // 额外等待确保页面完全加载

	logrus.Info("[Navigate] 成功导航到个人主页")
	return nil
}
