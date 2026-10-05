//go:build integration

// 集成测试：只需要内置浏览器，不需要登录态、不需要网络。
// 默认 go test 不编译不运行。手动跑：
//
//	go test -tags integration ./xiaohongshu/ -run TestUploadImagesFailsFast -v
package xiaohongshu

import (
	"testing"

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
