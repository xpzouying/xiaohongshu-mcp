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

		if (seenCandidate) return "[]";
		return "";
	}`).String()

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal favorite feeds: %w", err)
	}

	return feeds, nil
}
