package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
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
	navigate := NewNavigate(page)

	if err := navigate.ToProfilePage(ctx); err != nil {
		return nil, fmt.Errorf("failed to navigate to profile page: %w", err)
	}

	page.MustWaitDOMStable()

	clicked := page.MustEval(`() => {
		const candidates = [
			'span.channel',
			'.reds-tab-item',
			'.tab-item',
			'div[class*="tab"]'
		];

		for (const selector of candidates) {
			const nodes = Array.from(document.querySelectorAll(selector));
			const target = nodes.find((n) => (n.textContent || "").trim() === "收藏");
			if (target) {
				target.click();
				return true;
			}
		}
		return false;
	}`).Bool()
	if !clicked {
		return nil, fmt.Errorf("failed to switch to favorite tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(800 * time.Millisecond)

	result := page.MustEval(`() => {
		const state = window.__INITIAL_STATE__;
		const user = state?.user;
		if (!user) return "";

		const extractValue = (v) => {
			if (v === null || v === undefined) return undefined;
			if (typeof v === "object") {
				if (v.value !== undefined) return v.value;
				if (v._value !== undefined) return v._value;
			}
			return v;
		};

		const candidates = [
			user.collectNotes,
			user.collectNote,
			user.collections,
			user.collects,
			user.favorites,
			user.favoriteNotes
		];

		let seenCandidate = false;

		const toFeed = (node) => {
			if (!node || typeof node !== "object") return null;
			const id = node.id;
			const xsecToken = node.xsecToken ?? node.xsec_token ?? node.xsec;
			if (!id || !xsecToken) return null;
			return {
				...node,
				xsecToken
			};
		};

		const walk = (node, out) => {
			const value = extractValue(node);
			if (value === undefined || value === null) return;
			if (Array.isArray(value)) {
				value.forEach((item) => walk(item, out));
				return;
			}
			if (typeof value !== "object") return;

			const feed = toFeed(value);
			if (feed) {
				out.push(feed);
				return;
			}

			Object.values(value).forEach((item) => walk(item, out));
		};

		for (const candidate of candidates) {
			if (candidate === undefined) continue;
			seenCandidate = true;

			const feeds = [];
			walk(candidate, feeds);
			if (feeds.length > 0) return JSON.stringify(feeds);
		}

		// Fallback: 从收藏页面可见卡片链接提取笔记信息，避免 __INITIAL_STATE__ 字段变化导致抓取失败
			const anchors = Array.from(document.querySelectorAll('a[href*="/explore/"], a[href*="/discovery/item/"], a[href*="/user/profile/"]'));
		const fallbackFeeds = [];
		const seen = new Set();

			const tryExtractID = (pathname) => {
				const m1 = pathname.match(/\/explore\/([^/?#]+)/);
				if (m1 && m1[1]) return m1[1];
				const m2 = pathname.match(/\/discovery\/item\/([^/?#]+)/);
				if (m2 && m2[1]) return m2[1];
				const m3 = pathname.match(/\/user\/profile\/[^/?#]+\/([^/?#]+)/);
				if (m3 && m3[1]) return m3[1];
				return "";
			};

		for (const a of anchors) {
			const href = a.getAttribute("href");
			if (!href) continue;

			let u;
			try {
				u = new URL(href, location.origin);
			} catch {
				continue;
			}

			const id = tryExtractID(u.pathname);
			const xsecToken =
				u.searchParams.get("xsec_token") ||
				u.searchParams.get("xsecToken") ||
				a.getAttribute("data-xsec-token") ||
				"";
			if (!id || !xsecToken) continue;

			const key = id + ":" + xsecToken;
			if (seen.has(key)) continue;
			seen.add(key);

			const card = a.closest("section, article, .note-item, .feeds-page, .note-card") || a.parentElement;
			const titleEl =
				card?.querySelector(".title span") ||
				card?.querySelector(".title") ||
				card?.querySelector(".desc") ||
				card?.querySelector("h3") ||
				card?.querySelector("h4");
			const title = ((titleEl?.textContent || a.textContent || "").trim()).slice(0, 200);

			fallbackFeeds.push({
				id,
				xsecToken,
				modelType: "note",
				noteCard: {
					type: "normal",
					displayTitle: title
				},
				index: fallbackFeeds.length
			});
		}

		if (fallbackFeeds.length > 0) return JSON.stringify(fallbackFeeds);

		if (seenCandidate) return "[]";
		return "";
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
