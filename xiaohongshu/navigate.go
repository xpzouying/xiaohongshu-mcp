package xiaohongshu

import (
	"context"

	"github.com/playwright-community/playwright-go"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

type NavigateAction struct {
	page playwright.Page
}

func NewNavigate(page playwright.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	if _, err := n.page.Goto("https://www.xiaohongshu.com/explore", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60_000),
	}); err != nil {
		return err
	}
	// 等应用容器挂载
	_, err := n.page.WaitForSelector(`div#app`, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(60_000),
	})
	return err
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	if err := n.ToExplorePage(ctx); err != nil {
		return err
	}

	// 等 SPA 稳定再点侧边栏「我」
	if err := n.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		// networkidle 不一定达成，降级为继续
		logActionError("wait networkidle on explore", err)
	}

	profileLink, err := n.page.QuerySelector(`div.main-container li.user.side-bar-component a.link-wrapper span.channel`)
	if err != nil || profileLink == nil {
		return err
	}
	humanize.Delay(ctx, humanize.BeforeClick)
	if err := humanize.Click(profileLink); err != nil {
		return err
	}

	return n.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateLoad,
	})
}
