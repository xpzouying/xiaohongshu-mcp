//go:build integration

// 集成测试：起有头浏览器 + 需登录态 + 触网，默认 go test 不编译不运行。
// 只填到发布页校验标签，不点「发布」，不产生笔记。
//
// 手动跑：
//
//	XHS_TEST_IMAGE=/path/to.jpg XHS_TEST_SHOT=/tmp/shot.png \
//	  go test -tags integration ./xiaohongshu/ -run TestInputTagsCreatesTopic -v -timeout 15m
package xiaohongshu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInputTagsCreatesTopic 回归：标签必须真的成为话题节点，而不是留在正文里的纯文本。
//
// 旧实现固定 sleep 1s 后直接点第一个 .item。编辑器是 ProseMirror，联想列表随输入防抖
// 重渲染，取到的节点在按下鼠标前就可能被替换（Shape 取不到 quad），点击报
// 「元素无可点击区域」，整单发布失败。
func TestInputTagsCreatesTopic(t *testing.T) {
	image := os.Getenv("XHS_TEST_IMAGE")
	if image == "" {
		t.Skip("SKIP: 需要 XHS_TEST_IMAGE 指定一张本地图片")
	}
	if _, err := os.Stat(image); err != nil {
		t.Skipf("SKIP: 测试图片不可用: %v", err)
	}

	const tag = "gpt订阅"

	ctx := context.Background()

	b := browser.NewBrowser(false)
	defer b.Close()

	// 给整条流程一个上限：登录态失效时 MustElement 会一直重试到 ctx 结束。
	page := b.NewPage().Timeout(4 * time.Minute)
	defer page.Close()

	login := NewLogin(page)
	loggedIn, err := login.CheckLoginStatus(ctx)
	require.NoError(t, err, "登录态检查失败")
	if !loggedIn {
		t.Fatal("未登录：COOKIES_PATH 指向的 cookies 已失效，需重新扫码")
	}

	action, err := NewPublishImageAction(page)
	require.NoError(t, err)

	require.NoError(t, uploadImages(action.page, []string{image}))

	// 与 submitPublish 保持同样的顺序：标题 → 正文 → 关引导卡 → 回点标题 → 标签
	titleElem, err := action.page.Element("div.d-input input")
	require.NoError(t, err)
	require.NoError(t, humanize.Type(ctx, titleElem, "标签回归验证（不会发布）"))

	contentElem, err := getContentElement(action.page, contentElemTimeout)
	require.NoError(t, err)
	require.NoError(t, humanize.Type(ctx, contentElem, "只验证话题标签，不点发布。"))

	closeFeatureGuide(action.page)
	require.NoError(t, waitAndClickTitleInput(titleElem))
	require.NoError(t, inputTags(ctx, contentElem, []string{tag}))

	// 话题会渲染成 <a class="tiptap-topic" data-topic='{"name":"..."}'>
	topicElem, err := contentElem.Timeout(3 * time.Second).Element("a.tiptap-topic")
	require.NoError(t, err, "标签没有成为话题节点，可能退化成了纯文本")

	raw, err := topicElem.Attribute("data-topic")
	require.NoError(t, err)
	require.NotNil(t, raw, "话题节点缺少 data-topic")
	assert.Contains(t, *raw, tag)

	text, err := topicElem.Text()
	require.NoError(t, err)
	assert.Contains(t, text, tag)

	if shot := os.Getenv("XHS_TEST_SHOT"); shot != "" {
		action.page.MustScreenshot(shot)
		t.Logf("截图已保存: %s", shot)
	}
}
