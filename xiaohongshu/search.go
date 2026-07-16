package xiaohongshu

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	xhserrors "github.com/xpzouying/xiaohongshu-mcp/errors"
)

const (
	searchTimeout         = 25 * time.Second
	initialAttemptTimeout = 8 * time.Second
)

// SearchError 提供可供 API 层识别的搜索错误。
type SearchError struct {
	Code  string
	Stage string
	Err   error
}

func (e *SearchError) Error() string {
	return fmt.Sprintf("搜索失败 [%s]: %v", e.Stage, e.Err)
}

func (e *SearchError) Unwrap() error {
	return e.Err
}

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
	if filter.SortBy != "" && filter.SortBy != "综合" {
		internal, err := findInternalOption(1, filter.SortBy)
		if err != nil {
			return nil, fmt.Errorf("排序依据错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理笔记类型
	if filter.NoteType != "" && filter.NoteType != "不限" {
		internal, err := findInternalOption(2, filter.NoteType)
		if err != nil {
			return nil, fmt.Errorf("笔记类型错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理发布时间
	if filter.PublishTime != "" && filter.PublishTime != "不限" {
		internal, err := findInternalOption(3, filter.PublishTime)
		if err != nil {
			return nil, fmt.Errorf("发布时间错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理搜索范围
	if filter.SearchScope != "" && filter.SearchScope != "不限" {
		internal, err := findInternalOption(4, filter.SearchScope)
		if err != nil {
			return nil, fmt.Errorf("搜索范围错误: %w", err)
		}
		internalFilters = append(internalFilters, internal)
	}

	// 处理位置距离
	if filter.Location != "" && filter.Location != "不限" {
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
	page         *rod.Page
	timeout      time.Duration
	navigate     func(context.Context, string) error
	reload       func(context.Context) error
	waitResults  func(context.Context, string) (searchSnapshot, error)
	applyFilters func(context.Context, []internalFilterOption) error
}

type searchSnapshot struct {
	Feeds     []Feed `json:"feeds"`
	Signature string `json:"signature"`
	NoResult  bool   `json:"noResult"`
}

func NewSearchAction(page *rod.Page) *SearchAction {
	return &SearchAction{page: page, timeout: searchTimeout}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := s.timeout
	if timeout <= 0 {
		timeout = searchTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, searchContextError("start", err)
	}
	searchURL := makeSearchURL(keyword)
	snapshot, err := s.loadInitialResults(ctx, searchURL)
	if shouldRetryInitialSearch(snapshot, err) {
		if reloadErr := s.reloadPage(ctx); reloadErr != nil {
			return nil, newSearchError("retry_reload", ctx, reloadErr)
		}
		snapshot, err = s.waitSearchResults(ctx, "")
	}
	if err != nil {
		return nil, normalizeSearchError("wait_initial_state", ctx, err)
	}

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

		if len(allInternalFilters) > 0 {
			if err := s.applySearchFilters(ctx, allInternalFilters); err != nil {
				return nil, normalizeSearchError("apply_filters", ctx, err)
			}
			snapshot, err = s.waitSearchResults(ctx, snapshot.Signature)
			if err != nil {
				return nil, normalizeSearchError("wait_filtered_results", ctx, err)
			}
		}
	}

	return snapshot.Feeds, nil
}

func (s *SearchAction) loadInitialResults(ctx context.Context, target string) (searchSnapshot, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		if err := s.navigatePage(ctx, target); err != nil {
			return searchSnapshot{}, newSearchError("navigate", ctx, err)
		}
		return s.waitSearchResults(ctx, "")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return searchSnapshot{}, ctx.Err()
	}
	attemptTimeout := min(remaining, initialAttemptTimeout)
	attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	if err := s.navigatePage(attemptCtx, target); err != nil {
		return searchSnapshot{}, newSearchError("navigate", attemptCtx, err)
	}
	return s.waitSearchResults(attemptCtx, "")
}

func (s *SearchAction) navigatePage(ctx context.Context, target string) error {
	if s.navigate != nil {
		return s.navigate(ctx, target)
	}
	page := s.page.Context(ctx)
	if err := page.Navigate(target); err != nil {
		return err
	}
	return page.WaitStable(300 * time.Millisecond)
}

func (s *SearchAction) reloadPage(ctx context.Context) error {
	if s.reload != nil {
		return s.reload(ctx)
	}
	page := s.page.Context(ctx)
	if err := page.Reload(); err != nil {
		return err
	}
	return page.WaitStable(300 * time.Millisecond)
}

func (s *SearchAction) waitSearchResults(ctx context.Context, previousSignature string) (searchSnapshot, error) {
	if s.waitResults != nil {
		return s.waitResults(ctx, previousSignature)
	}
	page := s.page.Context(ctx)
	condition := fmt.Sprintf(`() => {
		const state = window.__INITIAL_STATE__;
		const holder = state && state.search && state.search.feeds;
		const feeds = holder && (holder.value !== undefined ? holder.value : holder._value);
		const signature = Array.isArray(feeds) ? feeds.map(feed => feed && (feed.id || feed.noteCard && feed.noteCard.noteId) || '').join('|') : '';
		const noResult = Boolean(document.querySelector('.no-result, .search-no-result, [class*="no-result"], [class*="empty-result"]'));
		return noResult || (Array.isArray(feeds) && feeds.length > 0 && signature !== %q);
	}`, previousSignature)
	if err := page.Wait(rod.Eval(condition)); err != nil {
		return searchSnapshot{}, newSearchError("wait_initial_state", ctx, err)
	}
	evaluated, err := page.Eval(`() => {
		const holder = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.search && window.__INITIAL_STATE__.search.feeds;
		const feeds = holder && (holder.value !== undefined ? holder.value : holder._value);
		const list = Array.isArray(feeds) ? feeds : [];
		return JSON.stringify({
			feeds: list,
			signature: list.map(feed => feed && (feed.id || feed.noteCard && feed.noteCard.noteId) || '').join('|'),
			noResult: Boolean(document.querySelector('.no-result, .search-no-result, [class*="no-result"], [class*="empty-result"]'))
		});
	}`)
	if err != nil {
		return searchSnapshot{}, newSearchError("read_results", ctx, err)
	}
	var snapshot searchSnapshot
	if err := json.Unmarshal([]byte(evaluated.Value.Str()), &snapshot); err != nil {
		return searchSnapshot{}, fmt.Errorf("解析搜索结果失败: %w", err)
	}
	if len(snapshot.Feeds) == 0 && !snapshot.NoResult {
		return searchSnapshot{}, xhserrors.ErrNoFeeds
	}
	return snapshot, nil
}

func (s *SearchAction) applySearchFilters(ctx context.Context, filters []internalFilterOption) error {
	if s.applyFilters != nil {
		return s.applyFilters(ctx, filters)
	}
	page := s.page.Context(ctx)
	filterButton, err := page.Element(`div.filter`)
	if err != nil {
		return err
	}
	if err := filterButton.Hover(); err != nil {
		return err
	}
	if err := page.Wait(rod.Eval(`() => document.querySelector('div.filter-panel') !== null`)); err != nil {
		return err
	}
	for _, filter := range filters {
		selector := fmt.Sprintf(`div.filter-panel div.filters:nth-child(%d) div.tags:nth-child(%d)`, filter.FiltersIndex, filter.TagsIndex)
		option, err := page.Element(selector)
		if err != nil {
			return err
		}
		if err := option.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return err
		}
	}
	return nil
}

func shouldRetryInitialSearch(snapshot searchSnapshot, err error) bool {
	if err == nil {
		return snapshot.NoResult && len(snapshot.Feeds) == 0
	}
	if stderrors.Is(err, xhserrors.ErrNoFeeds) {
		return true
	}
	var searchErr *SearchError
	return stderrors.As(err, &searchErr) && (searchErr.Stage == "navigate" || searchErr.Stage == "wait_initial_state")
}

func normalizeSearchError(stage string, ctx context.Context, err error) error {
	var searchErr *SearchError
	if stderrors.As(err, &searchErr) {
		return err
	}
	return newSearchError(stage, ctx, err)
}

func (s *SearchAction) withTimeout(timeout time.Duration) *SearchAction {
	clone := *s
	clone.timeout = timeout
	return &clone
}

func newSearchError(stage string, ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return searchContextError(stage, ctxErr)
	}
	return &SearchError{Code: "SEARCH_FAILED", Stage: stage, Err: err}
}

func searchContextError(stage string, err error) error {
	code := "SEARCH_CANCELED"
	if stderrors.Is(err, context.DeadlineExceeded) {
		code = "SEARCH_TIMEOUT"
	}
	return &SearchError{Code: code, Stage: stage, Err: err}
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_search_result_notes
	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_explore_feed
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
