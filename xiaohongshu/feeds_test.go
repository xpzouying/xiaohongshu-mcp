package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

func TestGetFeedsList(t *testing.T) {

	t.Skip("SKIP: 测试发布")

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	// NewFeedsListAction 内部已经处理导航
	action := NewFeedsListAction(page)

	feeds, err := action.GetFeedsList(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, feeds, "feeds should not be empty")

	fmt.Printf("成功获取到 %d 个 Feed\n", len(feeds))

	// 验证 JSON 结构完整性
	for i, feed := range feeds {
		// 验证必填字段
		require.NotEmpty(t, feed.ID, "Feed ID should not be empty")
		require.NotEmpty(t, feed.ModelType, "ModelType should not be empty")
		require.NotEmpty(t, feed.XsecToken, "XsecToken should not be empty")
		require.NotEmpty(t, feed.NoteCard.Type, "NoteCard Type should not be empty")
		require.NotEmpty(t, feed.NoteCard.DisplayTitle, "DisplayTitle should not be empty")
		require.NotEmpty(t, feed.NoteCard.User.UserID, "User ID should not be empty")
		require.NotEmpty(t, feed.NoteCard.User.Nickname, "User nickname should not be empty")

		// 如果是视频类型，检查视频信息
		if feed.NoteCard.Type == "video" {
			require.NotNil(t, feed.NoteCard.Video, "Video info should not be nil for video type")
			if feed.NoteCard.Video != nil {
				require.True(t, feed.NoteCard.Video.Capa.Duration > 0, "Video duration should be greater than 0")
			}
		}

		// 只对第一个 Feed 进行完整 JSON 序列化检查
		if i == 0 {
			// 序列化为 JSON
			jsonData, err := json.MarshalIndent(feed, "", "  ")
			require.NoError(t, err, "Failed to marshal feed")

			fmt.Printf("\n第一个 Feed 的完整 JSON 结构:\n%s\n", string(jsonData))

			// 反序列化检查
			var checkFeed Feed
			err = json.Unmarshal(jsonData, &checkFeed)
			require.NoError(t, err, "Failed to unmarshal feed")

			// 比较序列化前后是否一致
			require.Equal(t, feed.ID, checkFeed.ID)
			require.Equal(t, feed.ModelType, checkFeed.ModelType)
			require.Equal(t, feed.NoteCard.Type, checkFeed.NoteCard.Type)
		}

		// 打印前3个 Feed 的信息
		if i < 3 {
			fmt.Printf("\nFeed %d 基本信息:\n", i+1)
			fmt.Printf("  ID: %s\n", feed.ID)
			fmt.Printf("  ModelType: %s\n", feed.ModelType)
			fmt.Printf("  标题: %s\n", feed.NoteCard.DisplayTitle)
			fmt.Printf("  类型: %s\n", feed.NoteCard.Type)
			fmt.Printf("  作者: %s (@%s)\n", feed.NoteCard.User.Nickname, feed.NoteCard.User.UserID)
			fmt.Printf("  点赞数: %s\n", feed.NoteCard.InteractInfo.LikedCount)
			fmt.Printf("  封面尺寸: %dx%d\n", feed.NoteCard.Cover.Width, feed.NoteCard.Cover.Height)
			if feed.NoteCard.Type == "video" && feed.NoteCard.Video != nil {
				fmt.Printf("  视频时长: %d秒\n", feed.NoteCard.Video.Capa.Duration)
			}
		}
	}
}
