package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
)

type SearchResult struct {
	Search struct {
		Feeds FeedsValue `json:"feeds"`
	} `json:"search"`
}

type FilterOption struct {
	FiltersIndex int    `json:"filters_index"` // 筛选组索引 (1-based): 1=排序依据, 2=笔记类型, 3=发布时间, 4=搜索范围, 5=位置距离
	TagsIndex    int    `json:"tags_index"`    // 标签索引 (1-based): 1=不限, 2=第一个选项, 3=第二个选项...
	Text         string `json:"text"`          // 标签文本描述，用于说明
}

// 预定义的筛选选项映射表
var FilterOptionsMap = map[int][]FilterOption{
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

// validateFilterOption 验证筛选选项是否在有效范围内
func validateFilterOption(filter FilterOption) error {
	// 检查筛选组索引是否有效
	if filter.FiltersIndex < 1 || filter.FiltersIndex > 5 {
		return fmt.Errorf("无效的筛选组索引 %d，有效范围为 1-5", filter.FiltersIndex)
	}

	// 检查标签索引是否在对应筛选组的有效范围内
	options, exists := FilterOptionsMap[filter.FiltersIndex]
	if !exists {
		return fmt.Errorf("筛选组 %d 不存在", filter.FiltersIndex)
	}

	if filter.TagsIndex < 1 || filter.TagsIndex > len(options) {
		return fmt.Errorf("筛选组 %d 的标签索引 %d 超出范围，有效范围为 1-%d",
			filter.FiltersIndex, filter.TagsIndex, len(options))
	}

	return nil
}

// 便利函数：根据文本创建筛选选项
func NewFilterOption(filtersIndex int, text string) (FilterOption, error) {
	options, exists := FilterOptionsMap[filtersIndex]
	if !exists {
		return FilterOption{}, fmt.Errorf("筛选组 %d 不存在", filtersIndex)
	}

	for _, option := range options {
		if option.Text == text {
			return option, nil
		}
	}

	return FilterOption{}, fmt.Errorf("在筛选组 %d 中未找到文本 '%s'", filtersIndex, text)
}

// 便利函数：创建常用的筛选选项
func SortBy(text string) (FilterOption, error) {
	return NewFilterOption(1, text) // 排序依据
}

func NoteType(text string) (FilterOption, error) {
	return NewFilterOption(2, text) // 笔记类型
}

func TimeRange(text string) (FilterOption, error) {
	return NewFilterOption(3, text) // 发布时间
}

func SearchScope(text string) (FilterOption, error) {
	return NewFilterOption(4, text) // 搜索范围
}

func LocationDistance(text string) (FilterOption, error) {
	return NewFilterOption(5, text) // 位置距离
}

type SearchAction struct {
	page *rod.Page
}

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)
	page.MustNavigate(searchURL)
	page.MustWaitStable()

	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 如果有筛选条件，则应用筛选
	if len(filters) > 0 {
		// 验证所有筛选选项
		for _, filter := range filters {
			if err := validateFilterOption(filter); err != nil {
				return nil, fmt.Errorf("筛选选项验证失败: %w", err)
			}
		}

		// 悬停在筛选按钮上
		filterButton := page.MustElement(`div.filter`)
		filterButton.MustHover()

		// 等待筛选面板出现
		page.MustWait(`() => document.querySelector('div.filter-panel') !== null`)

		// 应用所有筛选条件
		for _, filter := range filters {
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

	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_search_result_notes
	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_explore_feed
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
