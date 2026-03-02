package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// GetFavoriteCategories 获取收藏下的专辑分类列表
func (f *FavoriteFeedsAction) GetFavoriteCategories(ctx context.Context) ([]FavoriteCategory, error) {
	page := f.page.Context(ctx)
	if err := f.navigateToFavoriteBoardTab(ctx, page); err != nil {
		return nil, err
	}

	result := page.MustEval(`() => {
		const anchors = Array.from(document.querySelectorAll('a[href*="/board/"]'));
		const map = new Map();

		const parseBoardId = (href) => {
			try {
				const u = new URL(href, location.origin);
				const m = u.pathname.match(/\/board\/([^/?#]+)/);
				return m && m[1] ? m[1] : "";
			} catch {
				return "";
			}
		};

		for (const a of anchors) {
			const href = a.getAttribute("href") || "";
			const id = parseBoardId(href);
			if (!id) continue;

			const card = a.closest("section, article, li, .board-item, .album-item, .note-item") || a;
			const text = (card.textContent || a.textContent || "").replace(/\s+/g, " ").trim();
			let name = text;
			let count = 0;
			const strict = text.match(/^(.+?)\\s*笔记・\\s*(\\d+)/);
			if (strict) {
				name = strict[1].trim();
				count = Number(strict[2]) || 0;
			} else {
				const countMatch = text.match(/笔记・(\\d+)/);
				if (countMatch) {
					count = Number(countMatch[1]) || 0;
					name = text.slice(0, countMatch.index).trim();
				}
			}
			if (!name) name = "未命名专辑";
			name = name.replace(/\s*笔记[・·]\s*\d+.*$/, "").trim() || name;

			if (!map.has(id)) {
				map.set(id, {
					id,
					name,
					noteCount: count,
					url: new URL(href, location.origin).toString(),
				});
			}
		}

		return JSON.stringify(Array.from(map.values()));
	}`).String()

	var categories []FavoriteCategory
	if err := json.Unmarshal([]byte(result), &categories); err != nil {
		return nil, fmt.Errorf("failed to unmarshal favorite categories: %w", err)
	}

	return categories, nil
}

