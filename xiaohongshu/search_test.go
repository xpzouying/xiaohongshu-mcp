package xiaohongshu

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

func TestSearch(t *testing.T) {

	t.Skip("SKIP: 测试发布")

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := NewSearchAction(page)

	feeds, err := action.Search(context.Background(), "Kimi")
	require.NoError(t, err)
	require.NotEmpty(t, feeds, "feeds should not be empty")

	fmt.Printf("成功获取到 %d 个 Feed\n", len(feeds))

	for _, feed := range feeds {
		fmt.Printf("Feed ID: %s\n", feed.ID)
		fmt.Printf("Feed Title: %s\n", feed.NoteCard.DisplayTitle)
	}
}

func TestSearchWithFilters(t *testing.T) {

	t.Skip("SKIP: 测试筛选功能")

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := NewSearchAction(page)

	// 方式1：直接使用索引
	filters1 := []FilterOption{
		{FiltersIndex: 2, TagsIndex: 3, Text: "图文"},  // 笔记类型 -> 图文
		{FiltersIndex: 3, TagsIndex: 2, Text: "一天内"}, // 发布时间 -> 一天内
	}

	feeds1, err := action.Search(context.Background(), "dn432", filters1...)
	require.NoError(t, err)
	require.NotEmpty(t, feeds1, "feeds should not be empty")

	fmt.Printf("方式1 - 成功获取到 %d 个筛选后的 Feed\n", len(feeds1))

	// 方式2：使用便利函数
	filter2, err := NoteType("图文")
	require.NoError(t, err)

	filter3, err := TimeRange("一天内")
	require.NoError(t, err)

	filters2 := []FilterOption{filter2, filter3}
	feeds2, err := action.Search(context.Background(), "dn432", filters2...)
	require.NoError(t, err)
	require.NotEmpty(t, feeds2, "feeds should not be empty")

	fmt.Printf("方式2 - 成功获取到 %d 个筛选后的 Feed\n", len(feeds2))

	for _, feed := range feeds2 {
		fmt.Printf("Feed ID: %s\n", feed.ID)
		fmt.Printf("Feed Title: %s\n", feed.NoteCard.DisplayTitle)
	}
}

func TestFilterValidation(t *testing.T) {
	// 测试有效的筛选选项
	validFilter := FilterOption{FiltersIndex: 2, TagsIndex: 3, Text: "图文"}
	err := validateFilterOption(validFilter)
	require.NoError(t, err)

	// 测试无效的筛选组索引
	invalidFilterGroup := FilterOption{FiltersIndex: 6, TagsIndex: 1, Text: "无效"}
	err = validateFilterOption(invalidFilterGroup)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的筛选组索引")

	// 测试无效的标签索引
	invalidTagIndex := FilterOption{FiltersIndex: 2, TagsIndex: 5, Text: "无效"}
	err = validateFilterOption(invalidTagIndex)
	require.Error(t, err)
	require.Contains(t, err.Error(), "标签索引 5 超出范围")

	// 测试便利函数
	filter, err := NoteType("图文")
	require.NoError(t, err)
	require.Equal(t, 2, filter.FiltersIndex)
	require.Equal(t, 3, filter.TagsIndex)
	require.Equal(t, "图文", filter.Text)

	// 测试不存在的文本
	_, err = NoteType("不存在的类型")
	require.Error(t, err)
	require.Contains(t, err.Error(), "未找到文本")
}
