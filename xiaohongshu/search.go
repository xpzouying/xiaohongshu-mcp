package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

type SearchResult struct {
	Search struct {
		Feeds FeedsValue `json:"feeds"`
	} `json:"search"`
}

type SearchAction struct {
	page *rod.Page
}

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)

	// 导航到搜索页面
	if err := page.Navigate(searchURL); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("导航到搜索页面失败，关键词: %s", keyword))
	}

	// 等待页面稳定
	if err := page.WaitStable(3 * time.Second); err != nil {
		return nil, errors.Wrap(err, "等待搜索页面稳定失败")
	}

	// 等待关键脚本加载，使用重试机制
	for i := 0; i < 30; i++ {
		result, err := page.Eval(`() => {
			return window.__INITIAL_STATE__ !== undefined;
		}`)
		if err == nil && result.Value.Bool() {
			break
		}
		if i == 29 {
			return nil, errors.New("等待搜索页面__INITIAL_STATE__加载超时")
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 获取 window.__INITIAL_STATE__ 并转换为 JSON 字符串
	resultObj, err := page.Eval(`() => {
			if (window.__INITIAL_STATE__) {
				return JSON.stringify(window.__INITIAL_STATE__);
			}
			return "";
		}`)
	if err != nil {
		return nil, errors.Wrap(err, "执行JavaScript获取搜索结果失败")
	}

	result := resultObj.Value.String()

	if result == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}

	var searchResult SearchResult
	if err := json.Unmarshal([]byte(result), &searchResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}

	return searchResult.Search.Feeds.Value, nil
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
