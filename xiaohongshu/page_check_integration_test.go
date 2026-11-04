//go:build integration

package xiaohongshu

import (
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// TestCheckPageBlockedLocal 直接验证 CheckPageBlocked 这条 CDP 读取路径。
//
// 搜索用例走的是 JS 返回的 path，覆盖不到这里；而 CheckPageBlocked 恰恰是
// 「等待结束后页面才被换掉」时的兜底判定，必须单独覆盖。
func TestCheckPageBlockedLocal(t *testing.T) {
	b := newSearchFixtureBrowser(t)

	cases := []struct {
		name string
		url  string
		want error
	}{
		{
			name: "安全验证页",
			url:  "https://www.xiaohongshu.com/website-login/captcha",
			want: errors.ErrVerificationRequired,
		},
		{
			// 路径是登录页，只是查询串里带着验证页地址，不能被带偏。
			name: "登录页的 redirectPath 指向验证页",
			url:  "https://www.xiaohongshu.com/website-login/?redirectPath=/website-login/captcha",
			want: errors.ErrLoginRequired,
		},
		{
			name: "正常搜索结果页不应误判",
			url:  "https://www.xiaohongshu.com/search_result?keyword=fixture",
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := b.MustPage()
			defer page.Close()

			router := page.HijackRequests()
			router.MustAdd("*", func(h *rod.Hijack) {
				h.OnError = func(error) {}
				h.Response.SetHeader("Content-Type", "text/html; charset=utf-8")
				if strings.HasPrefix(h.Request.URL().Path, "/website-login") {
					h.Response.SetBody(`<!doctype html><meta charset="utf-8">拦截页`)
					return
				}
				h.Response.SetBody(`<!doctype html><meta charset="utf-8">正常页`)
			})
			go router.Run()
			defer func() { _ = router.Stop() }()

			require.NoError(t, page.Timeout(30*time.Second).Navigate(c.url))
			require.Equal(t, c.want, CheckPageBlocked(page))
		})
	}
}
