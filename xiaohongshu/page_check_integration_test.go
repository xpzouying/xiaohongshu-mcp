//go:build integration

// 集成测试：起浏览器 + 本地 fixture。不触网、不需要登录态。
// 默认 go test 不编译不运行。
// 手动跑：go test -tags integration ./xiaohongshu/ -run TestCheckPageBlocked
package xiaohongshu

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// newBlockFixture 复刻安全验证页与登录页的路径。
//
// 真实的风控验证页无法按需触发，所以用本地页面替代。这里被验证的是
// "读到当前地址并据此判定" 这段代码——尤其是取地址的方式是否真的有效
// （page.Info() 的 URL 字段实测为空，换回它会静默失效）。
func newBlockFixture(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/website-login/captcha", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>安全验证</title></head><body>x</body></html>`)
	})
	mux.HandleFunc("/website-login/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>登录</title></head><body>x</body></html>`)
	})
	mux.HandleFunc("/search_result", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>搜索结果</title></head><body>x</body></html>`)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func TestCheckPageBlocked(t *testing.T) {
	srv := newBlockFixture(t)

	b := browser.NewBrowser(true)
	defer b.Close()

	cases := []struct {
		name string
		path string
		want error
	}{
		{"安全验证页", "/website-login/captcha", errors.ErrVerificationRequired},
		{"登录页", "/website-login/", errors.ErrLoginRequired},
		{"正常页面不应被误判", "/search_result", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := b.NewPage()
			defer page.Close()

			p := page.Timeout(30 * time.Second)
			require.NoError(t, p.Navigate(srv.URL+c.path))

			require.Equal(t, c.want, CheckPageBlocked(p))
		})
	}
}
