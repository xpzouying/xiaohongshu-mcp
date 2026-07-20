//go:build integration

// 集成测试：起有头浏览器 + 需登录态，默认 go test 不编译不运行。
// 发布是写操作，额外保留 t.Skip 双保险，避免 -tags integration 时误发帖。
package xiaohongshu

import (
	"context"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/browser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {

	t.Skip("SKIP: 写操作（发帖），手动去掉此行才真正执行")

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action, err := NewPublishImageAction(page)
	require.NoError(t, err)

	err = action.Publish(context.Background(), PublishImageContent{
		Title:      "Hello World",
		Content:    "Hello World",
		ImagePaths: []string{"/tmp/1.jpg"},
	})
	assert.NoError(t, err)
}
