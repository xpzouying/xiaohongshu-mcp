package xiaohongshu

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	xhserrors "github.com/xpzouying/xiaohongshu-mcp/errors"
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

func collectInternalFilters(filters []FilterOption) ([]internalFilterOption, error) {
	var allInternalFilters []internalFilterOption
	for _, filter := range filters {
		internalFilters, err := convertToInternalFilters(filter)
		if err != nil {
			return nil, err
		}
		allInternalFilters = append(allInternalFilters, internalFilters...)
	}

	for _, filter := range allInternalFilters {
		if err := validateInternalFilterOption(filter); err != nil {
			return nil, err
		}
	}

	return allInternalFilters, nil
}

type SearchAction struct {
	page *rod.Page
}

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	var feeds []Feed
	var searchErr error
	err := recoverSearchPanic(func() {
		feeds, searchErr = s.search(ctx, keyword, filters...)
	})
	if err != nil {
		return nil, err
	}
	if searchErr != nil {
		return nil, searchErr
	}

	return feeds, nil
}

func (s *SearchAction) search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)
	logrus.Infof("搜索Feeds: 打开搜索页 url=%s", searchURL)
	page.Timeout(30 * time.Second).MustNavigate(searchURL)

	logrus.Info("搜索Feeds: 等待搜索页初始数据")
	page.Timeout(20 * time.Second).MustWait(`() => {
		return window.__INITIAL_STATE__ !== undefined &&
			window.__INITIAL_STATE__.search !== undefined &&
			window.__INITIAL_STATE__.search.feeds !== undefined;
	}`)
	page.Timeout(15 * time.Second).MustWait(`() => {
		const feeds = window.__INITIAL_STATE__?.search?.feeds;
		const feedsData = feeds?.value !== undefined ? feeds.value : feeds?._value;
		return Array.isArray(feedsData) && feedsData.length > 0;
	}`)

	allInternalFilters, err := collectInternalFilters(filters)
	if err != nil {
		return nil, fmt.Errorf("筛选选项转换失败: %w", err)
	}

	// 如果有实际筛选条件，则应用筛选
	if len(allInternalFilters) > 0 {
		logrus.Infof("搜索Feeds: 应用筛选条件数量=%d", len(allInternalFilters))

		// 应用所有筛选条件
		for _, filter := range allInternalFilters {
			// 筛选面板由 hover 触发，点击后可能收起；每次点击前都重新展开。
			logrus.Debug("搜索Feeds: 查找筛选按钮")
			filterButton := page.Timeout(5 * time.Second).MustElement(`div.filter`)
			filterButton.MustHover()

			logrus.Debug("搜索Feeds: 等待筛选面板")
			page.Timeout(5 * time.Second).MustWait(`() => document.querySelector('div.filter-panel') !== null`)

			selector := fmt.Sprintf(`div.filter-panel div.filters:nth-child(%d) div.tags:not([aria-hidden="true"])`,
				filter.FiltersIndex)
			logrus.Infof("搜索Feeds: 查找可见筛选项 %s selector=%s index=%d", filter.Text, selector, filter.TagsIndex)
			options := page.Timeout(5 * time.Second).MustElements(selector)
			if len(options) < filter.TagsIndex {
				return nil, fmt.Errorf("筛选项 %s 不存在: selector=%s index=%d visibleCount=%d",
					filter.Text, selector, filter.TagsIndex, len(options))
			}
			option := options[filter.TagsIndex-1]
			logrus.Infof("搜索Feeds: 点击筛选项 %s", filter.Text)
			option.MustClick()
			logrus.Infof("搜索Feeds: 已点击筛选项 %s", filter.Text)
		}

		// 点击筛选后页面会异步刷新数据，避免等待整页 stable 或依赖易变的 loading 状态。
		time.Sleep(1500 * time.Millisecond)
	} else {
		logrus.Info("搜索Feeds: 无实际筛选条件，跳过筛选面板")
	}

	logrus.Info("搜索Feeds: 读取 feeds 数据")
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
		logSearchState(page)
		return nil, xhserrors.ErrNoFeeds
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}
	logrus.Infof("搜索Feeds: 读取 feeds 完成 count=%d", len(feeds))

	return feeds, nil
}

func logSearchState(page *rod.Page) {
	state := page.Timeout(2 * time.Second).MustEval(`() => JSON.stringify({
		url: location.href,
		title: document.title,
		bodyText: document.body?.innerText?.slice(0, 300) ?? "",
		hasInitialState: !!window.__INITIAL_STATE__,
		searchKeys: Object.keys(window.__INITIAL_STATE__?.search ?? {}),
		feedsType: typeof (window.__INITIAL_STATE__?.search?.feeds),
		feedsValueType: typeof (window.__INITIAL_STATE__?.search?.feeds?.value),
		feedsValueLength: Array.isArray(window.__INITIAL_STATE__?.search?.feeds?.value)
			? window.__INITIAL_STATE__.search.feeds.value.length
			: null,
		feedsRawLength: Array.isArray(window.__INITIAL_STATE__?.search?.feeds?._value)
			? window.__INITIAL_STATE__.search.feeds._value.length
			: null,
	})`).String()
	logrus.Warnf("搜索Feeds: 未捕获到 feeds 数据，页面状态=%s", state)
}

func recoverSearchPanic(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if panicErr, ok := r.(error); ok {
				if stderrors.Is(panicErr, context.Canceled) || stderrors.Is(panicErr, context.DeadlineExceeded) {
					err = panicErr
					return
				}
				err = fmt.Errorf("搜索页面操作失败: %w", panicErr)
				return
			}

			panic(r)
		}
	}()

	fn()
	return nil
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_search_result_notes
	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_explore_feed
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
