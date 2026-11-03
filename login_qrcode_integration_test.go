//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/defaults"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func TestLoginQrcodeClosesBrowserOnNavigationPanic(t *testing.T) {
	// 首次下载浏览器的时间不计入取码超时。
	_, err := browser.EnsureBrowser()
	require.NoError(t, err)

	var sawNavigation atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect && r.Host == "www.xiaohongshu.com:443" {
			sawNavigation.Store(true)
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()

	oldProxy, oldDir := configs.Proxy(), defaults.Dir
	defer func() {
		configs.SetProxy(oldProxy)
		defaults.Dir = oldDir
	}()
	configs.SetProxy(proxy.URL)
	profileDir := t.TempDir()
	defaults.Dir = profileDir
	t.Setenv("COOKIES_PATH", filepath.Join(t.TempDir(), "cookies.json"))

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	require.Panics(t, func() { NewXiaohongshuService().GetLoginQrcode(ctx) })
	require.True(t, sawNavigation.Load())
	// rod 在浏览器退出后才删除 profile。
	_, err = os.Lstat(profileDir)
	require.ErrorIs(t, err, os.ErrNotExist)
}
