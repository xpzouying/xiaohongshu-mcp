//go:build integration

package xiaohongshu

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

func TestCreatorPublishPageLocal(t *testing.T) {
	b := newSearchFixtureBrowser(t)

	cases := []struct {
		name       string
		open       func(*rod.Page) (*PublishAction, error)
		body       string
		timeout    time.Duration
		wantAccept string
		wantErr    error
	}{
		{
			name: "图文经过短暂登录页", open: NewPublishImageAction, body: creatorUploadAfterLogin(),
			timeout: 6 * time.Second, wantAccept: "image/*",
		},
		{
			name: "视频经过短暂登录页", open: NewPublishVideoAction, body: creatorUploadAfterLogin(),
			timeout: 6 * time.Second, wantAccept: "video/*",
		},
		{
			name: "持续停在登录页", open: NewPublishImageAction,
			body:    "<script>history.replaceState(null, '', '/login')</script>",
			timeout: 1500 * time.Millisecond, wantErr: errors.ErrLoginRequired,
		},
		{
			name: "进入安全验证页", open: NewPublishImageAction,
			body:    "<script>history.replaceState(null, '', '/website-login/captcha')</script>",
			timeout: 3 * time.Second, wantErr: errors.ErrVerificationRequired,
		},
		{
			name: "上传区域有占位但没有文件控件", open: NewPublishImageAction,
			body:    "<div class='upload-content'>加载中</div>",
			timeout: 1500 * time.Millisecond, wantErr: context.DeadlineExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := b.MustPage()
			defer page.Close()

			// 模拟主站有 Cookie、创作者中心仍要求登录。
			require.NoError(t, page.SetCookies([]*proto.NetworkCookieParam{{
				Name: "web_session", Value: "fixture", Domain: ".xiaohongshu.com", Path: "/", Secure: true,
			}}))
			router := page.HijackRequests()
			var visits atomic.Int32
			router.MustAdd("*", func(h *rod.Hijack) {
				h.Response.SetHeader("Content-Type", "text/html; charset=utf-8")
				if h.Request.URL().Path == creatorPublishPath {
					visits.Add(1)
					h.Response.SetBody("<!doctype html><meta charset=utf-8><body>" + tc.body)
				}
			})
			go router.Run()
			defer router.Stop()

			probe := page.Timeout(tc.timeout)
			defer probe.CancelTimeout()
			action, err := tc.open(probe)
			require.EqualValues(t, 1, visits.Load(), "只导航到发布页一次")
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, action)
				return
			}

			require.NoError(t, err)
			input, err := action.page.Element("div.upload-content input[type=file]")
			require.NoError(t, err)
			accept, err := input.Attribute("accept")
			require.NoError(t, err)
			require.NotNil(t, accept)
			require.Equal(t, tc.wantAccept, *accept)
		})
	}
}

func creatorUploadAfterLogin() string {
	return fmt.Sprintf(`<script>
history.replaceState(null, '', '/login');
setTimeout(() => {
  history.replaceState(null, '', %q);
  document.body.innerHTML = %q;
}, 450);
</script>`, creatorPublishPath, `
<div class="creator-tab">上传图文</div>
<div class="creator-tab">上传视频</div>
<div class="upload-content"><input type="file" accept="video/*"></div>`) + `
<script>
document.addEventListener('click', event => {
  if (!event.target.matches('.creator-tab')) return;
  document.querySelector('input[type=file]').accept =
    event.target.textContent === '上传图文' ? 'image/*' : 'video/*';
});
</script>`
}
