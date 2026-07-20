package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

type SearchResult struct {
	Search struct {
		Feeds FeedsValue `json:"feeds"`
	} `json:"search"`
}

// FilterOption 筛选选项结构体
type FilterOption struct {
	SortBy      string `json:"sort_by,omitempty" jsonschema:"排序依据: 综合|最新|最多点赞|最多评论|最多收藏,默认为'综合'"`
	NoteType    string `json:"note_type,omitempty" jsonschema:"笔记类型: 不限|视频|图文,默认为'不限'"`
	PublishTime string `json:"publish_time,omitempty" jsonschema:"发布时间: 不限|一天内|一周内|半年内,默认为'不限'"`
	SearchScope string `json:"search_scope,omitempty" jsonschema:"搜索范围: 不限|已看过|未看过|已关注,默认为'不限'"`
	Location    string `json:"location,omitempty" jsonschema:"位置距离: 不限|同城|附近,默认为'不限'"`
}

// internalFilterOption 内部使用的筛选选项(基于索引)
type internalFilterOption struct {
	FiltersIndex int    // 筛选组索引
	TagsIndex    int    // 标签索引
	Text         string // 标签文本描述
}

// 预定义的筛选选项映射表（内部使用）
var filterOptionsMap = map[int][]internalFilterOption{
	1: { // 排序依据
		{FiltersIndex: 1, TagsIndex: 1, Text: "综合"},
		{FiltersIndex: 1, TagsIndex: 2, Text: "最新"},
		{FiltersIndex: 1, TagsIndex: 3, Text: "最多点赞"},
		{FiltersIndex: 1, TagsIndex: 4, Text: "最多评论"},
		{FiltersIndex: 1, TagsIndex: 5, Text: "最多收藏"},
	},
	2: { // 笔记类型
		{FiltersIndex: 2, TagsIndex: 1, Text: "不限"},
		{FiltersIndex: 2, TagsIndex: 2, Text: "视频"},
		{FiltersIndex: 2, TagsIndex: 3, Text: "图文"},
	},
	3: { // 发布时间
		{FiltersIndex: 3, TagsIndex: 1, Text: "不限"},
		{FiltersIndex: 3, TagsIndex: 2, Text: "一天内"},
		{FiltersIndex: 3, TagsIndex: 3, Text: "一周内"},
		{FiltersIndex: 3, TagsIndex: 4, Text: "半年内"},
	},
	4: { // 搜索范围
		{FiltersIndex: 4, TagsIndex: 1, Text: "不限"},
		{FiltersIndex: 4, TagsIndex: 2, Text: "已看过"},
		{FiltersIndex: 4, TagsIndex: 3, Text: "未看过"},
		{FiltersIndex: 4, TagsIndex: 4, Text: "已关注"},
	},
	5: { // 位置距离
		{FiltersIndex: 5, TagsIndex: 1, Text: "不限"},
		{FiltersIndex: 5, TagsIndex: 2, Text: "同城"},
		{FiltersIndex: 5, TagsIndex: 3, Text: "附近"},
	},
}

