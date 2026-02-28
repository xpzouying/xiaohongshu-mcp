package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

// 支持英文/简写/序号，降低调用方编码或参数风格差异导致的失败概率
var filterOptionAliases = map[int]map[string]string{
	1: { // sort_by
		"1":              "综合",
		"2":              "最新",
		"3":              "最多点赞",
		"4":              "最多评论",
		"5":              "最多收藏",
		"all":            "综合",
		"default":        "综合",
		"comprehensive":  "综合",
		"latest":         "最新",
		"most_likes":     "最多点赞",
		"most_liked":     "最多点赞",
		"most_comments":  "最多评论",
		"most_commented": "最多评论",
		"most_favorites": "最多收藏",
		"most_favorited": "最多收藏",
	},
	2: { // note_type
		"1":          "不限",
		"2":          "视频",
		"3":          "图文",
		"all":        "不限",
		"default":    "不限",
		"video":      "视频",
		"image_text": "图文",
		"image-text": "图文",
		"text_image": "图文",
	},
	3: { // publish_time
		"1":          "不限",
		"2":          "一天内",
		"3":          "一周内",
		"4":          "半年内",
		"all":        "不限",
		"default":    "不限",
		"any":        "不限",
		"any_time":   "不限",
		"one_day":    "一天内",
		"one_week":   "一周内",
		"half_year":  "半年内",
		"halfyear":   "半年内",
	},
	4: { // search_scope
		"1":         "不限",
		"2":         "已看过",
		"3":         "未看过",
		"4":         "已关注",
		"all":       "不限",
		"default":   "不限",
		"viewed":    "已看过",
		"unviewed":  "未看过",
		"followed":  "已关注",
		"following": "已关注",
	},
	5: { // location
		"1":       "不限",
		"2":       "同城",
		"3":       "附近",
		"all":     "不限",
		"default": "不限",
		"local":   "同城",
		"nearby":  "附近",
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

	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return internalFilterOption{}, fmt.Errorf("筛选组 %d 的筛选值不能为空", filtersIndex)
	}

	if aliases, ok := filterOptionAliases[filtersIndex]; ok {
		if canonical, exists := aliases[strings.ToLower(normalized)]; exists {
			normalized = canonical
		}
	}

	for _, option := range options {
		if option.Text == normalized {
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

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)
	page.MustNavigate(searchURL)
	page.MustWaitStable()

	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 如果有筛选条件，则尝试应用筛选（无效筛选将被忽略）
	if len(filters) > 0 {
		var allInternalFilters []internalFilterOption
		for _, filter := range filters {
			internalFilters, err := convertToInternalFilters(filter)
			if err != nil {
				logrus.WithError(err).WithField("filter", filter).Warn("筛选选项转换失败，已忽略该筛选")
				continue
			}
			allInternalFilters = append(allInternalFilters, internalFilters...)
		}

		validFilters := make([]internalFilterOption, 0, len(allInternalFilters))
		for _, filter := range allInternalFilters {
			if err := validateInternalFilterOption(filter); err != nil {
				logrus.WithError(err).WithField("filter", filter).Warn("筛选选项验证失败，已忽略该筛选")
				continue
			}
			validFilters = append(validFilters, filter)
		}

		// 仅在至少有一个有效筛选条件时操作筛选面板，避免空筛选导致额外等待
		if len(validFilters) > 0 {
			// 悬停在筛选按钮上
			filterButton := page.MustElement(`div.filter`)
			filterButton.MustHover()

			// 等待筛选面板出现
			page.MustWait(`() => document.querySelector('div.filter-panel') !== null`)

			// 应用所有筛选条件
			for _, filter := range validFilters {
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
	}

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

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
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
