package xiaohongshu

import (
	"context"

	"github.com/go-rod/rod"
)

type NavigateAction struct {
	page *rod.Page
}

func NewNavigate(page *rod.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	page := n.page.Context(ctx)

	SafeNavigate(page, "https://www.xiaohongshu.com/explore").
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
	profileLink := page.MustElement(`div.main-container li.user.side-bar-component a.link-wrapper span.channel`)
	profileLink.MustClick()

	// Wait for navigation to complete
	page.MustWaitLoad()

	return nil
}

// SafeNavigate navigates to the given URL and immediately injects an async JS script
// to auto-click GDPR Cookie Consent dialogs. Returns the page so it can be chained.
func SafeNavigate(page *rod.Page, url string) *rod.Page {
	page.MustEvalOnNewDocument(`
		const interval = setInterval(() => {
			let btns = Array.from(document.querySelectorAll('button, div[role="button"], span'))
			let target = btns.find(el => {
				let text = el.textContent ? el.textContent.toLowerCase() : "";
				return text.includes('accept all cookies') || 
				       text.includes('agree and continue') ||
				       text.includes('accept cookies') ||
				       text.includes('同意') ||
				       text.includes('接受全部') ||
				       text.includes('允许');
			})
			if (target) {
				target.click();
				console.log('Cookie consent dialog auto-accepted');
				clearInterval(interval);
			}
		}, 200);
		setTimeout(() => clearInterval(interval), 10000);
	`)

	page.MustNavigate(url)

	return page
}

