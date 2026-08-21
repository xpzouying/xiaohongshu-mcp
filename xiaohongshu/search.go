package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
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

// filterGroup 面板上的一个筛选组：标签是什么、对应入参的哪个字段、允许哪些取值。
//
// 组和选项一律按文本定位，不用序号。面板里同一个选项可能渲染成多个 div.tags
// （数量随视口而变），首项是否重复各组也不一致，下标对不齐。
type filterGroup struct {
	label   string                    // 面板上这一组的标签文本
	pick    func(FilterOption) string // 从入参里取这一组的值
	allowed []string                  // 合法取值；在打开页面之前就能挡掉写错的值
}

var filterGroups = []filterGroup{
	{"排序依据", func(f FilterOption) string { return f.SortBy },
		[]string{"综合", "最新", "最多点赞", "最多评论", "最多收藏"}},
	{"笔记类型", func(f FilterOption) string { return f.NoteType },
		[]string{"不限", "视频", "图文"}},
	{"发布时间", func(f FilterOption) string { return f.PublishTime },
		[]string{"不限", "一天内", "一周内", "半年内"}},
	{"搜索范围", func(f FilterOption) string { return f.SearchScope },
		[]string{"不限", "已看过", "未看过", "已关注"}},
	{"位置距离", func(f FilterOption) string { return f.Location },
		[]string{"不限", "同城", "附近"}},
}

// pendingFilter 一个待应用的筛选项。
type pendingFilter struct {
	group  string // 组标签
	option string // 选项文本
}

// collectFilters 把入参展开成待应用的筛选项，顺便校验取值。
//
// 校验放在这里是为了在打开浏览器之前就挡掉写错的值——否则要等导航、悬停、
// 在面板里找不到之后才能报错，等于为了说一句"你写错了"先向平台发一次请求。
func collectFilters(filters []FilterOption) ([]pendingFilter, error) {
	var pending []pendingFilter

	for _, f := range filters {
		for _, g := range filterGroups {
			value := g.pick(f)
			if value == "" {
				continue
			}
			if !slices.Contains(g.allowed, value) {
				return nil, fmt.Errorf("%s 不支持 %q，可选：%s",
					g.label, value, strings.Join(g.allowed, "、"))
			}
			pending = append(pending, pendingFilter{group: g.label, option: value})
		}
	}

	return pending, nil
}

type SearchAction struct {
	page *rod.Page
}

const (
	maxSearchResults      = 50
	maxSearchScrollRounds = 12
	maxSearchIdleRounds   = 3
	searchScrollWait      = 3 * time.Second
	searchScrollPoll      = 300 * time.Millisecond
)

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(90 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	return s.SearchWithLimit(ctx, keyword, 0, filters...)
}

// SearchWithLimit searches notes and, when maxResults is positive, scrolls the
// result page to collect additional lazy-loaded batches. A zero limit preserves
// the historical Search behavior and returns the first batch only.
func (s *SearchAction) SearchWithLimit(ctx context.Context, keyword string, maxResults int, filters ...FilterOption) ([]Feed, error) {
	// 先校验筛选取值，必须在导航之前——写错的值不该先向平台发一次请求再报错。
	pending, err := collectFilters(filters)
	if err != nil {
		return nil, err
	}

	// 注意 .Context(ctx) 会替换掉 NewSearchAction 里设的 deadline，必须在其后重新 Timeout，
	// 否则搜索页不 stable 时 MustWaitStable/MustWait 会永久挂起（无 deadline 可依赖）。
	page := s.page.Context(ctx).Timeout(90 * time.Second)

	searchURL := makeSearchURL(keyword)
	page.MustNavigate(searchURL)
	page.MustWaitStable()
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)
	humanize.Delay(ctx, humanize.AfterNavigate)

	if len(pending) > 0 {
		// 悬停在筛选按钮上展开面板
		filterButton := page.MustElement(`div.filter`)
		if err := humanize.Hover(filterButton); err != nil {
			return nil, fmt.Errorf("悬停筛选按钮失败: %w", err)
		}
		humanize.Delay(ctx, humanize.BeforeClick)

		// 等待筛选面板出现
		page.MustWait(`() => document.querySelector('div.filter-panel') !== null`)

		// 记下筛选前的结果，用来判断筛选后的数据什么时候到位
		before := readFeedIDs(page)

		// 用 ClickNoWait：筛选面板是 hover 浮层，rod 的 WaitInteractable 会误判被遮挡而死等；
		// ClickNoWait 移进面板内选项（维持 hover、面板不关）再点。
		for _, pf := range pending {
			option, err := findFilterOption(page, pf)
			if err != nil {
				return nil, err
			}
			humanize.Delay(ctx, humanize.BeforeClick)
			if err := humanize.ClickNoWait(option); err != nil {
				return nil, fmt.Errorf("点击筛选选项「%s」失败: %w", pf.option, err)
			}
		}

		waitFeedsChanged(page, before, 15*time.Second)
	}

	feeds, err := readSearchFeeds(page)
	if err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		return onlyNotes(feeds), nil
	}
	if maxResults > maxSearchResults {
		maxResults = maxSearchResults
	}

	collected := make([]Feed, 0, maxResults)
	seen := make(map[string]struct{}, maxResults)
	collected = appendUniqueNotes(collected, seen, feeds, maxResults)
	idleRounds := 0
	for round := 0; round < maxSearchScrollRounds && len(collected) < maxResults && idleRounds < maxSearchIdleRounds; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := page.Eval(`() => {
			const root = document.scrollingElement || document.documentElement;
			window.scrollTo(0, root.scrollHeight);
			return root.scrollHeight;
		}`); err != nil {
			logrus.Warnf("搜索结果滚动失败，返回已收集的 %d 条笔记: %v", len(collected), err)
			break
		}

		added, err := waitForMoreSearchFeeds(ctx, page, &collected, seen, maxResults, searchScrollWait)
		if err != nil {
			return nil, err
		}
		if added {
			idleRounds = 0
		} else {
			idleRounds++
		}
	}

	return collected, nil
}

