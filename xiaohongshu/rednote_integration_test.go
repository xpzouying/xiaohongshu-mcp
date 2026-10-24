//go:build integration

package xiaohongshu

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

// 本地页面验证关键分支，不登录、不访问真实站点、不发布笔记。
func fixturePage(t *testing.T) *rod.Page {
	t.Helper()
	t.Setenv("COOKIES_PATH", t.TempDir()+"/cookies.json")
	b := browser.NewBrowser(true)
	t.Cleanup(b.Close)
	page := b.NewPage().Timeout(20 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	router := page.HijackRequests()
	router.MustAdd("*", func(h *rod.Hijack) {
		h.Response.SetHeader("Content-Type", "text/html").SetBody(`<html><body><input type="file"></body></html>`)
	})
	go router.Run()
	t.Cleanup(func() { require.NoError(t, router.Stop()) })
	page.MustNavigate(webURL("/explore")).MustWaitLoad()
	return page
}

func TestRedNoteLoginStateFixtures(t *testing.T) {
	t.Setenv("XHS_SITE", "rednote")
	page := fixturePage(t)
	for _, tt := range []struct {
		name, state string
		want        bool
	}{
		{"signed in", `{loggedIn:true,userInfo:{userId:'123',guest:false}}`, true},
		{"wrapped state", `{loggedIn:{value:true},userInfo:{value:{user_id:'123'}}}`, true},
		{"explicit logout", `{loggedIn:false,userInfo:{userId:'123'}}`, false},
		{"wrapped logout", `{loggedIn:{value:false},userInfo:{value:{userId:'123'}}}`, false},
		{"guest", `{loggedIn:true,userInfo:{userId:'guest',guest:true}}`, false},
		{"flag without identity", `{loggedIn:true}`, false},
		{"empty identity", `{userInfo:{userId:' '}}`, false},
		{"missing state", `undefined`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			page.MustSetDocumentContent(`<html><body><script>window.__INITIAL_STATE__={user:` + tt.state + `}</script></body></html>`)
			require.Equal(t, tt.want, loginPageShowsSignedInUser(page))
		})
	}
	page.MustNavigate("https://rednote.com/explore").MustWaitLoad()
	page.MustSetDocumentContent(`<html><body><script>window.__INITIAL_STATE__={user:{loggedIn:true,userInfo:{userId:'123'}}}</script></body></html>`)
	require.True(t, loginPageShowsSignedInUser(page))
	page.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()
	page.MustSetDocumentContent(`<html><body><script>window.__INITIAL_STATE__={user:{loggedIn:true,userInfo:{userId:'123'}}}</script></body></html>`)
	require.False(t, loginPageShowsSignedInUser(page))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.False(t, NewLogin(page).WaitForLogin(ctx))
}

func TestDraftButtonAndConfirmationFixtures(t *testing.T) {
	page := fixturePage(t)
	for _, label := range []string{"保存草稿", "暂存离开", "Save draft"} {
		t.Run(label, func(t *testing.T) {
			page.MustSetDocumentContent(`<html><body><button onclick="document.body.dataset.published='yes'">发布</button><button style="display:none">保存草稿</button><button onclick="document.body.dataset.saved='yes';document.getElementById('result').textContent='暂存成功'">` + label + `</button><div role="alert" id="result"></div></body></html>`)
			require.NoError(t, clickSaveDraftButton(page))
			require.NoError(t, waitDraftSaved(page, time.Second))
			require.Equal(t, "yes", *page.MustElement("body").MustAttribute("data-saved"))
			require.Nil(t, page.MustElement("body").MustAttribute("data-published"))
		})
	}
	page.MustSetDocumentContent(`<html><body><button>发布</button><button>保存并发布</button></body></html>`)
	require.Error(t, clickSaveDraftButton(page))
	page.MustSetDocumentContent(`<html><body><div role="alert">草稿保存失败</div><div role="alert" style="display:none">草稿已保存</div></body></html>`)
	require.Error(t, waitDraftSaved(page, 10*time.Millisecond))
}

func TestCreatorSessionAndPublishResultFixtures(t *testing.T) {
	t.Setenv("XHS_SITE", "rednote")
	page := fixturePage(t)
	page.MustNavigate(creatorURL("/publish/publish")).MustWaitLoad()
	require.True(t, hasCreatorSession(page))
	require.NoError(t, ensureCreatorSession(page))
	page.MustSetDocumentContent(`<html><body><form>Sign in</form></body></html>`)
	require.False(t, hasCreatorSession(page), "发布 URL 本身不足以证明登录")
	page.MustNavigate(creatorURL("/login?lastUrl=/publish/success")).MustWaitLoad()
	require.False(t, hasCreatorSession(page))
	require.Error(t, waitPublishSuccess(page, 10*time.Millisecond))
	page.MustNavigate(creatorURL("/publish/success?source=official")).MustWaitLoad()
	require.NoError(t, waitPublishSuccess(page, time.Second))
	page.MustNavigate("https://creator.xiaohongshu.com/publish/success").MustWaitLoad()
	require.Error(t, waitPublishSuccess(page, 10*time.Millisecond))
}
