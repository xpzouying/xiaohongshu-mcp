package xiaohongshu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

func TestDeleteNoteByIndex(t *testing.T) {
	// 删除属于破坏性操作，默认跳过
	t.Skip("SKIP: 删除笔记为破坏性操作，请确认后开启")
	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action, err := NewNoteManagerAction(page)
	require.NoError(t, err)

	notes, err := action.ListNotes(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, notes, "notes should not be empty")

	deleted, _, err := action.DeleteNote(context.Background(), DeleteNoteOptions{Index: 1})
	require.NoError(t, err)
	require.NotEmpty(t, deleted.Title)
}

func TestDeleteNoteByTitle(t *testing.T) {
	// 删除属于破坏性操作，默认跳过
	t.Skip("SKIP: 删除笔记为破坏性操作，请确认后开启")

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action, err := NewNoteManagerAction(page)
	require.NoError(t, err)

	deleted, _, err := action.DeleteNote(context.Background(), DeleteNoteOptions{
		Keyword: "",
		Title:   "测试",
	})
	require.NoError(t, err)
	require.NotEmpty(t, deleted.Title)
}
