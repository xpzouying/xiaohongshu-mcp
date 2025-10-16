package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

type FeedsListAction struct {
	page *rod.Page
}

// FeedsResult 定义页面初始状态结构
type FeedsResult struct {
	Feed FeedData `json:"feed"`
}

func NewFeedsListAction(page *rod.Page) *FeedsListAction {
	pp := page.Timeout(60 * time.Second)

	pp.MustNavigate("https://www.xiaohongshu.com")
	pp.MustWaitDOMStable()

	return &FeedsListAction{page: pp}
}

// GetFeedsList 获取页面的 Feed 列表数据
func (f *FeedsListAction) GetFeedsList(ctx context.Context) ([]Feed, error) {
	page := f.page.Context(ctx)

	time.Sleep(1 * time.Second)

	// 直接获取 window.__INITIAL_STATE__.feed.feeds.value 避免循环引用
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.feed &&
		    window.__INITIAL_STATE__.feed.feeds &&
		    window.__INITIAL_STATE__.feed.feeds.value) {
			return JSON.stringify(window.__INITIAL_STATE__.feed.feeds.value);
		}
		return "";
	}`).String()

	if result == "" {
		return nil, fmt.Errorf("feed.feeds.value not found in __INITIAL_STATE__")
	}

	// 直接解析为 Feed 数组
	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}

	return feeds, nil
}
