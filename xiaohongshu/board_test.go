package xiaohongshu

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

// 需要已登录的 cookies，本地联调时去掉 Skip 跑
func TestListBoards(t *testing.T) {

	t.Skip("SKIP: 需要登录态")

	b := browser.NewBrowser(true)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	boards, err := NewBoardAction(page).ListBoards(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, boards)

	for _, bd := range boards {
		require.NotEmpty(t, bd.ID)
		require.NotEmpty(t, bd.Name)
		fmt.Printf("%s  %s  笔记 %d\n", bd.ID, bd.Name, bd.NoteCount)
	}
}

// 专辑之间直接移动：笔记应当出现在目标专辑、从原专辑消失
func TestMoveNoteBetweenBoards(t *testing.T) {

	t.Skip("SKIP: 需要登录态，并按自己账号填 noteID / boardID")

	const (
		noteID   = ""
		fromID   = ""
		targetID = ""
	)

	b := browser.NewBrowser(true)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := NewBoardAction(page)
	require.NoError(t, action.MoveNoteToBoard(context.Background(), noteID, targetID, fromID))

	in, err := noteInBoard(page, fromID, noteID)
	require.NoError(t, err)
	require.False(t, in, "笔记应当已从原专辑移出")
}

// 移出专辑但保留收藏
func TestRemoveNoteFromBoard(t *testing.T) {

	t.Skip("SKIP: 需要登录态，并按自己账号填 noteID")

	const noteID = ""

	b := browser.NewBrowser(true)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	require.NoError(t, NewBoardAction(page).RemoveNoteFromBoard(context.Background(), noteID, ""))
}

// 新建 / 删除专辑
func TestCreateAndDeleteBoard(t *testing.T) {

	t.Skip("SKIP: 需要登录态")

	b := browser.NewBrowser(true)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := NewBoardAction(page)
	ctx := context.Background()

	board, err := action.CreateBoard(ctx, "临时测试专辑", true)
	require.NoError(t, err)
	require.NotEmpty(t, board.ID)

	// 专辑名超过 12 个字符应当被拦下
	_, err = action.CreateBoard(ctx, "这个名字明显超过了十二个字符限制", true)
	require.Error(t, err)

	require.NoError(t, action.DeleteBoard(ctx, board.ID))
}
