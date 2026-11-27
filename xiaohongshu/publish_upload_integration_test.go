//go:build integration

// 集成测试：需要内置浏览器；成功路径那条还需要登录态（要走创作中心发布页）。
// 默认 go test 不编译不运行。手动跑：
//
//	go test -tags integration ./xiaohongshu/ -run TestUploadImagesFailsFast -v
//
//	XHS_TEST_IMAGE=/path/to.jpg XHS_TEST_SHOT=/tmp/shot.png \
//	  go test -tags integration ./xiaohongshu/ -run TestUploadImagesAcceptsValidImage -v -timeout 10m
package xiaohongshu

import (
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/xpzouying/xiaohongshu-mcp/browser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUploadImagesFailsFastOnMissingImage 回归：图片缺失时必须在动页面之前就报错。
//
// 原先只 warn 后跳过该张继续传，调用方以为图片都发出去了，而 service 的响应又按请求
// 张数回，结果「少发图但报发布完成」。
//
// 这里故意用空白页：校验发生在上传之前，不该触碰 DOM——所以报错必须点名缺失的文件，
// 而不是「查找上传输入框失败」。
func TestUploadImagesFailsFastOnMissingImage(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	if err != nil {
		t.Skipf("SKIP: 浏览器不可用: %v", err)
	}

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank")
	page.MustWaitLoad()

	const missing = "/nonexistent/xhs-missing-image.jpg"

	err = uploadImages(page, []string{missing})
	require.Error(t, err, "图片缺失时不应返回成功")
	assert.Contains(t, err.Error(), missing)
	assert.NotContains(t, err.Error(), "上传输入框", "校验必须在查找上传输入框之前")
}

// TestUploadImagesAcceptsValidImage 回归：有效图片必须真的上传出预览。
//
// 与上一条互为对照——上一条保证「缺图片不静默跳过」，这一条保证「改成先整体校验之后，
// 有效图片仍然照常上传」。只填到发布页，不点「发布」。
func TestUploadImagesAcceptsValidImage(t *testing.T) {
	image := os.Getenv("XHS_TEST_IMAGE")
	if image == "" {
		t.Skip("SKIP: 需要 XHS_TEST_IMAGE 指定一张本地图片")
	}
	if _, err := os.Stat(image); err != nil {
		t.Skipf("SKIP: 测试图片不可用: %v", err)
	}

	b := browser.NewBrowser(false)
	defer b.Close()

	// 给整条流程一个上限：登录态失效时 MustElement 会一直重试到 ctx 结束。
	page := b.NewPage().Timeout(4 * time.Minute)
	defer page.Close()

	action, err := NewPublishImageAction(page)
	require.NoError(t, err)

	require.NoError(t, uploadImages(action.page, []string{image}))

	previews, err := action.page.Elements(".img-preview-area .pr")
	require.NoError(t, err)
	assert.Len(t, previews, 1, "有效图片应上传出 1 张预览")

	if shot := os.Getenv("XHS_TEST_SHOT"); shot != "" {
		action.page.MustScreenshot(shot)
		t.Logf("截图已保存: %s", shot)
	}
}
