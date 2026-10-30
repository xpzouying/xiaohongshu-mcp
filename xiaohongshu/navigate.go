// Modified 2026-10-30 by Marvelooous: wait for navigation targets instead of unrelated page resources.
// SPDX-License-Identifier: Apache-2.0

package xiaohongshu

import (
	"context"

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
	page := n.page.Context(ctx).Timeout(pageReadyTimeout)
	return navigateToElement(page, "https://www.xiaohongshu.com/explore", `div#app .main-container`)
}

func (n *NavigateAction) ToProfilePage(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*pageReadyTimeout)
	defer cancel()
	page := n.page.Context(ctx)

	// First navigate to explore page
	if err := n.ToExplorePage(ctx); err != nil {
		return err
	}

	// Find and click the "我" channel link in sidebar
	profileLink, err := page.Element(`.main-container .user a.link-wrapper`)
	if err != nil {
		return err
	}
	humanize.Delay(ctx, humanize.BeforeClick)
	if err := humanize.Click(profileLink); err != nil {
		return err
	}

	// The profile is a SPA route; unrelated resources need not finish loading.
	return page.Wait(rod.Eval(`() => location.pathname.startsWith('/user/profile/')`))
}
