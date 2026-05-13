package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var (
	categoryNameCountRegexp = regexp.MustCompile(`^(.+?)\s*笔记[・·]\s*(\d+)`)
	categoryCountRegexp     = regexp.MustCompile(`笔记[・·]\s*(\d+)`)
)

// GetFavoriteCategories 获取收藏下的专辑分类列表
func (f *FavoriteFeedsAction) GetFavoriteCategories(ctx context.Context) ([]FavoriteCategory, error) {
	page := f.page.Context(ctx)
	if err := f.navigateToFavoriteBoardTab(ctx, page); err != nil {
		return nil, err
	}

	return f.collectFavoriteCategoriesFromPage(page)
}

func (f *FavoriteFeedsAction) collectFavoriteCategoriesFromPage(page *rod.Page) ([]FavoriteCategory, error) {
	anchors, err := page.Elements(`a[href*="/board/"]`)
	if err != nil {
		return nil, fmt.Errorf("failed to find favorite category links: %w", err)
	}

	categories := make([]FavoriteCategory, 0, len(anchors))
	seen := make(map[string]struct{}, len(anchors))
	for _, anchor := range anchors {
		href, ok := readElementAttr(anchor, "href")
		if !ok {
			continue
		}

		category, ok := buildFavoriteCategoryFromBoardLink(anchor, href)
		if !ok {
			continue
		}
		if _, exists := seen[category.ID]; exists {
			continue
		}

		seen[category.ID] = struct{}{}
		categories = append(categories, category)
	}

	return categories, nil
}

func buildFavoriteCategoryFromBoardLink(anchor *rod.Element, href string) (FavoriteCategory, bool) {
	link, err := parseXHSURL(href)
	if err != nil {
		return FavoriteCategory{}, false
	}

	categoryID := extractBoardIDFromPath(link.Path)
	if categoryID == "" {
		return FavoriteCategory{}, false
	}

	text, err := anchor.Text()
	if err != nil {
		text = ""
	}
	name, noteCount := parseFavoriteCategoryText(text)

	return FavoriteCategory{
		ID:        categoryID,
		Name:      name,
		NoteCount: noteCount,
		URL:       link.String(),
	}, true
}

func parseXHSURL(rawURL string) (*url.URL, error) {
	link, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if link.IsAbs() {
		return link, nil
	}

	base, _ := url.Parse("https://www.xiaohongshu.com")
	return base.ResolveReference(link), nil
}

func extractBoardIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "board" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func parseFavoriteCategoryText(text string) (string, int) {
	text = strings.Join(strings.Fields(text), " ")
	name := text
	noteCount := 0

	if matches := categoryNameCountRegexp.FindStringSubmatch(text); len(matches) == 3 {
		name = strings.TrimSpace(matches[1])
		noteCount, _ = strconv.Atoi(matches[2])
	} else if matches := categoryCountRegexp.FindStringSubmatch(text); len(matches) == 2 {
		noteCount, _ = strconv.Atoi(matches[1])
		name = strings.TrimSpace(text[:strings.Index(text, matches[0])])
	}

	if name == "" {
		name = "未命名专辑"
	}
	return name, noteCount
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

	if !clickTabByText(page, "收藏") {
		return fmt.Errorf("failed to switch to favorite tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(500 * time.Millisecond)

	if !clickTabByTextPrefix(page, "专辑") {
		return fmt.Errorf("failed to switch to board tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(700 * time.Millisecond)
	return nil
}

func clickTabByTextPrefix(page *rod.Page, tabTextPrefix string) bool {
	selectors := []string{"span.channel", ".reds-tab-item", ".tab-item", "div[class*=\"tab\"]"}
	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.Text()
			if err != nil || !strings.HasPrefix(strings.TrimSpace(text), tabTextPrefix) {
				continue
			}

			if err := element.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return true
			}
		}
	}

	return false
}