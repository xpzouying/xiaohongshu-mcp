package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

type NavigateAction struct {
	page *rod.Page
}

func NewNavigate(page *rod.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	page := n.page.Context(ctx).Timeout(60 * time.Second) // 加超时保护，避免 MustNavigate/MustWaitStable 无限挂

	page.MustNavigate("https://www.xiaohongshu.com/explore")
	waitLoadTolerant(page, 15*time.Second)
	page.MustElement(`div#app`)

	return nil
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	page := n.page.Context(ctx).Timeout(60 * time.Second) // 加超时保护，避免 MustNavigate/MustWaitStable 无限挂

	// First navigate to explore page
	if err := n.ToExplorePage(ctx); err != nil {
		return err
	}

	if err := page.WaitStable(time.Second); err != nil {
		logrus.Warnf("explore 页未稳定，继续点击 profile 入口: %v", err)
	}

	// Find and click the "我" channel link in sidebar
	profileLink := page.MustElement(`div.main-container li.user.side-bar-component a.link-wrapper span.channel`)
	humanize.Delay(ctx, humanize.BeforeClick)
	if err := humanize.Click(profileLink); err != nil {
		return err
	}

	// Wait for navigation to complete
	waitLoadTolerant(page, 15*time.Second)

	return nil
}
