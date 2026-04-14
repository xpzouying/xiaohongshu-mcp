package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// LikedListResponse 点赞笔记响应
type LikedListResponse struct {
	Feeds   []Feed `json:"feeds"`
	Count   int    `json:"count"`
	HasMore bool   `json:"has_more"`
}

// LikedListAction 负责获取用户点赞列表
type LikedListAction struct {
	page *rod.Page
}

func NewLikedListAction(page *rod.Page) *LikedListAction {
	pp := page.Timeout(90 * time.Second)
	return &LikedListAction{page: pp}
}

// GetLikedList 获取当前登录用户点赞的笔记列表
func (a *LikedListAction) GetLikedList(ctx context.Context, num int) (*LikedListResponse, error) {
	page := a.page.Context(ctx)

	if num <= 0 {
		num = 30
	}

	logrus.Info("LikedList: navigating to explore")
	page.MustNavigate("https://www.xiaohongshu.com/explore")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 获取当前登录用户 ID
	userID, err := getMyUserID(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	logrus.Infof("LikedList: got user ID: %s", userID)

	// 导航到用户主页，等 SPA 完全初始化
	profileURL := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s", userID)
	page.MustNavigate(profileURL)
	page.MustWaitDOMStable()
	time.Sleep(5 * time.Second)

	// 点击点赞 tab
	page.MustEval(`() => {
		const t = Array.from(document.querySelectorAll(".reds-tab-item")).find(t => t.textContent.trim() === "点赞");
		if (t) t.dispatchEvent(new MouseEvent("click", {bubbles: true, cancelable: true}));
	}`)
	logrus.Info("LikedList: clicked 点赞 tab")
	time.Sleep(4 * time.Second)

	// 滚动加载更多内容
	page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
	time.Sleep(2 * time.Second)
	page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
	time.Sleep(2 * time.Second)

	return a.extractAllDeduped(page, num)
}

type likedNote struct {
	Title string `json:"title"`
	Link  string `json:"link"`
	Img   string `json:"img"`
}

// extractNewItems 提取点赞 tab 新出现的内容
// 策略：去除所有重复链接后取新出现的条目
func (a *LikedListAction) extractNewItems(page *rod.Page, noteLinksJSON string, num int) (*LikedListResponse, error) {
	script := fmt.Sprintf(`() => {
		const noteLinks = %s;
		const noteSet = new Set(noteLinks);
		const seenLinks = new Set();
		const items = Array.from(document.querySelectorAll(".note-item"));
		const newItems = [];
		for (const e of items) {
			const a = e.querySelector("a");
			if (!a || seenLinks.has(a.href)) continue;
			seenLinks.add(a.href);
			if (!noteSet.has(a.href)) {
				const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
				newItems.push({
					title: titleEl ? titleEl.textContent.trim() : "",
					link: a.href
				});
			}
			if (newItems.length >= %d) break;
		}
		return JSON.stringify(newItems);
	}`, noteLinksJSON, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" || result == "[]" {
		logrus.Warn("LikedList: no new items found, falling back")
		return &LikedListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var notes []likedNote
	if err := json.Unmarshal([]byte(result), &notes); err != nil {
		return nil, fmt.Errorf("failed to parse liked notes: %w", err)
	}

	var feeds []Feed
	for _, n := range notes {
		if n.Link == "" {
			continue
		}
		feeds = append(feeds, Feed{ID: extractFeedID(n.Link), NoteCard: NoteCard{DisplayTitle: n.Title}})
	}

	logrus.Infof("LikedList: extracted %d liked notes (deduped)", len(feeds))
	return &LikedListResponse{Feeds: feeds, Count: len(feeds), HasMore: len(feeds) == num}, nil
}

// extractLikedOnly 从点赞 tab 中提取非发布笔记的内容

// extractAllDeduped 获取点赞 tab 中所有去重的笔记
func (a *LikedListAction) extractAllDeduped(page *rod.Page, num int) (*LikedListResponse, error) {
	script := fmt.Sprintf(`() => {
		const seen = new Set();
		const result = [];
		for (const e of document.querySelectorAll(".note-item")) {
			const a = e.querySelector("a");
			if (!a || seen.has(a.href)) continue;
			seen.add(a.href);
			const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
			result.push({
				title: titleEl ? titleEl.textContent.trim() : "",
				link: a.href
			});
			if (result.length >= %d) break;
		}
		return JSON.stringify(result);
	}`, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" || result == "[]" {
		return &LikedListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var notes []likedNote
	if err := json.Unmarshal([]byte(result), &notes); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	var feeds []Feed
	for _, n := range notes {
		if n.Link == "" {
			continue
		}
		feeds = append(feeds, Feed{ID: extractFeedID(n.Link), NoteCard: NoteCard{DisplayTitle: n.Title}})
	}

	logrus.Infof("LikedList: extracted %d liked notes (all deduped)", len(feeds))
	return &LikedListResponse{Feeds: feeds, Count: len(feeds), HasMore: len(feeds) == num}, nil
}

func (a *LikedListAction) extractLikedOnly(page *rod.Page, publishedLinksJSON string, num int) (*LikedListResponse, error) {
	script := fmt.Sprintf(`() => {
		const published = new Set(%s);
		const seen = new Set();
		const result = [];
		for (const e of document.querySelectorAll(".note-item")) {
			const a = e.querySelector("a");
			if (!a || seen.has(a.href)) continue;
			seen.add(a.href);
			if (published.has(a.href)) continue;
			const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
			result.push({
				title: titleEl ? titleEl.textContent.trim() : "",
				link: a.href
			});
			if (result.length >= %d) break;
		}
		return JSON.stringify(result);
	}`, publishedLinksJSON, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" || result == "[]" {
		logrus.Warn("LikedList: no liked items found")
		return &LikedListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var notes []likedNote
	if err := json.Unmarshal([]byte(result), &notes); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	var feeds []Feed
	for _, n := range notes {
		if n.Link == "" {
			continue
		}
		feeds = append(feeds, Feed{ID: extractFeedID(n.Link), NoteCard: NoteCard{DisplayTitle: n.Title}})
	}

	logrus.Infof("LikedList: extracted %d liked notes", len(feeds))
	return &LikedListResponse{Feeds: feeds, Count: len(feeds), HasMore: len(feeds) == num}, nil
}

func (a *LikedListAction) extractFromDOMOffset(page *rod.Page, offset int, num int) (*LikedListResponse, error) {
	script := fmt.Sprintf(`() => {
		const items = Array.from(document.querySelectorAll(".note-item"));
		const total = items.length;
		let start = %d;
		const limit = %d;
		// 如果 offset 超出范围，从末尾往前取 limit 条
		if (start >= total) start = Math.max(0, total - limit);
		return JSON.stringify(items.slice(start, start + limit).map(e => {
			const a = e.querySelector("a");
			const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
			const img = e.querySelector("img");
			return {
				title: titleEl ? titleEl.textContent.trim() : "",
				link: a ? a.href : "",
				img: img ? img.src : ""
			};
		}));
	}`, offset, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" || result == "[]" {
		return &LikedListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var notes []likedNote
	if err := json.Unmarshal([]byte(result), &notes); err != nil {
		return nil, fmt.Errorf("failed to parse DOM notes: %w", err)
	}

	var feeds []Feed
	for _, n := range notes {
		if n.Link == "" {
			continue
		}
		feedID := extractFeedID(n.Link)
		feed := Feed{ID: feedID, NoteCard: NoteCard{DisplayTitle: n.Title}}
		feeds = append(feeds, feed)
	}

	logrus.Infof("LikedList: extracted %d notes from DOM (offset %d)", len(feeds), offset)
	return &LikedListResponse{Feeds: feeds, Count: len(feeds), HasMore: len(feeds) == num}, nil
}

func (a *LikedListAction) extractFromDOM(page *rod.Page, num int) (*LikedListResponse, error) {
	script := fmt.Sprintf(`() => {
		const items = Array.from(document.querySelectorAll(".note-item"));
		const limit = %d;
		return JSON.stringify(items.slice(0, limit).map(e => {
			const a = e.querySelector("a");
			const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
			const img = e.querySelector("img");
			return {
				title: titleEl ? titleEl.textContent.trim() : "",
				link: a ? a.href : "",
				img: img ? img.src : ""
			};
		}));
	}`, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" || result == "[]" {
		return &LikedListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var notes []likedNote
	if err := json.Unmarshal([]byte(result), &notes); err != nil {
		return nil, fmt.Errorf("failed to parse DOM notes: %w", err)
	}

	var feeds []Feed
	for _, n := range notes {
		if n.Link == "" {
			continue
		}
		feedID := extractFeedID(n.Link)
		feed := Feed{
			ID: feedID,
			NoteCard: NoteCard{
				DisplayTitle: n.Title,
			},
		}
		feeds = append(feeds, feed)
	}

	logrus.Infof("LikedList: extracted %d notes from DOM", len(feeds))
	return &LikedListResponse{
		Feeds:   feeds,
		Count:   len(feeds),
		HasMore: len(feeds) == num,
	}, nil
}
