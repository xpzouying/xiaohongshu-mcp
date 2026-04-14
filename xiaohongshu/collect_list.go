package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// CollectListResponse 收藏夹响应
type CollectListResponse struct {
	Feeds   []Feed `json:"feeds"`
	Count   int    `json:"count"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

// CollectListAction 负责获取用户收藏列表
type CollectListAction struct {
	page *rod.Page
}

func NewCollectListAction(page *rod.Page) *CollectListAction {
	pp := page.Timeout(90 * time.Second)
	return &CollectListAction{page: pp}
}

// GetCollectList 获取当前登录用户的收藏笔记列表
// 实现方式：导航到用户主页 -> 点击收藏 tab -> 从 DOM 和 __INITIAL_STATE__ 读取数据
func (a *CollectListAction) GetCollectList(ctx context.Context, cursor string, num int) (*CollectListResponse, error) {
	page := a.page.Context(ctx)

	if num <= 0 {
		num = 30
	}

	offset, err := parseListCursor(cursor)
	if err != nil {
		return nil, err
	}

	// 1. 导航到 explore 页面，触发登录态
	logrus.Info("CollectList: navigating to profile page")
	page.MustNavigate("https://www.xiaohongshu.com/explore")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 2. 获取当前登录用户 ID
	userID, err := getMyUserID(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	logrus.Infof("CollectList: got user ID: %s", userID)

	// 3. 直接导航到收藏 tab URL
	profileURL := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?tab=fav", userID)
	page.MustNavigate(profileURL)
	page.MustWaitDOMStable()
	time.Sleep(4 * time.Second)

	loadEnoughNoteItems(page, offset+num)

	// 5. 从 DOM 抓取笔记列表
	return a.extractFromDOM(page, offset, num)
}

// clickCollectTab 点击收藏 tab
func (a *CollectListAction) clickCollectTab(page *rod.Page) error {
	result := page.MustEval(`() => {
		const tabs = Array.from(document.querySelectorAll(".reds-tab-item"));
		const collectTab = tabs.find(t => t.textContent.trim() === "收藏");
		if (collectTab) {
			collectTab.click();
			return true;
		}
		return false;
	}`).Bool()

	if !result {
		return fmt.Errorf("collect tab not found")
	}
	logrus.Info("CollectList: clicked collect tab")
	return nil
}

// collectNote DOM 中的笔记节点结构
type collectNote struct {
	Title string `json:"title"`
	Link  string `json:"link"`
	Img   string `json:"img"`
}

type collectListDOMResult struct {
	Total int           `json:"total"`
	Items []collectNote `json:"items"`
}

// extractFromDOM 从 DOM 中提取收藏笔记列表
func (a *CollectListAction) extractFromDOM(page *rod.Page, offset, num int) (*CollectListResponse, error) {
	script := fmt.Sprintf(`() => {
		const items = Array.from(document.querySelectorAll(".note-item"));
		const offset = %d;
		const limit = %d;
		const selected = items.slice(offset, offset + limit).map(e => {
			const a = e.querySelector("a");
			const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
			const img = e.querySelector("img");
			return {
				title: titleEl ? titleEl.textContent.trim() : "",
				link: a ? a.href : "",
				img: img ? img.src : ""
			};
		});
		return JSON.stringify({
			total: items.length,
			items: selected
		});
	}`, offset, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" {
		return &CollectListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var payload collectListDOMResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse DOM notes: %w", err)
	}

	// 转换为 Feed 格式
	var feeds []Feed
	for _, n := range payload.Items {
		if n.Link == "" {
			continue
		}
		// 从链接提取 feed ID
		feedID := extractFeedID(n.Link)
		feed := Feed{
			ID: feedID,
			NoteCard: NoteCard{
				DisplayTitle: n.Title,
			},
		}
		feeds = append(feeds, feed)
	}

	nextOffset := offset + len(payload.Items)
	hasMore := payload.Total > nextOffset
	nextCursor := ""
	if hasMore {
		nextCursor = fmt.Sprintf("%d", nextOffset)
	}

	logrus.Infof("CollectList: extracted %d notes from DOM", len(feeds))
	return &CollectListResponse{
		Feeds:   feeds,
		Count:   len(feeds),
		Cursor:  nextCursor,
		HasMore: hasMore,
	}, nil
}