func readSearchFeeds(page *rod.Page) ([]Feed, error) {
	value, err := page.Eval(`() => {
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
	}`)
	if err != nil {
		return nil, fmt.Errorf("failed to read search feeds: %w", err)
	}
	result := value.Value.Str()

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}

	return feeds, nil
}

func appendUniqueNotes(dst []Feed, seen map[string]struct{}, feeds []Feed, limit int) []Feed {
	for _, feed := range onlyNotes(feeds) {
		key := feed.ID
		if key == "" {
			key = feed.XsecToken
		}
		if key != "" {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
		}
		dst = append(dst, feed)
		if len(dst) >= limit {
			return dst
		}
	}
	return dst
}

func waitForMoreSearchFeeds(
	ctx context.Context,
	page *rod.Page,
	collected *[]Feed,
	seen map[string]struct{},
	limit int,
	timeout time.Duration,
) (bool, error) {
	before := len(*collected)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		feeds, err := readSearchFeeds(page)
		if err == nil {
			*collected = appendUniqueNotes(*collected, seen, feeds, limit)
			if len(*collected) >= limit {
				return true, nil
			}
		}

		timer := time.NewTimer(searchScrollPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return len(*collected) > before, nil
}

// feedIDsJS 读当前结果集的 id 列表，用来判断数据有没有换一批。
const feedIDsJS = `() => {
	const f = window.__INITIAL_STATE__?.search?.feeds;
	const v = f ? (f.value !== undefined ? f.value : f._value) : null;
	return v ? v.map(x => x.id).join(",") : "";
}`

func readFeedIDs(page *rod.Page) string {
	res, err := page.Eval(feedIDsJS)
	if err != nil {
		return ""
	}
	return res.Value.Str()
}

// waitFeedsChanged 等筛选后的数据到位。
//
// 点完筛选项之后不能立刻读结果：站点是先把 feeds 清空、再灌入新数据，
// 中间这段时间读到的要么是空，要么还是筛选前那一批。原先用
// MustWait(__INITIAL_STATE__ !== undefined) 等，而这个条件从首屏起就为真、
// 立即返回，等于没等——多个筛选项一起用时表现为只有一部分生效。
//
// 超时不报错：筛选已经点上了，宁可返回可能偏旧的数据，也不要整个搜索失败。
func waitFeedsChanged(page *rod.Page, before string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if now := readFeedIDs(page); now != "" && now != before {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	logrus.Warnf("筛选后等待结果刷新超时（%s），返回的可能是筛选前的数据", timeout)
}

// findFilterOption 在筛选面板里定位一个选项：按标签找到组，再在组内按文本找选项。
//
// 全程不用序号。同一个选项在面板里可能渲染成多个 div.tags（数量随视口而变，
// 且首项是否重复各组不一致），下标对不齐；早前用 div.tags:nth-child(N) 会选错项。
// 多份重复的位置尺寸完全相同，取第一个点下去落在同一处。
//
// 作用域必须限定在 div.filter-panel 内且只认 div.tags：页面别处存在同文本的
// 可见元素（顶部频道栏的「图文」「视频」、标签「综合」），放宽会点错地方。
func findFilterOption(page *rod.Page, pf pendingFilter) (*rod.Element, error) {
	groups, err := page.Elements("div.filter-panel div.filters")
	if err != nil {
		return nil, fmt.Errorf("读取筛选面板失败: %w", err)
	}

	for _, group := range groups {
		// 组标签是 div.filters 下的直接子 span
		label, err := group.Element(":scope > span")
		if err != nil {
			continue
		}
		text, err := label.Text()
		if err != nil || strings.TrimSpace(text) != pf.group {
			continue
		}

		options, err := group.Elements("div.tags")
		if err != nil {
			return nil, fmt.Errorf("读取「%s」的选项失败: %w", pf.group, err)
		}

		var available []string
		for _, opt := range options {
			t, err := opt.Text()
			if err != nil {
				continue
			}
			t = strings.TrimSpace(t)
			if t == pf.option {
				return opt, nil
			}
			available = append(available, t)
		}
		return nil, fmt.Errorf("「%s」里没有选项「%s」，页面上是：%s",
			pf.group, pf.option, strings.Join(available, "、"))
	}

	return nil, fmt.Errorf("筛选面板里没有「%s」这一组", pf.group)
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_search_result_notes
	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_explore_feed
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