// convertToInternalFilters 将 FilterOption 转换为内部的 internalFilterOption 列表
func convertToInternalFilters(filter FilterOption) ([]internalFilterOption, error) {
	var internalFilters []internalFilterOption

	// 处理排序依据
	if filter.SortBy != "" {
		internal, err := findInternalOption(1, filter.SortBy)
		if err != nil {
			return nil, fmt.Errorf("排序依据错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理笔记类型
	if filter.NoteType != "" {
		internal, err := findInternalOption(2, filter.NoteType)
		if err != nil {
			return nil, fmt.Errorf("笔记类型错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理发布时间
	if filter.PublishTime != "" {
		internal, err := findInternalOption(3, filter.PublishTime)
		if err != nil {
			return nil, fmt.Errorf("发布时间错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理搜索范围
	if filter.SearchScope != "" {
		internal, err := findInternalOption(4, filter.SearchScope)
		if err != nil {
			return nil, fmt.Errorf("搜索范围错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理位置距离
	if filter.Location != "" {
		internal, err := findInternalOption(5, filter.Location)
		if err != nil {
			return nil, fmt.Errorf("位置距离错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	return internalFilters, nil
}

// findInternalOption 根据筛选组索引和文本查找内部筛选选项
func findInternalOption(filtersIndex int, text string) (internalFilterOption, error) {
	options, exists := filterOptionsMap[filtersIndex]
	if !exists {
		return internalFilterOption{}, fmt.Errorf("筛选组 %d 不存在", filtersIndex)
	}

	for _, option := range options {
		if option.Text == text {
			return option, nil
		}
	}

	return internalFilterOption{}, fmt.Errorf("在筛选组 %d 中未找到文本 '%s'", filtersIndex, text)
}

// validateInternalFilterOption 验证内部筛选选项是否在有效范围内
func validateInternalFilterOption(filter internalFilterOption) error {
	// 检查筛选组索引是否有效
	if filter.FiltersIndex < 1 || filter.FiltersIndex > 5 {
		return fmt.Errorf("无效的筛选组索引 %d，有效范围为 1-5", filter.FiltersIndex)
	}

	// 检查标签索引是否在对应筛选组的有效范围内
	options, exists := filterOptionsMap[filter.FiltersIndex]
	if !exists {
		return fmt.Errorf("筛选组 %d 不存在", filter.FiltersIndex)
	}

	if filter.TagsIndex < 1 || filter.TagsIndex > len(options) {
		return fmt.Errorf("筛选组 %d 的标签索引 %d 超出范围，有效范围为 1-%d",
			filter.FiltersIndex, filter.TagsIndex, len(options))
	}

	return nil
}

type SearchAction struct {
	page *rod.Page
}

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, limit int, filters ...FilterOption) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)
	page.MustNavigate(searchURL)
	page.MustWaitStable()

	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 如果有筛选条件，则应用筛选
	if len(filters) > 0 {
		// 将所有 FilterOption 转换为内部筛选选项
		var allInternalFilters []internalFilterOption
		for _, filter := range filters {
			internalFilters, err := convertToInternalFilters(filter)
			if err != nil {
				return nil, fmt.Errorf("筛选选项转换失败: %w", err)
			}
			allInternalFilters = append(allInternalFilters, internalFilters...)
		}

		// 验证所有内部筛选选项
		for _, filter := range allInternalFilters {
			if err := validateInternalFilterOption(filter); err != nil {
				return nil, fmt.Errorf("筛选选项验证失败: %w", err)
			}
		}

		// 悬停在筛选按钮上
		filterButton := page.MustElement(`div.filter`)
		filterButton.MustHover()

		// 等待筛选面板出现
		page.MustWait(`() => document.querySelector('div.filter-panel') !== null`)

		// 应用所有筛选条件
		for _, filter := range allInternalFilters {
			selector := fmt.Sprintf(`div.filter-panel div.filters:nth-child(%d) div.tags:nth-child(%d)`,
				filter.FiltersIndex, filter.TagsIndex)
			option := page.MustElement(selector)
			option.MustClick()
		}

		// 等待页面更新
		page.MustWaitStable()
		// 重新等待 __INITIAL_STATE__ 更新
		page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)
	}

	// 提取首屏 feeds
	feeds, err := extractSearchFeeds(page)
	if err != nil {
		return nil, err
	}

	// limit=0: 只返回首屏; limit>0: 加载到目标数量; limit<0: 无限滚动直到停滞
	if limit == 0 {
		return feeds, nil
	}
	if limit > 0 && len(feeds) >= limit {
		return feeds[:limit], nil
	}

	// 滚动加载更多
	feeds, err = s.scrollLoadMore(ctx, page, feeds, limit)
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(feeds) > limit {
		feeds = feeds[:limit]
	}
	return feeds, nil
}

// scrollLoadMore 通过滚动页面加载更多搜索结果
func (s *SearchAction) scrollLoadMore(ctx context.Context, page *rod.Page, feeds []Feed, limit int) ([]Feed, error) {
	const maxStaleRounds = 5 // 连续无新数据的最大轮数

	// 构建已有 feed ID 集合，用于去重
	seen := make(map[string]bool, len(feeds))
	for _, f := range feeds {
		seen[f.ID] = true
	}

	staleCount := 0

	for limit < 0 || len(feeds) < limit {
		// 检查 ctx 是否已取消，取消时返回已加载的部分数据
		if ctx.Err() != nil {
			logrus.Infof("搜索滚动加载被取消，已加载 %d 条数据", len(feeds))
			return feeds, nil
		}

		prevCount := len(feeds)

		// 鼠标移动到页面随机位置
		page.Mouse.MustMoveTo(float64(200+rand.Intn(600)), float64(300+rand.Intn(400)))

		// 每轮拆分为 2~4 次小滚动
		subScrolls := 2 + rand.Intn(3)
		for i := 0; i < subScrolls; i++ {
			deltaY := float64(300 + rand.Intn(400))
			page.Mouse.MustScroll(0, deltaY)
			sleepRandom(200, 500)
		}

		// 约 30% 概率向上回滚一小段，模拟真实浏览
		if rand.Intn(100) < 30 {
			upDelta := float64(-(50 + rand.Intn(100)))
			page.Mouse.MustScroll(0, upDelta)
			sleepRandom(100, 300)
		}

		// 翻页间隔：1~3 秒
		sleepRandom(1000, 3000)

		// 等待页面稳定后重新提取 feeds
		page.MustWaitStable()

		newFeeds, err := extractSearchFeeds(page)
		if err != nil {
			logrus.Warnf("滚动加载提取 feeds 失败: %v，返回已加载数据", err)
			return feeds, nil
		}

		// 去重：只追加新出现的 feed
		for _, f := range newFeeds {
			if !seen[f.ID] {
				seen[f.ID] = true
				feeds = append(feeds, f)
			}
		}

		logrus.Infof("搜索滚动加载: 当前 %d 条，目标 %d 条", len(feeds), limit)

		// 停滞检测
		if len(feeds) <= prevCount {
			staleCount++
			if staleCount >= maxStaleRounds {
				logrus.Infof("搜索滚动加载: 连续 %d 轮无新数据，已到底部", maxStaleRounds)
				return feeds, nil
			}
		} else {
			staleCount = 0
		}
	}

	return feeds, nil
}

// extractSearchFeeds 从页面提取搜索结果 feeds（通过 JS 读取 __INITIAL_STATE__），并按 ID 去重
func extractSearchFeeds(page *rod.Page) ([]Feed, error) {
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.search &&
		    window.__INITIAL_STATE__.search.feeds) {
			const feeds = window.__INITIAL_STATE__.search.feeds;
			const feedsData = feeds.value !== undefined ? feeds.value : feeds._value;
			if (feedsData) {
				return JSON.stringify(feedsData);
			}
		}
		return "";
	}`).String()

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var rawFeeds []Feed
	if err := json.Unmarshal([]byte(result), &rawFeeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}

	// 按 ID 去重
	seen := make(map[string]bool, len(rawFeeds))
	feeds := make([]Feed, 0, len(rawFeeds))
	for _, f := range rawFeeds {
		if !seen[f.ID] {
			seen[f.ID] = true
			feeds = append(feeds, f)
		}
	}

	return feeds, nil
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_search_result_notes
	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_explore_feed
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
