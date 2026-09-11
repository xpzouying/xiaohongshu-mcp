package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
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
	// explore 页资源多，load 事件常迟迟不触发；div#app 出现才是 SPA 真正就绪的标志。
	softWaitLoad(page, "explore 页")
	page.MustElement(`div#app`)

	return nil
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	page := n.page.Context(ctx).Timeout(60 * time.Second) // 加超时保护，避免 MustNavigate/MustWaitStable 无限挂

	// First navigate to explore page
	if err := n.ToExplorePage(ctx); err != nil {
		return err
	}

	softWaitStable(page, "个人页-侧边栏")

	// Find and click the "我" channel link in sidebar
	profileLink := page.MustElement(`div.main-container li.user.side-bar-component a.link-wrapper span.channel`)
	humanize.Delay(ctx, humanize.BeforeClick)
	if err := humanize.Click(profileLink); err != nil {
		return err
	}

	// Wait for navigation to complete
	softWaitLoad(page, "个人页-点击跳转后")

	return nil
}
