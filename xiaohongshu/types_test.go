package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOnlyNotes 固定「搜索与列表只返回笔记」这条约束。
//
// 取值取自真机走查：搜「露营」+ 图文，23 条里混着 3 条 live_v2 和 2 条 hot_query；
// 换成视频筛选，20 条笔记的 modelType 同样是 note，只是 noteCard.type 变成 video。
func TestOnlyNotes(t *testing.T) {
	t.Run("滤掉直播卡片与搜索热词", func(t *testing.T) {
		feeds := []Feed{
			{ID: "1", ModelType: "note", NoteCard: NoteCard{Type: "normal", DisplayTitle: "山顶露营"}},
			{ID: "2", ModelType: "live_v2"},
			{ID: "3", ModelType: "hot_query"},
			{ID: "4", ModelType: "note", NoteCard: NoteCard{Type: "normal", DisplayTitle: "露营日记"}},
		}

		got := onlyNotes(feeds)

		assert.Len(t, got, 2)
		assert.Equal(t, "1", got[0].ID)
		assert.Equal(t, "4", got[1].ID)
	})

	t.Run("视频笔记不被误伤", func(t *testing.T) {
		feeds := []Feed{
			{ID: "1", ModelType: "note", NoteCard: NoteCard{Type: "video"}},
			{ID: "2", ModelType: "note", NoteCard: NoteCard{Type: "normal"}},
		}

		assert.Len(t, onlyNotes(feeds), 2, "视频与图文的 modelType 同为 note，都要保留")
	})

	t.Run("无标题的笔记要保留", func(t *testing.T) {
		// 平台允许笔记没有标题，这类条目 displayTitle 为空但确实是笔记，
		// 不能跟着非笔记条目一起滤掉
		feeds := []Feed{{ID: "1", ModelType: "note", NoteCard: NoteCard{Type: "normal"}}}

		assert.Len(t, onlyNotes(feeds), 1)
	})

	t.Run("全是非笔记时返回空而不是 nil", func(t *testing.T) {
		got := onlyNotes([]Feed{{ModelType: "hot_query"}})

		assert.NotNil(t, got, "返回空切片，避免调用方拿到 nil 再判空")
		assert.Empty(t, got)
	})
}
