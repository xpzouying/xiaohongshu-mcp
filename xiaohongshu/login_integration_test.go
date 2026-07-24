//go:build integration

package xiaohongshu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/require"
)

// TestLoginNavigateDoesNotWaitForSubresources 模拟页面子资源永久不结束，
// 防止登录导航重新引入 WaitLoad 导致接口无界挂起。
func TestLoginNavigateDoesNotWaitForSubresources(t *testing.T) {
	browserBin, found := launcher.LookPath()
	if !found {
		t.Skip("未找到本机 Chromium/Chrome")
	}

	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/never" {
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-release
			return
		}

		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><img src="/never"></body></html>`)
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	controlURL := launcher.New().
		Bin(browserBin).
		Headless(true).
		NoSandbox(true).
		MustLaunch()
	browser := rod.New().ControlURL(controlURL).MustConnect()
	t.Cleanup(func() { _ = browser.Close() })

	page := browser.MustPage()
	t.Cleanup(func() { _ = page.Close() })

	action := NewLogin(page)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startedAt := time.Now()
	_, err := action.navigate(ctx, server.URL)
	require.NoError(t, err)
	require.Less(t, time.Since(startedAt), 5*time.Second)
}
