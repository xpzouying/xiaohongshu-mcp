package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// PublishedListResponse 已发布笔记响应
type PublishedListResponse struct {
	Feeds   []Feed `json:"feeds"`
	Count   int    `json:"count"`
	HasMore bool   `json:"has_more"`
}

// PublishedListAction 负责获取用户已发布笔记列表
type PublishedListAction struct {
	page *rod.Page
}

func NewPublishedListAction(page *rod.Page) *PublishedListAction {
	pp := page.Timeout(90 * time.Second)
	return &PublishedListAction{page: pp}
}

// GetPublishedList 获取当前登录用户自己发布的笔记列表
// URL: ?tab=note&subTab=note
func (a *PublishedListAction) GetPublishedList(ctx context.Context, num int) (*PublishedListResponse, error) {
	page := a.page.Context(ctx)

	if num <= 0 {
		num = 30
	}

	logrus.Info("PublishedList: navigating to explore")
	page.MustNavigate("https://www.xiaohongshu.com/explore")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	userID, err := getMyUserID(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	logrus.Infof("PublishedList: got user ID: %s", userID)

	// 直接导航到发布内容 tab URL
	profileURL := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?tab=note&subTab=note", userID)
	page.MustNavigate(profileURL)
	page.MustWaitDOMStable()
	time.Sleep(4 * time.Second)

	return a.extractFromDOM(page, num)
}

func (a *PublishedListAction) extractFromDOM(page *rod.Page, num int) (*PublishedListResponse, error) {
	script := fmt.Sprintf(`() => {
		const items = Array.from(document.querySelectorAll(".note-item"));
		return JSON.stringify(items.slice(0, %d).map(e => {
			const a = e.querySelector("a");
			const titleEl = e.querySelector("footer span, [class*=title], .title, a span");
			return {
				title: titleEl ? titleEl.textContent.trim() : "",
				link: a ? a.href : ""
			};
		}));
	}`, num)

	result := page.MustEval(script).String()
	if result == "" || result == "null" || result == "[]" {
		return &PublishedListResponse{Feeds: []Feed{}, Count: 0}, nil
	}

	var notes []struct {
		Title string `json:"title"`
		Link  string `json:"link"`
	}
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

	logrus.Infof("PublishedList: extracted %d notes", len(feeds))
	return &PublishedListResponse{Feeds: feeds, Count: len(feeds), HasMore: len(feeds) == num}, nil
}
