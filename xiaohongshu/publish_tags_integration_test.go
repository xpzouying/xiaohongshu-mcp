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

	"github.com/go-rod/rod"
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
//
// 一次会话里连续插多个标签：既拿到多个样本（旧实现是概率性失败，单个标签一次通过说明
// 不了什么），也覆盖「上一个标签的联想结果还没刷掉、点到它」这个点错话题的场景。
func TestInputTagsCreatesTopic(t *testing.T) {
	image := os.Getenv("XHS_TEST_IMAGE")
	if image == "" {
		t.Skip("SKIP: 需要 XHS_TEST_IMAGE 指定一张本地图片")
	}
	if _, err := os.Stat(image); err != nil {
		t.Skipf("SKIP: 测试图片不可用: %v", err)
	}

	tags := []string{"gpt订阅", "ChatGPT", "人工智能", "程序员", "效率工具"}

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
	require.NoError(t, inputTags(ctx, contentElem, tags))

	if shot := os.Getenv("XHS_TEST_SHOT"); shot != "" {
		action.page.MustScreenshot(shot)
		t.Logf("截图已保存: %s", shot)
	}

	// 话题会渲染成 <a class="tiptap-topic" data-topic='{"name":"..."}'>
	topicElems := waitTopicElems(t, contentElem, len(tags))
	require.Len(t, topicElems, len(tags), "话题节点数少于标签数，有标签退化成纯文本或没插进去")

	// 顺序也要对得上：第 i 个话题节点必须就是第 i 个标签，否则说明点到了残留的联想项。
	for i, elem := range topicElems {
		raw, err := elem.Attribute("data-topic")
		require.NoError(t, err)
		require.NotNil(t, raw, "第 %d 个话题节点缺少 data-topic", i+1)
		assert.Contains(t, *raw, tags[i], "第 %d 个话题与请求的标签不符（可能点到了残留的联想项）", i+1)

		text, err := elem.Text()
		require.NoError(t, err)
		assert.Contains(t, text, tags[i])
	}
}

// waitTopicElems 等话题节点渲染到 want 个。Element.Elements 是单次查询、不重试，
// 而 ProseMirror 插入话题后会重渲染，所以这里自己轮询。
func waitTopicElems(t *testing.T, contentElem *rod.Element, want int) []*rod.Element {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		elems, err := contentElem.Elements("a.tiptap-topic")
		require.NoError(t, err)
		if len(elems) >= want || time.Now().After(deadline) {
			return elems
		}
		time.Sleep(200 * time.Millisecond)
	}
}
