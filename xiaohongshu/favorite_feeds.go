package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

type FavoriteFeedsAction struct {
	page *rod.Page
}

func NewFavoriteFeedsAction(page *rod.Page) *FavoriteFeedsAction {
	pp := page.Timeout(60 * time.Second)
	return &FavoriteFeedsAction{page: pp}
}

// GetFavoriteFeeds 获取当前登录用户的收藏笔记列表
func (f *FavoriteFeedsAction) GetFavoriteFeeds(ctx context.Context) ([]Feed, error) {
	page := f.page.Context(ctx)

	if err := navigateToFavoriteNoteTab(page); err != nil {
		return nil, err
	}

	profileURL := page.MustEval(`() => location.href`).String()
	favoriteURL, err := favoriteNoteTabURL(profileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build favorite note tab URL: %w", err)
	}

	page.MustNavigate(favoriteURL).MustWaitLoad().MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	result := page.MustEval(`() => {
		const feeds = [];
		const seen = new Set();
		const normalize = (s) => (s || "").replace(/\s+/g, " ").trim();

		const noteFromHref = (href) => {
			try {
				const u = new URL(href, location.origin);
				const m = u.pathname.match(/^\/explore\/([^/?#]+)/);
				if (!m || !m[1]) return null;
				return { id: m[1], xsecToken: u.searchParams.get("xsec_token") || u.searchParams.get("xsecToken") || "" };
			} catch {
				return null;
			}
		};

		const anchors = Array.from(document.querySelectorAll('a[href*="/explore/"]'));
		for (const a of anchors) {
			const parsed = noteFromHref(a.getAttribute("href") || "");
			if (!parsed || seen.has(parsed.id)) continue;
			seen.add(parsed.id);

			const card = a.closest("section, article, li, .note-item, .feeds-page, .note-card") || a.parentElement || a;
			const titleEl =
				card?.querySelector(".title span") ||
				card?.querySelector(".title") ||
				card?.querySelector(".desc") ||
				card?.querySelector("h3") ||
				card?.querySelector("h4");
			const authorEl =
				card?.querySelector(".author .name") ||
				card?.querySelector(".name") ||
				card?.querySelector('a[href*="/user/profile/"] span') ||
				card?.querySelector('a[href*="/user/profile/"]');

			feeds.push({
				id: parsed.id,
				xsecToken: parsed.xsecToken,
				modelType: "note",
				noteCard: {
					type: "normal",
					displayTitle: normalize(titleEl?.textContent || a.textContent || ""),
					user: {
						nickName: normalize(authorEl?.textContent || "")
					}
				},
				index: feeds.length
			});
		}

		return JSON.stringify(feeds);
	}`).String()

	if result == "" {
		debugInfo := page.MustEval(`() => {
			const state = window.__INITIAL_STATE__ || {};
			const user = state.user || null;
			const userKeys = user ? Object.keys(user).slice(0, 60) : [];

			const tabs = Array.from(document.querySelectorAll('span.channel, .reds-tab-item, .tab-item, div[class*="tab"]'))
				.map((n) => (n.textContent || "").trim())
				.filter(Boolean)
				.slice(0, 20);

			const hrefs = Array.from(document.querySelectorAll('a[href]'))
				.map((a) => a.getAttribute("href") || "")
				.filter((href) => href.includes("/explore/") || href.includes("/discovery/item/") || href.includes("collect"))
				.slice(0, 20);

			return JSON.stringify({
				url: location.href,
				title: document.title,
				hasInitialState: !!window.__INITIAL_STATE__,
				hasUser: !!user,
				userKeys,
				tabs,
				hrefs,
			});
		}`).String()

		if debugInfo == "" {
			return nil, errors.ErrNoFeeds
		}
		return nil, fmt.Errorf("%w; debug=%s", errors.ErrNoFeeds, debugInfo)
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal favorite feeds: %w", err)
	}

	return feeds, nil
}

func navigateToFavoriteNoteTab(page *rod.Page) error {
	page.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad().MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	profileTarget := page.MustEval(`() => {
		const selectors = [
			'div.main-container li.user.side-bar-component a.link-wrapper',
			'li.user.side-bar-component a',
			'a.link-wrapper[href*="/user/profile/"]'
		];
		for (const selector of selectors) {
			const node = document.querySelector(selector);
			if (!node) continue;
			const href = node.getAttribute("href");
			if (href && href.includes("/user/profile/")) {
				return new URL(href, location.origin).toString();
			}
			node.click();
			return "__clicked__";
		}
		return "";
	}`).String()

	if profileTarget == "" {
		return fmt.Errorf("failed to find profile link")
	}
	if profileTarget == "__clicked__" {
		page.MustWaitLoad().MustWaitDOMStable()
		time.Sleep(1200 * time.Millisecond)
		return nil
	}

	page.MustNavigate(profileTarget).MustWaitLoad().MustWaitDOMStable()
	time.Sleep(1200 * time.Millisecond)
	return nil
}

func favoriteNoteTabURL(profileURL string) (string, error) {
	u, err := url.Parse(profileURL)
	if err != nil {
		return "", err
	}
	u.RawQuery = "tab=fav&subTab=note"
	return u.String(), nil
}
