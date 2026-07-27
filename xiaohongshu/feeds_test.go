//go:build integration

// 集成测试：起有头浏览器 + 触网 + 需登录态，默认 go test 不编译不运行。
// 手动跑：go test -tags integration ./xiaohongshu/ -run TestGetFeedsList
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
	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := NewFeedsListAction(page)

	feeds, err := action.GetFeedsList(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, feeds, "feeds should not be empty")

	fmt.Printf("成功获取到 %d 个 Feed\n", len(feeds))

	for i, feed := range feeds {
		require.NotEmpty(t, feed.ID, "Feed ID should not be empty")
		require.NotEmpty(t, feed.ModelType, "ModelType should not be empty")
		require.NotEmpty(t, feed.XsecToken, "XsecToken should not be empty")
		require.NotEmpty(t, feed.NoteCard.Type, "NoteCard Type should not be empty")
		require.NotEmpty(t, feed.NoteCard.DisplayTitle, "DisplayTitle should not be empty")
		require.NotEmpty(t, feed.NoteCard.User.UserID, "User ID should not be empty")
		require.NotEmpty(t, feed.NoteCard.User.Nickname, "User nickname should not be empty")

		if feed.NoteCard.Type == "video" {
			require.NotNil(t, feed.NoteCard.Video, "Video info should not be nil for video type")
			if feed.NoteCard.Video != nil {
				require.True(t, feed.NoteCard.Video.Capa.Duration > 0, "Video duration should be greater than 0")
			}
		}

		// 只对第一个 Feed 进行完整 JSON 序列化检查
		if i == 0 {
			jsonData, err := json.MarshalIndent(feed, "", "  ")
			require.NoError(t, err, "Failed to marshal feed")

			fmt.Printf("\n第一个 Feed 的完整 JSON 结构:\n%s\n", string(jsonData))

			var checkFeed Feed
			err = json.Unmarshal(jsonData, &checkFeed)
			require.NoError(t, err, "Failed to unmarshal feed")

			require.Equal(t, feed.ID, checkFeed.ID)
			require.Equal(t, feed.ModelType, checkFeed.ModelType)
			require.Equal(t, feed.NoteCard.Type, checkFeed.NoteCard.Type)
		}

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
