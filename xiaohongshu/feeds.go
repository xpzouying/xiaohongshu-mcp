package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

type FeedsListAction struct {
	page *rod.Page
}

func NewFeedsListAction(page *rod.Page) *FeedsListAction {
	pp := page.Timeout(60 * time.Second)

	pp.MustNavigate("https://www.xiaohongshu.com")
	pp.MustWaitDOMStable()

	return &FeedsListAction{page: pp}
}

// GetFeedsList 获取页面的 Feed 列表数据
func (f *FeedsListAction) GetFeedsList(ctx context.Context) ([]Feed, error) {
	// 重设超时：.Context(ctx) 会替换掉构造函数里 Timeout(60s) 的 deadline
	page := f.page.Context(ctx).Timeout(60 * time.Second)

	readFeeds := func() string {
		return page.MustEval(`() => {
			if (window.__INITIAL_STATE__ &&
			    window.__INITIAL_STATE__.feed &&
			    window.__INITIAL_STATE__.feed.feeds) {
				const feeds = window.__INITIAL_STATE__.feed.feeds;
				const feedsData = feeds.value !== undefined ? feeds.value : feeds._value;
				if (feedsData) {
					return JSON.stringify(feedsData);
				}
			}
			return "";
		}`).String()
	}

	// 轮询等 __INITIAL_STATE__.feed 注水就绪（替代固定 1s，治偶发 ErrNoFeeds）
	var result string
	deadline := time.Now().Add(8 * time.Second)
	for {
		if result = readFeeds(); result != "" || time.Now().After(deadline) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}

	return feeds, nil
}