// GetFavoriteFeedsByCategory 获取指定收藏专辑下的笔记
func (f *FavoriteFeedsAction) GetFavoriteFeedsByCategory(ctx context.Context, categoryID, categoryName string, limit int) ([]Feed, *FavoriteCategory, error) {
	page := f.page.Context(ctx)
	if err := f.navigateToFavoriteBoardTab(ctx, page); err != nil {
		return nil, nil, err
	}

	targetResult := page.MustEval(`(args) => {
		const { categoryID, categoryName } = args;
		const anchors = Array.from(document.querySelectorAll('a[href*="/board/"]'));

		const parseBoardId = (href) => {
			try {
				const u = new URL(href, location.origin);
				const m = u.pathname.match(/\/board\/([^/?#]+)/);
				return m && m[1] ? m[1] : "";
			} catch {
				return "";
			}
		};

		const normalize = (s) => (s || "").replace(/\s+/g, " ").trim().toLowerCase();
		const normalizedName = normalize(categoryName);

		for (const a of anchors) {
			const href = a.getAttribute("href") || "";
			const id = parseBoardId(href);
			if (!id) continue;

			const card = a.closest("section, article, li, .board-item, .album-item, .note-item") || a;
			const text = (card.textContent || a.textContent || "").replace(/\s+/g, " ").trim();
			let name = text;
			let count = 0;
			const strict = text.match(/^(.+?)\\s*笔记・\\s*(\\d+)/);
			if (strict) {
				name = strict[1].trim();
				count = Number(strict[2]) || 0;
			} else {
				const countMatch = text.match(/笔记・(\\d+)/);
				if (countMatch) {
					count = Number(countMatch[1]) || 0;
					name = text.slice(0, countMatch.index).trim();
				}
			}
			if (!name) name = "未命名专辑";
			name = name.replace(/\s*笔记[・·]\s*\d+.*$/, "").trim() || name;

			const idMatch = categoryID && id === categoryID;
			const normalizedCandidate = normalize(name);
			const nameMatch = normalizedName && (normalizedCandidate === normalizedName || normalizedCandidate.startsWith(normalizedName) || normalizedCandidate.includes(normalizedName));
			const fallback = !categoryID && !normalizedName;
			if (!(idMatch || nameMatch || fallback)) continue;

			return JSON.stringify({
				id,
				name,
				noteCount: count,
				url: new URL(href, location.origin).toString(),
				clickHref: href,
			});
		}

		return "";
	}`, map[string]any{
		"categoryID":   categoryID,
		"categoryName": categoryName,
	}).String()

	if targetResult == "" {
		return nil, nil, fmt.Errorf("favorite category not found (id=%q, name=%q)", categoryID, categoryName)
	}

	var target struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		NoteCount int    `json:"noteCount"`
		URL       string `json:"url"`
		ClickHref string `json:"clickHref"`
	}
	if err := json.Unmarshal([]byte(targetResult), &target); err != nil {
		return nil, nil, fmt.Errorf("failed to parse target category: %w", err)
	}

	page.MustNavigate(target.URL).MustWaitLoad().MustWaitDOMStable()
	time.Sleep(800 * time.Millisecond)

	feedResult := page.MustEval(`(limit) => {
		const feeds = [];
		const seen = new Set();

		const extractValue = (v) => {
			if (v === null || v === undefined) return undefined;
			if (typeof v === "object") {
				if (v.value !== undefined) return v.value;
				if (v._value !== undefined) return v._value;
			}
			return v;
		};

		const toFeed = (node) => {
			if (!node || typeof node !== "object") return null;
			const id = node.id || node.noteId || node.note_id;
			const xsecToken = node.xsecToken ?? node.xsec_token ?? node.xsec;
			if (!id || !xsecToken) return null;
			return {
				...node,
				id,
				xsecToken,
				modelType: node.modelType || "note",
				noteCard: node.noteCard || node,
			};
		};

		const walkState = (node) => {
			const value = extractValue(node);
			if (value === undefined || value === null) return;
			if (Array.isArray(value)) {
				for (const item of value) walkState(item);
				return;
			}
			if (typeof value !== "object") return;

			const feed = toFeed(value);
			if (feed) {
				const key = feed.id + ":" + feed.xsecToken;
				if (!seen.has(key)) {
					seen.add(key);
					feed.index = feeds.length;
					feeds.push(feed);
				}
				return;
			}

			for (const v of Object.values(value)) walkState(v);
		};

		// 优先从 board 状态读取，通常包含完整 xsecToken
		const state = window.__INITIAL_STATE__ || {};
		const board = state.board || {};
		const boardCandidates = [
			board.boardFeedsMap,
			board.boardDetails,
			board.boardListData,
		];
		for (const c of boardCandidates) {
			walkState(c);
			if (limit > 0 && feeds.length >= limit) break;
		}
		if (feeds.length > 0) {
			if (limit > 0) return JSON.stringify(feeds.slice(0, limit));
			return JSON.stringify(feeds);
		}

		const parseID = (u) => {
			const p = u.pathname || "";
			const m1 = p.match(/\/user\/profile\/[^/?#]+\/([^/?#]+)/);
			if (m1 && m1[1]) return m1[1];
			const m2 = p.match(/\/explore\/([^/?#]+)/);
			if (m2 && m2[1]) return m2[1];
			const m3 = p.match(/\/discovery\/item\/([^/?#]+)/);
			if (m3 && m3[1]) return m3[1];
			return "";
		};

		const anchors = Array.from(document.querySelectorAll('a[href*="/user/profile/"], a[href*="/explore/"], a[href*="/discovery/item/"]'));
		for (const a of anchors) {
			const href = a.getAttribute("href") || "";
			let u;
			try { u = new URL(href, location.origin); } catch { continue; }
			const id = parseID(u);
			const xsecToken =
				u.searchParams.get("xsec_token") ||
				u.searchParams.get("xsecToken") ||
				a.getAttribute("data-xsec-token") ||
				"";
			if (!id || !xsecToken) continue;

			const key = id + ":" + xsecToken;
			if (seen.has(key)) continue;
			seen.add(key);

			const card = a.closest("section, article, li, .note-item, .feeds-page, .note-card") || a.parentElement;
			const titleEl =
				card?.querySelector(".title span") ||
				card?.querySelector(".title") ||
				card?.querySelector(".desc") ||
				card?.querySelector("h3") ||
				card?.querySelector("h4");
			const title = ((titleEl?.textContent || a.textContent || "").trim()).slice(0, 200);

			feeds.push({
				id,
				xsecToken,
				modelType: "note",
				noteCard: { type: "normal", displayTitle: title },
				index: feeds.length,
			});

			if (limit > 0 && feeds.length >= limit) break;
		}

		return JSON.stringify(feeds);
	}`, limit).String()

	var feeds []Feed
	if err := json.Unmarshal([]byte(feedResult), &feeds); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal category feeds: %w", err)
	}

	category := &FavoriteCategory{
		ID:        target.ID,
		Name:      target.Name,
		NoteCount: target.NoteCount,
		URL:       target.URL,
	}

	return feeds, category, nil
}

func (f *FavoriteFeedsAction) navigateToFavoriteBoardTab(ctx context.Context, page *rod.Page) error {
	navigate := NewNavigate(page)
	if err := navigate.ToProfilePage(ctx); err != nil {
		return fmt.Errorf("failed to navigate to profile page: %w", err)
	}
	page.MustWaitDOMStable()

	clickedFav := page.MustEval(`() => {
		const sels=['span.channel','.reds-tab-item','.tab-item','div[class*="tab"]'];
		for(const s of sels){
			const n=[...document.querySelectorAll(s)].find(x=>(x.textContent||'').trim()==='收藏');
			if(n){n.click();return true;}
		}
		return false;
	}`).Bool()
	if !clickedFav {
		return fmt.Errorf("failed to switch to favorite tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(500 * time.Millisecond)

	clickedBoard := page.MustEval(`() => {
		const sels=['span.channel','.reds-tab-item','.tab-item','div[class*="tab"]'];
		for(const s of sels){
			const nodes=[...document.querySelectorAll(s)];
			const n=nodes.find(x=>(x.textContent||'').trim().startsWith('专辑'));
			if(n){n.click();return true;}
		}
		// already in subTab=board also consider success
		return location.search.includes('subTab=board');
	}`).Bool()
	if !clickedBoard {
		return fmt.Errorf("failed to switch to board tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(700 * time.Millisecond)
	return nil
}
