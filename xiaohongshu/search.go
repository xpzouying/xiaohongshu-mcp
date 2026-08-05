package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
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
type filterGroup struct {
	label   string
	pick    func(FilterOption) string
	allowed []string
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
	group  string
	option string
}

// collectFilters 把入参展开成待应用的筛选项，顺便校验取值。
// 校验放在打开浏览器之前，写错的值不该先向平台发一次请求再报错。
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
	page playwright.Page
}

func NewSearchAction(page playwright.Page) *SearchAction {
	return &SearchAction{page: page}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	// 先校验筛选取值，必须在导航之前——写错的值不该先向平台发一次请求再报错。
	pending, err := collectFilters(filters)
	if err != nil {
		return nil, err
	}

	page := s.page
	searchURL := makeSearchURL(keyword)
	if _, err := page.Goto(searchURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60_000),
	}); err != nil {
		return nil, fmt.Errorf("导航搜索页失败: %w", err)
	}
	if _, err := page.WaitForFunction(`() => window.__INITIAL_STATE__ !== undefined`, nil,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(60_000)}); err != nil {
		return nil, fmt.Errorf("等待页面状态失败: %w", err)
	}
	humanize.Delay(ctx, humanize.AfterNavigate)

	if len(pending) > 0 {
		// 悬停在筛选按钮上展开面板
		filterButton, err := page.QuerySelector(`div.filter`)
		if err != nil || filterButton == nil {
			return nil, fmt.Errorf("未找到筛选按钮: %w", err)
		}
		if err := humanize.Hover(filterButton); err != nil {
			return nil, fmt.Errorf("悬停筛选按钮失败: %w", err)
		}
		humanize.Delay(ctx, humanize.BeforeClick)

		// 等待筛选面板出现
		if _, err := page.WaitForSelector(`div.filter-panel`, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateAttached,
			Timeout: playwright.Float(10_000),
		}); err != nil {
			return nil, fmt.Errorf("等待筛选面板失败: %w", err)
		}

		// 记下筛选前的结果，用来判断筛选后的数据什么时候到位
		before := readFeedIDs(page)

		// 用 ClickNoWait：筛选面板是 hover 浮层，遮挡校验会误判而死等；
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

	result := evalString(page, `() => {
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

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}

	return onlyNotes(feeds), nil
}

// feedIDsJS 读当前结果集的 id 列表，用来判断数据有没有换一批。
const feedIDsJS = `() => {
	const f = window.__INITIAL_STATE__?.search?.feeds;
	const v = f ? (f.value !== undefined ? f.value : f._value) : null;
	return v ? v.map(x => x.id).join(",") : "";
}`

func readFeedIDs(page playwright.Page) string {
	return evalString(page, feedIDsJS)
}

// waitFeedsChanged 等筛选后的数据到位。
// 点完筛选项之后不能立刻读结果：站点先把 feeds 清空再灌新数据，
// 超时不报错——筛选已点上，宁可返回偏旧数据也不要整个搜索失败。
func waitFeedsChanged(page playwright.Page, before string, timeout time.Duration) {
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
// 全程不用序号；作用域限定在 div.filter-panel 内且只认 div.tags，避免点错同文本元素。
func findFilterOption(page playwright.Page, pf pendingFilter) (playwright.ElementHandle, error) {
	groups, err := page.QuerySelectorAll("div.filter-panel div.filters")
	if err != nil {
		return nil, fmt.Errorf("读取筛选面板失败: %w", err)
	}

	for _, group := range groups {
		label, err := group.QuerySelector(":scope > span")
		if err != nil || label == nil {
			continue
		}
		text, err := label.InnerText()
		if err != nil || strings.TrimSpace(text) != pf.group {
			continue
		}

		options, err := group.QuerySelectorAll("div.tags")
		if err != nil {
			return nil, fmt.Errorf("读取「%s」的选项失败: %w", pf.group, err)
		}

		var available []string
		for _, opt := range options {
			t, err := opt.InnerText()
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
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
