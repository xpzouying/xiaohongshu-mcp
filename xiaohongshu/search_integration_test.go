//go:build integration

// 集成测试：起有头浏览器 + 触网 + 需登录态，默认 go test 不编译不运行。
// 手动跑：go test -tags integration ./xiaohongshu/ -run TestSearch
package xiaohongshu

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

func TestSearch(t *testing.T) {
	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer func() {
		_ = page.Close()
	}()

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
	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer func() {
		_ = page.Close()
	}()

	action := NewSearchAction(page)

	filter := FilterOption{
		SortBy:      "最新",
		NoteType:    "不限",
		PublishTime: "半年内",
		SearchScope: "不限",
		Location:    "不限",
	}

	feeds, err := action.Search(context.Background(), "无锡 夏天 遛娃 凉快 户外", filter)
	require.NoError(t, err)
	require.NotEmpty(t, feeds, "feeds should not be empty")

	fmt.Printf("成功获取到 %d 个筛选后的 Feed\n", len(feeds))

	for _, feed := range feeds {
		fmt.Printf("Feed ID: %s\n", feed.ID)
		fmt.Printf("Feed Title: %s\n", feed.NoteCard.DisplayTitle)
	}
}

func TestFindFilterOptionSkipsHiddenDuplicate(t *testing.T) {
	b := browser.NewBrowser(true)
	defer b.Close()

	page := b.NewPage()
	defer func() {
		_ = page.Close()
	}()

	page.MustSetDocumentContent(`<div class="filter-panel">
		<div class="filters">
			<span>笔记类型</span>
			<div id="hidden" class="tags" style="display: none">图文</div>
			<div id="visible" class="tags" style="width: 80px; height: 24px">图文</div>
		</div>
	</div>`)

	option, err := findFilterOption(page, pendingFilter{group: "笔记类型", option: "图文"})
	require.NoError(t, err)
	id, err := option.Attribute("id")
	require.NoError(t, err)
	require.NotNil(t, id)
	require.Equal(t, "visible", *id)
}
