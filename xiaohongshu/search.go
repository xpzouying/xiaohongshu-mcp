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

// internalFilterOption 内部使用的筛选选项
type internalFilterOption struct {
	FiltersIndex int    // 筛选组索引
	TagsIndex    int    // 标签索引
	GroupText    string // 筛选组标题
	Text         string // 标签文本描述
}

// 预定义的筛选选项映射表（内部使用）
var filterOptionsMap = map[int][]internalFilterOption{
	1: { // 排序依据
		{FiltersIndex: 1, TagsIndex: 1, GroupText: "排序依据", Text: "综合"},
		{FiltersIndex: 1, TagsIndex: 2, GroupText: "排序依据", Text: "最新"},
		{FiltersIndex: 1, TagsIndex: 3, GroupText: "排序依据", Text: "最多点赞"},
		{FiltersIndex: 1, TagsIndex: 4, GroupText: "排序依据", Text: "最多评论"},
		{FiltersIndex: 1, TagsIndex: 5, GroupText: "排序依据", Text: "最多收藏"},
	},
	2: { // 笔记类型
		{FiltersIndex: 2, TagsIndex: 1, GroupText: "笔记类型", Text: "不限"},
		{FiltersIndex: 2, TagsIndex: 2, GroupText: "笔记类型", Text: "视频"},
		{FiltersIndex: 2, TagsIndex: 3, GroupText: "笔记类型", Text: "图文"},
	},
	3: { // 发布时间
		{FiltersIndex: 3, TagsIndex: 1, GroupText: "发布时间", Text: "不限"},
		{FiltersIndex: 3, TagsIndex: 2, GroupText: "发布时间", Text: "一天内"},
		{FiltersIndex: 3, TagsIndex: 3, GroupText: "发布时间", Text: "一周内"},
		{FiltersIndex: 3, TagsIndex: 4, GroupText: "发布时间", Text: "半年内"},
	},
	4: { // 搜索范围
		{FiltersIndex: 4, TagsIndex: 1, GroupText: "搜索范围", Text: "不限"},
		{FiltersIndex: 4, TagsIndex: 2, GroupText: "搜索范围", Text: "已看过"},
		{FiltersIndex: 4, TagsIndex: 3, GroupText: "搜索范围", Text: "未看过"},
		{FiltersIndex: 4, TagsIndex: 4, GroupText: "搜索范围", Text: "已关注"},
	},
	5: { // 位置距离
		{FiltersIndex: 5, TagsIndex: 1, GroupText: "位置距离", Text: "不限"},
		{FiltersIndex: 5, TagsIndex: 2, GroupText: "位置距离", Text: "同城"},
		{FiltersIndex: 5, TagsIndex: 3, GroupText: "位置距离", Text: "附近"},
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

func (s *SearchAction) Search(ctx context.Context, keyword string, filters ...FilterOption) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)
	if err := page.Navigate(searchURL); err != nil {
		return nil, fmt.Errorf("搜索页导航失败: %w", err)
	}
	if err := page.Timeout(5 * time.Second).WaitStable(500 * time.Millisecond); err != nil {
		logrus.Warnf("等待搜索页稳定失败，继续尝试读取结果: %v", err)
	}

	if err := waitForInitialState(page); err != nil {
		return nil, err
	}

	// 如果有筛选条件，则应用筛选
	if len(filters) > 0 {
		filterCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		filterPage := page.Context(filterCtx)
		if err := s.applyFilters(filterPage, filters...); err != nil {
			logrus.Warnf("搜索筛选应用失败，降级为未筛选搜索结果: %v", err)
		}
		cancel()
	}

	resultObj, err := page.Timeout(10 * time.Second).Evaluate(rod.Eval(`() => {
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
	}`))
	if err != nil {
		return nil, fmt.Errorf("读取搜索结果失败: %w", err)
	}
	result := resultObj.Value.String()

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var feeds []Feed
	if err := json.Unmarshal([]byte(result), &feeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feeds: %w", err)
	}

	return feeds, nil
}

func waitForInitialState(page *rod.Page) error {
	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`() => window.__INITIAL_STATE__ !== undefined`)); err != nil {
		return fmt.Errorf("等待搜索结果初始化失败: %w", err)
	}
	return nil
}

