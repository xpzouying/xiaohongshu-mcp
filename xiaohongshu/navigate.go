package xiaohongshu

import (
	"context"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

type NavigateAction struct {
	page *rod.Page
}

func NewNavigate(page *rod.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	page := n.page.Context(ctx)

	page.MustNavigate(configs.ExploreURL()).
		MustWaitLoad().
		MustElement(`div#app`)

	return nil
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	page := n.page.Context(ctx)

	// First navigate to explore page
	if err := n.ToExplorePage(ctx); err != nil {
		return err
	}

	page.MustWaitStable()

	// Find and click the "我" channel link in sidebar
	// 注意：rednote.com 的侧边栏结构不同（div.bottom-menu-component），此选择器仅适用于 xiaohongshu.com
	profileLink := page.MustElement(`div.main-container li.user a.link-wrapper span.channel`)
	profileLink.MustClick()

	// Wait for navigation to complete
	page.MustWaitLoad()

	return nil
}
