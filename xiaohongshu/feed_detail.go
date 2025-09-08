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

// NewFeedDetailAction 创建 Feed 详情页动作
func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// GetFeedDetail 获取 Feed 详情页数据
func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string) (*FeedDetailResponse, error) {
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

	// 将原始结果保存到 feed_detail.json 文件用于测试
	err := os.WriteFile("feed_detail.json", []byte(result), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write feed_detail.json: %w", err)
	}

	// 解析为通用的 map 结构
	var rawData map[string]any
	if err := json.Unmarshal([]byte(result), &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}

	// 提取结构化数据
	return f.extractFeedDetailData(rawData, feedID)
}

// extractFeedDetailData 从原始数据中提取结构化的 Feed 详情和评论数据
func (f *FeedDetailAction) extractFeedDetailData(rawData map[string]any, feedID string) (*FeedDetailResponse, error) {
	// 从 Vue 响应式数据中提取实际数据
	noteData, err := f.extractNestedValue(rawData, "note", "noteDetailMap", feedID, "note", "_value")
	if err != nil {
		return nil, fmt.Errorf("failed to extract note data: %w", err)
	}

	commentsData, err := f.extractNestedValue(rawData, "note", "noteDetailMap", feedID, "comments", "_value")
	if err != nil {
		// 评论数据可能不存在，不是致命错误
		commentsData = map[string]any{
			"list":    []any{},
			"hasMore": false,
		}
	}

	// 直接转换为结构化类型
	noteBytes, err := json.Marshal(noteData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal note data: %w", err)
	}
	var feedDetail FeedDetail
	if err := json.Unmarshal(noteBytes, &feedDetail); err != nil {
		return nil, fmt.Errorf("failed to unmarshal note data to FeedDetail: %w", err)
	}

	commentsBytes, err := json.Marshal(commentsData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal comments data: %w", err)
	}
	var commentList CommentList
	if err := json.Unmarshal(commentsBytes, &commentList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal comments data to CommentList: %w", err)
	}

	return &FeedDetailResponse{
		Note:     feedDetail,
		Comments: commentList,
	}, nil
}

// extractNestedValue 从嵌套的 map 结构中提取值
func (f *FeedDetailAction) extractNestedValue(data map[string]any, keys ...string) (any, error) {
	current := data
	for i, key := range keys {
		if current == nil {
			return nil, fmt.Errorf("nil value at key path: %v", keys[:i])
		}

		value, exists := current[key]
		if !exists {
			return nil, fmt.Errorf("key '%s' not found at path: %v", key, keys[:i+1])
		}

		if i == len(keys)-1 {
			// 最后一个 key，返回值
			return value, nil
		}

		// 继续深入下一层
		if nextMap, ok := value.(map[string]any); ok {
			current = nextMap
		} else {
			return nil, fmt.Errorf("expected map[string]any at key '%s', got %T", key, value)
		}
	}

	return nil, fmt.Errorf("no keys provided")
}
