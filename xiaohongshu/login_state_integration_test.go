//go:build integration

package xiaohongshu

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
)

// 本地拦截页面，验证 isLoggedIn 按页面自己的登录态判断，不依赖 /explore 的侧栏。
func TestIsLoggedInReadsPageState(t *testing.T) {
	b := newSearchFixtureBrowser(t)
	cases := []struct {
		name, body string
		want       bool
	}{
		{"logged_in_ref", `<script>window.__INITIAL_STATE__ = {user: {loggedIn: {_value: true}, userInfo: {_value: {guest: false, userId: "u"}}}}</script>`, true},
		{"logged_in_value", `<script>window.__INITIAL_STATE__ = {user: {loggedIn: {value: true}}}</script>`, true},
		{"user_info_only", `<script>window.__INITIAL_STATE__ = {user: {loggedIn: {_value: false}, userInfo: {_value: {guest: false, user_id: "u"}}}}</script>`, true},
		{"guest", `<script>window.__INITIAL_STATE__ = {user: {loggedIn: {_value: false}, userInfo: {_value: {guest: true}}}}</script>`, false},
		{"no_state", ``, false},
		{"sidebar", `<div class="main-container"><div class="user"><div class="link-wrapper"><span class="channel">我</span></div></div></div>`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := b.MustPage()
			router := page.HijackRequests()
			router.MustAdd("*", func(h *rod.Hijack) {
				h.Response.SetHeader("Content-Type", "text/html; charset=utf-8")
				h.Response.SetBody(`<!doctype html><meta charset="utf-8">` + tc.body)
			})
			go router.Run()
			t.Cleanup(func() {
				_ = page.Close()
				_ = router.Stop()
			})
			require.NoError(t, page.Navigate(loginPageURL))
			require.NoError(t, page.WaitLoad())
			require.Equal(t, tc.want, isLoggedIn(page))
		})
	}
}