func (s *SearchAction) applyFilters(page *rod.Page, filters ...FilterOption) error {
	allInternalFilters, err := convertFilters(filters...)
	if err != nil {
		return err
	}
	if len(allInternalFilters) == 0 {
		return nil
	}

	if err := openFilterPanel(page); err != nil {
		return err
	}

	for _, filter := range allInternalFilters {
		if err := clickFilterOptionByText(page, filter); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}

	time.Sleep(time.Second)
	return nil
}

func convertFilters(filters ...FilterOption) ([]internalFilterOption, error) {
	var allInternalFilters []internalFilterOption
	for _, filter := range filters {
		internalFilters, err := convertToInternalFilters(filter)
		if err != nil {
			return nil, fmt.Errorf("筛选选项转换失败: %w", err)
		}
		allInternalFilters = append(allInternalFilters, internalFilters...)
	}

	for _, filter := range allInternalFilters {
		if err := validateInternalFilterOption(filter); err != nil {
			return nil, fmt.Errorf("筛选选项验证失败: %w", err)
		}
	}
	return allInternalFilters, nil
}

func openFilterPanel(page *rod.Page) error {
	filterButton, err := page.Timeout(5 * time.Second).Element(`div.filter`)
	if err != nil {
		return fmt.Errorf("未找到筛选按钮: %w", err)
	}
	if err := filterButton.Hover(); err != nil {
		return fmt.Errorf("悬停筛选按钮失败: %w", err)
	}
	if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`() => document.querySelector('div.filter-panel') !== null`)); err != nil {
		return fmt.Errorf("筛选面板未出现: %w", err)
	}
	return nil
}

func clickFilterOptionByText(page *rod.Page, filter internalFilterOption) error {
	if err := openFilterPanel(page); err != nil {
		return err
	}

	active, err := filterOptionActiveByJS(page, filter)
	if err == nil && active {
		logrus.Debugf("搜索筛选已处于选中状态: %s=%s", filter.GroupText, filter.Text)
		return nil
	}
	if err != nil {
		logrus.Debugf("读取搜索筛选状态失败，继续尝试点击: %v", err)
	}

	if err := clickSearchFilterOption(page, filter); err != nil {
		return fmt.Errorf("点击筛选项 %s=%s 失败: %w", filter.GroupText, filter.Text, err)
	}

	time.Sleep(300 * time.Millisecond)
	active, err = filterOptionActiveByJS(page, filter)
	if err == nil && active {
		return nil
	}
	if err != nil {
		return err
	}

	return fmt.Errorf("筛选项点击后未变为选中状态: %s=%s", filter.GroupText, filter.Text)
}

func filterOptionActiveByJS(page *rod.Page, filter internalFilterOption) (bool, error) {
	res, err := page.Timeout(2 * time.Second).Evaluate(rod.Eval(`(groupText, targetText) => {
		const visible = (el) => {
			const style = window.getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return style.display !== 'none'
				&& style.visibility !== 'hidden'
				&& Number(style.opacity) !== 0
				&& rect.width > 0
				&& rect.height > 0;
		};
		const groups = [...document.querySelectorAll('div.filter-panel div.filters')].filter(visible);
		const group = groups.find(el => (el.innerText || '').includes(groupText));
		if (!group) return false;
		const tags = [...group.querySelectorAll('div.tags')].filter(visible);
		return tags.some(el => {
			const text = (el.innerText || '').trim();
			const classes = String(el.className || '').split(/\s+/);
			return text === targetText && classes.includes('active');
		});
	}`, filter.GroupText, filter.Text))
	if err != nil {
		return false, err
	}
	return res.Value.Bool(), nil
}

