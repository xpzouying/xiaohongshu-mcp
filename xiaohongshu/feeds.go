package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

type FeedsListAction struct {
	page *rod.Page
}

// FeedsResult 定义页面初始状态结构
type FeedsResult struct {
	Feed FeedData `json:"feed"`
}

func NewFeedsListAction(page *rod.Page) (*FeedsListAction, error) {
	pp := page.Timeout(60 * time.Second)

	// 导航到小红书首页
	if err := pp.Navigate("https://www.xiaohongshu.com"); err != nil {
		return nil, errors.Wrap(err, "导航到小红书首页失败")
	}

	// 等待页面稳定
	if err := pp.WaitStable(3 * time.Second); err != nil {
		return nil, errors.Wrap(err, "等待页面稳定失败")
	}

	// 等待关键脚本加载，使用重试机制
	for i := 0; i < 30; i++ {
		result, err := pp.Eval(`() => {
			return window.__INITIAL_STATE__ !== undefined;
		}`)
		if err == nil && result.Value.Bool() {
			break
		}
		if i == 29 {
			return nil, errors.New("等待__INITIAL_STATE__加载超时，页面可能存在问题")
		}
		time.Sleep(500 * time.Millisecond)
	}

	return &FeedsListAction{page: pp}, nil
}

// GetFeedsList 获取页面的 Feed 列表数据
func (f *FeedsListAction) GetFeedsList(ctx context.Context) ([]Feed, error) {
	page := f.page.Context(ctx)

	// 获取 window.__INITIAL_STATE__ 并转换为 JSON 字符串
	resultObj, err := page.Eval(`() => {
		if (window.__INITIAL_STATE__) {
			return JSON.stringify(window.__INITIAL_STATE__);
		}
		return "";
	}`)
	if err != nil {
		return nil, errors.Wrap(err, "执行JavaScript获取__INITIAL_STATE__失败")
	}

	result := resultObj.Value.String()

	if result == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}

	// 解析完整的 InitialState
	var state FeedsResult
	if err := json.Unmarshal([]byte(result), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}

	// 返回 feed.feeds._value
	return state.Feed.Feeds.Value, nil
}
