package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

type FeedsListAction struct {
	page playwright.Page
}

func NewFeedsListAction(page playwright.Page) *FeedsListAction {
	if _, err := page.Goto("https://www.xiaohongshu.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60_000),
	}); err != nil {
		logActionError("navigate to xiaohongshu home", err)
	}
	// 等 __INITIAL_STATE__ 注水；超时由 GetFeedsList 的轮询兜底
	_, _ = page.WaitForFunction(`() => window.__INITIAL_STATE__ !== undefined`, nil,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(15_000)})

	return &FeedsListAction{page: page}
}

// GetFeedsList 获取页面的 Feed 列表数据
func (f *FeedsListAction) GetFeedsList(ctx context.Context) ([]Feed, error) {
	page := f.page

	readFeeds := func() string {
		res, err := page.Evaluate(`() => {
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
		}`)
		if err != nil {
			return ""
		}
		s, _ := res.(string)
		return s
	}

	// 轮询等 __INITIAL_STATE__.feed 注水就绪（替代固定 1s，治偶发 ErrNoFeeds）
	var result string
	deadline := time.Now().Add(8 * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

	return onlyNotes(feeds), nil
}