func clickSearchFilterOption(page *rod.Page, filter internalFilterOption) error {
	res, err := page.Timeout(3 * time.Second).Evaluate(rod.Eval(`(groupText, targetText) => {
		const visible = (el) => {
			const style = window.getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return style.display !== 'none'
				&& style.visibility !== 'hidden'
				&& Number(style.opacity) !== 0
				&& rect.width > 0
				&& rect.height > 0;
		};
		const groups = [...document.querySelectorAll('div.filter-panel div.filters')].filter(visible);
		const group = groups.find(el => (el.innerText || '').includes(groupText));
		if (!group) return { ok: false, reason: 'group not found: ' + groupText };

		const tags = [...group.querySelectorAll('div.tags')].filter(visible);
		const candidates = tags
			.filter(el => (el.innerText || '').trim() === targetText)
			.map((el, index) => ({
				el,
				index,
				className: el.className || '',
				tabIndex: el.getAttribute('tabindex') || '',
				ariaChecked: el.getAttribute('aria-checked') || '',
				role: el.getAttribute('role') || '',
			}));
		if (!candidates.length) return { ok: false, reason: 'tag not found: ' + groupText + '=' + targetText };

		const candidate = candidates.find(item => !String(item.className).split(/\s+/).includes('active')) || candidates[0];
		const el = candidate.el;
		el.scrollIntoView({ block: 'center', inline: 'center' });
		for (const type of ['mouseover', 'mouseenter', 'mousedown', 'mouseup', 'click']) {
			el.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true, view: window }));
		}

		return {
			ok: true,
			clicked: {
				index: candidate.index,
				className: candidate.className,
				tabIndex: candidate.tabIndex,
				ariaChecked: candidate.ariaChecked,
				role: candidate.role,
			},
		};
	}`, filter.GroupText, filter.Text))
	if err != nil {
		return err
	}
	if !res.Value.Get("ok").Bool() {
		return fmt.Errorf("%s", res.Value.Get("reason").Str())
	}
	return nil
}

func findFilterGroup(page *rod.Page, filter internalFilterOption) (*rod.Element, error) {
	groups, err := page.Timeout(5 * time.Second).Elements(`div.filter-panel div.filters`)
	if err != nil {
		return nil, fmt.Errorf("查找筛选组失败: %w", err)
	}

	for _, group := range groups {
		if !isSearchFilterElementVisible(group) {
			continue
		}
		text, err := group.Text()
		if err != nil {
			logrus.Debugf("读取筛选组文本失败: %v", err)
			continue
		}
		if strings.Contains(strings.TrimSpace(text), filter.GroupText) {
			return group, nil
		}
	}

	return nil, fmt.Errorf("未找到筛选组: %s", filter.GroupText)
}

func findVisibleFilterTag(group *rod.Element, targetText string) (*rod.Element, error) {
	tags, err := group.Elements(`div.tags`)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if !isSearchFilterElementVisible(tag) {
			continue
		}
		text, err := tag.Text()
		if err != nil {
			logrus.Debugf("读取筛选项文本失败: %v", err)
			continue
		}
		if strings.TrimSpace(text) == targetText {
			return tag, nil
		}
	}

	return nil, fmt.Errorf("组内未找到可见文本: %s", targetText)
}

func filterOptionActive(group *rod.Element, targetText string) bool {
	tags, err := group.Elements(`div.tags`)
	if err != nil {
		logrus.Debugf("读取筛选项列表失败: %v", err)
		return false
	}

	for _, tag := range tags {
		if !isSearchFilterElementVisible(tag) {
			continue
		}
		text, err := tag.Text()
		if err != nil || strings.TrimSpace(text) != targetText {
			continue
		}

		className, err := tag.Attribute("class")
		if err != nil || className == nil {
			continue
		}
		if hasExactClass(*className, "active") {
			return true
		}
	}
	return false
}

func isSearchFilterElementVisible(elem *rod.Element) bool {
	res, err := elem.Eval(`() => {
		const style = window.getComputedStyle(this);
		const rect = this.getBoundingClientRect();
		return style.display !== 'none'
			&& style.visibility !== 'hidden'
			&& Number(style.opacity) !== 0
			&& rect.width > 0
			&& rect.height > 0;
	}`)
	if err != nil {
		logrus.Debugf("检查搜索筛选元素可见性失败: %v", err)
		return true
	}
	return res.Value.Bool()
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_search_result_notes
	//https://www.xiaohongshu.com/search_result?keyword=%25E7%258E%258B%25E5%25AD%2590&source=web_explore_feed
	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
