package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
)

// FeedDetailAction 表示 Feed 详情页动作
type FeedDetailAction struct {
	page *rod.Page
}

// FeedDetailResult 表示 Feed 详情页的 __INITIAL_STATE__ 结构
type FeedDetailResult struct {
	// 这里会在获取到实际数据后进一步定义结构
	Data map[string]any `json:",inline"`
}

// NewFeedDetailAction 创建 Feed 详情页动作
func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// GetFeedDetail 获取 Feed 详情页数据
func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string) (*FeedDetailResult, error) {
	page := f.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitStable()
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 获取 window.__INITIAL_STATE__ 并转换为 JSON 字符串
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__) {
			return JSON.stringify(window.__INITIAL_STATE__);
		}
		return "";
	}`).String()

	if result == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}

	// 将原始结果保存到 feed_detail.json 文件
	err := os.WriteFile("feed_detail.json", []byte(result), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write feed_detail.json: %w", err)
	}

	// 解析为通用的 map 结构
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}

	return &FeedDetailResult{Data: data}, nil
}
