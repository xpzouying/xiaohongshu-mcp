package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
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

// waitForFeedsSettled 等待搜索结果异步数据真正加载完成。
// window.__INITIAL_STATE__ 在页面刚初始化时就已存在（feeds 是空数组占位），
// 直接判断它 !== undefined 会在异步请求返回之前读到"假的空结果"。
// 这里改为轮询等待，直到 feeds 数组非空、或明确出现登录墙/笔记链接等可判定信号；
// 超时（8秒）后放弃等待，按当前状态继续——此时才认为是真实的零结果。
func waitForFeedsSettled(page *rod.Page) {
	settledJS := `() => {
		if (window.__INITIAL_STATE__ === undefined) return false;
		if (document.body && document.body.innerText.includes('登录后查看搜索结果')) return true;
		const search = window.__INITIAL_STATE__.search;
		if (!search) return false;
		const feeds = search.feeds;
		const raw = feeds ? (feeds.value !== undefined ? feeds.value : feeds._value) : undefined;
		if (Array.isArray(raw) && raw.length > 0) return true;
		return document.querySelectorAll('a[href*="/explore/"]').length > 0;
	}`

	_ = rod.Try(func() {
		page.Timeout(8 * time.Second).MustWait(settledJS)
	})
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	// 注意 .Context(ctx) 会替换掉 NewSearchAction 里设的 60s deadline，必须在其后重新 Timeout，
	// 否则搜索页不 stable 时 MustWaitStable/MustWait 会永久挂起（无 deadline 可依赖）。
	page := s.page.Context(ctx).Timeout(60 * time.Second)

	// 搜索页直接打开经常落到“登录后查看搜索结果”的壳页。
	// 先从首页进入，复用页面初始化后的登录态和前端上下文，再触发真实搜索。
	page.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()
	page.MustWait(`() => document.querySelector('#search-input') !== null`)
	searchInput := page.MustElement("#search-input")
	searchInput.MustSelectAllText()
	searchInput.MustInput(keyword).MustType(input.Enter)
	page.MustWait(`() => window.location.href.includes('/search_result') || (document.body && document.body.innerText.includes('登录后查看搜索结果'))`)
	page.MustWaitLoad()
	waitForFeedsSettled(page)

	// 将所有 FilterOption 转换为内部筛选选项
	var allInternalFilters []internalFilterOption
	for _, filter := range filters {
		internalFilters, err := convertToInternalFilters(filter)
		if err != nil {
			return nil, fmt.Errorf("筛选选项转换失败: %w", err)
		}
		allInternalFilters = append(allInternalFilters, internalFilters...)
	}

	// 只有存在有效筛选项时才操作筛选面板；空筛选对象会导致页面一直等待弹层。
	if len(allInternalFilters) > 0 {
		// 验证所有内部筛选选项
		for _, filter := range allInternalFilters {
			if err := validateInternalFilterOption(filter); err != nil {
				return nil, fmt.Errorf("筛选选项验证失败: %w", err)
			}
		}

		// 悬停在筛选按钮上
		filterButton := page.MustElement(`div.filter`)
		filterButton.MustHover()
		humanize.Delay(ctx, humanize.BeforeClick)

		// 等待筛选面板出现
		page.MustWait(`() => document.querySelector('div.filter-panel') !== null`)

		// 用 ClickNoWait：筛选面板是 hover 浮层，rod 的 WaitInteractable 会误判被遮挡而死等；
		// ClickNoWait 移进面板内选项（维持 hover、面板不关）再点。
		for _, filter := range allInternalFilters {
			selector := fmt.Sprintf(`div.filter-panel div.filters:nth-child(%d) div.tags:nth-child(%d)`,
				filter.FiltersIndex, filter.TagsIndex)
			option := page.MustElement(selector)
			humanize.Delay(ctx, humanize.BeforeClick)
			if err := humanize.ClickNoWait(option); err != nil {
				return nil, fmt.Errorf("点击筛选选项失败: %w", err)
			}
		}

		// 搜索页会持续请求推荐流，等待 stable 容易卡死；这里只等筛选后的状态回填。
		page.MustWaitLoad()
		waitForFeedsSettled(page)
	}

	pageState := page.MustEval(`() => JSON.stringify({
		bodyText: document.body ? document.body.innerText.slice(0, 500) : "",
		hasLoginGate: document.body ? document.body.innerText.includes("登录后查看搜索结果") : false,
		pathname: location.pathname,
		url: location.href,
	})`).String()

	if pageState != "" {
		var state struct {
			BodyText     string `json:"bodyText"`
			HasLoginGate bool   `json:"hasLoginGate"`
			Pathname     string `json:"pathname"`
			URL          string `json:"url"`
		}
		if err := json.Unmarshal([]byte(pageState), &state); err == nil && state.HasLoginGate {
			return nil, fmt.Errorf("搜索页未进入可见结果态，当前页面提示需要登录查看搜索结果: %s", state.URL)
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
