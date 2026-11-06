//go:build integration

package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// 本地页面全部拦截，不加载登录态，也不访问小红书。
func newSearchFixtureBrowser(t *testing.T, wrapClient ...func(rod.CDPClient) rod.CDPClient) *rod.Browser {
	t.Helper()
	bin := os.Getenv("XHS_TEST_BROWSER")
	if bin == "" {
		var ok bool
		bin, ok = launcher.LookPath()
		if !ok {
			t.Skip("未找到本地浏览器，可用 XHS_TEST_BROWSER 指定路径")
		}
	}
	l := launcher.New().Bin(bin).Headless(true)
	controlURL, err := l.Launch()
	require.NoError(t, err)
	b := rod.New().ControlURL(controlURL).NoDefaultDevice()
	if len(wrapClient) > 0 {
		client, err := cdp.StartWithURL(context.Background(), controlURL, nil)
		require.NoError(t, err)
		b = rod.New().Client(wrapClient[0](client)).NoDefaultDevice()
	}
	require.NoError(t, b.Connect())
	t.Cleanup(func() {
		_ = b.Close()
		l.Cleanup()
	})
	return b
}

type searchFixtureTiming struct {
	afterNavigate time.Duration
}

func (t searchFixtureTiming) Timing() humanize.TimingProfile {
	d := t.afterNavigate
	if d == 0 {
		d = time.Millisecond
	}
	return humanize.TimingProfile{
		humanize.AfterNavigate: {Mu: -7, Min: d, Max: d},
	}
}

func TestSearchReadyLocal(t *testing.T) {
	b := newSearchFixtureBrowser(t)

	cases := []struct {
		name     string
		want     int
		wantErr  string
		wantIs   error
		deadline time.Duration
		// afterNavigate 模拟生产里的导航后停顿（600ms~6s）。
		// 风控的重定向可能正好落在这段窗口里，必须能单独覆盖。
		afterNavigate time.Duration
	}{
		{name: "background", want: 1},
		{name: "dom", want: 1},
		{name: "slow_load", want: 1},
		{name: "render_delay", want: 1},
		{name: "ref_value", want: 1},
		{name: "stale_keyword", want: 1},
		{name: "body_empty_text", want: 1},
		{name: "empty"},
		{name: "empty_no_items"},
		{name: "api_error", wantErr: "搜索接口"},
		{name: "http_error", wantErr: "HTTP 503"},
		{name: "retry", want: 1},
		{name: "overlap_empty", want: 1},
		{name: "overlap_error", want: 1},
		{name: "unrelated", want: 1},
		{name: "later_page", want: 1},
		{name: "network_error", wantErr: "搜索请求未完成"},
		{name: "invalid_response", wantErr: "解析搜索响应"},
		{name: "never_ready", wantErr: "context deadline exceeded"},

		// 风控是延迟重定向：导航返回时页面还是搜索页，几百毫秒后主框架才被换掉。
		// 这两条覆盖「跳转发生在等待期间」——只在导航后判一次是抓不到的。
		{name: "redirect_captcha", wantIs: errors.ErrVerificationRequired, deadline: 3500 * time.Millisecond},
		{name: "redirect_login", wantIs: errors.ErrLoginRequired, deadline: 3500 * time.Millisecond},
		// 立刻替换：等价于导航直接落在验证页/登录页。
		{name: "redirect_captcha_now", wantIs: errors.ErrVerificationRequired, deadline: 3500 * time.Millisecond},
		{name: "redirect_login_now", wantIs: errors.ErrLoginRequired, deadline: 3500 * time.Millisecond},
		// 结果已就绪、搜索「成功」之后，风控才把页面换掉——落在 Search 的
		// AfterNavigate 延迟窗口里。这是上一版真正的漏网点：测试把该延迟压成
		// 1ms，正好绕开它，于是这里退回 ErrNoFeeds 也没人发现。
		{
			name:          "ready_then_redirect_captcha",
			wantIs:        errors.ErrVerificationRequired,
			deadline:      3500 * time.Millisecond,
			afterNavigate: time.Second,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			humanize.SetProvider(searchFixtureTiming{afterNavigate: tc.afterNavigate})
			t.Cleanup(func() { humanize.SetProvider(humanize.DefaultProvider{}) })

			page := b.MustPage()
			done := make(chan struct{})
			router := page.HijackRequests()
			var attempts atomic.Int32
			router.MustAdd("*", func(h *rod.Hijack) {
				h.OnError = func(error) {}
				path := h.Request.URL().Path
				h.Response.SetHeader("Content-Type", "text/html; charset=utf-8")
				switch {
				case strings.HasPrefix(path, "/search_result"):
					h.Response.SetBody(searchFixtureHTML(tc.name))
				case path == "/fixture-background" || path == "/fixture-slow-image":
					<-done
					h.Response.SetBody("")
				case path == "/api/sns/web/v2/search/notes":
					var request struct {
						Keyword string
						Page    int
					}
					_ = json.Unmarshal([]byte(h.Request.Body()), &request)
					attempt := attempts.Add(1)
					if strings.HasPrefix(tc.name, "overlap_") && attempt == 2 {
						time.Sleep(450 * time.Millisecond)
					} else {
						time.Sleep(120 * time.Millisecond)
					}
					h.Response.SetHeader("Content-Type", "application/json")
					body := `{"code":0,"success":true,"data":{"items":[{"id":"expected","model_type":"note"}],"has_more":false}}`
					switch {
					case request.Keyword == "other" || request.Page != 1:
						body = `{"code":-1,"success":false}`
					case tc.name == "overlap_empty" && attempt == 1:
						body = `{"code":0,"success":true,"data":{"has_more":false}}`
					case tc.name == "overlap_error" && attempt == 1:
						body = `{"code":-100,"success":false}`
					case tc.name == "empty":
						body = `{"code":0,"success":true,"data":{"items":[],"has_more":false}}`
					case tc.name == "empty_no_items":
						body = `{"code":0,"success":true,"data":{"has_more":false}}`
					case tc.name == "api_error" || (tc.name == "retry" && attempt == 1):
						body = `{"code":-100,"success":false,"msg":"fixture"}`
					case tc.name == "http_error":
						h.Response.Payload().ResponseCode = 503
						body = `{"code":-100,"success":false}`
					case tc.name == "network_error":
						h.Response.Fail(proto.NetworkErrorReasonFailed)
					case tc.name == "invalid_response":
						body = `not JSON`
					}
					h.Response.SetBody(body)
				default:
					h.Response.SetBody("")
				}
			})
			go router.Run()
			t.Cleanup(func() {
				close(done)
				_ = page.Close()
				_ = router.Stop()
			})

			deadline := tc.deadline
			if deadline == 0 {
				deadline = 2 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			var feeds []Feed
			var err error
			start := time.Now()
			require.NotPanics(t, func() { feeds, err = NewSearchAction(page).Search(ctx, tc.name) })
			elapsed := time.Since(start)

			if tc.wantIs != nil {
				// 必须能被 errors.Is 识别，调用方才能分流（引导扫码 vs 重新登录）。
				require.ErrorIs(t, err, tc.wantIs)
				// 必须在 deadline 前退出，而不是耗到超时只报一句 context deadline exceeded。
				require.NotErrorIs(t, err, context.DeadlineExceeded)
				require.Less(t, elapsed, 2*time.Second,
					"应在 deadline 之前退出，实际 %v（deadline %v）", elapsed, deadline)
				return
			}
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, feeds, tc.want)
			if tc.want > 0 {
				require.Equal(t, "expected", feeds[0].ID)
			}
		})
	}
}

// redirectFixtureHTML 风控重定向页：刻意不设置 window.__INITIAL_STATE__，
// 与真实的验证页/登录页一致——那里永远等不到搜索结果。
var redirectFixtureHTML = map[string]string{
	"redirect_captcha": `<!doctype html><meta charset="utf-8"><script>setTimeout(() => location.replace('/website-login/captcha'), 500)</script>`,
	// 登录页带上真实的 redirectPath 查询串：路径仍是登录页，不能被其中的
	// captcha 字样带偏。这样 JS 侧改成 location.href 或子串匹配都会被抓到。
	"redirect_login":       `<!doctype html><meta charset="utf-8"><script>setTimeout(() => location.replace('/website-login/?redirectPath=/website-login/captcha'), 500)</script>`,
	"redirect_captcha_now": `<!doctype html><meta charset="utf-8"><script>setTimeout(() => location.replace('/website-login/captcha'), 0)</script>`,
	"redirect_login_now":   `<!doctype html><meta charset="utf-8"><script>setTimeout(() => location.replace('/website-login/'), 0)</script>`,
}

func searchFixtureHTML(name string) string {
	if html, ok := redirectFixtureHTML[name]; ok {
		return html
	}

	keyword, _ := json.Marshal(name)
	return fmt.Sprintf(`<!doctype html><meta charset="utf-8"><div id="clock"></div>
<script>
const mode = %s;
const feedRef = mode === 'ref_value' ? {value: []} : {_value: []};
const search = window.__INITIAL_STATE__ = {search: {
    feeds: feedRef, state: {_value: 'loading'}, searchContext: {keyword: mode}
}};
const setFeeds = feeds => {feedRef[mode === 'ref_value' ? 'value' : '_value'] = feeds};
if (mode === 'body_empty_text' || mode === 'api_error') {
    document.body.insertAdjacentHTML('beforeend', '<div>暂无相关笔记</div>');
}
if (mode === 'empty' || mode === 'empty_no_items' || mode === 'never_ready') {
    // 实测筛选开始时就会出现这个容器，还不能据此认定为空结果。
    document.body.insertAdjacentHTML('beforeend', '<div class="search-empty-wrapper">没有筛选到相关内容</div>');
}
if (mode === 'stale_keyword') {
    search.search.searchContext.keyword = 'previous';
    setFeeds([{id: 'previous', modelType: 'note'}]);
    search.search.state._value = 'success';
}
if (mode === 'background') setInterval(() => fetch('/fixture-background').catch(() => {}), 60);
if (mode === 'dom') setInterval(() => document.getElementById('clock').textContent = String(Date.now()), 40);
if (mode === 'slow_load') document.body.insertAdjacentHTML('beforeend', '<img src="/fixture-slow-image">');
const request = (keyword, page = 1) => fetch('/api/sns/web/v2/search/notes', {
    method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({keyword, page})
}).then(async response => ({status: response.status, body: await response.json()}));
(async () => {
    if (mode === 'never_ready') return;
    if (mode === 'unrelated' || mode === 'later_page') {
        await request(mode === 'later_page' ? mode : 'other', mode === 'later_page' ? 2 : 1);
        search.search.state._value = 'error';
        await new Promise(resolve => setTimeout(resolve, 180));
        search.search.state._value = 'loading';
    }
    if (mode.startsWith('overlap_')) {
        // 站点对旧空响应/异常先设置 error，再检查当前搜索 ID。
        request(mode).then(() => {setFeeds([]); search.search.state._value = 'error'});
        await new Promise(resolve => setTimeout(resolve, 30));
    }
    let response = await request(mode);
    if (mode === 'retry' && !response.body.success) response = await request(mode);
    if (mode === 'render_delay' || mode === 'stale_keyword') await new Promise(resolve => setTimeout(resolve, 200));
    search.search.searchContext.keyword = mode;
    if (response.status !== 200 || !response.body.success || !response.body.data?.items?.length) {
        setFeeds([]);
        search.search.state._value = 'error';
        document.body.insertAdjacentHTML('beforeend', '<div class="search-empty-wrapper">没找到相关内容，换个词试试吧</div>');
        return;
    }
    setFeeds([{id: 'expected', modelType: 'note', noteCard: {displayTitle: 'local fixture'}}]);
    search.search.state._value = 'success';
    if (mode === 'ready_then_redirect_captcha') {
        // 结果已就绪之后才被风控换页，落在 Search 的 AfterNavigate 延迟窗口里。
        setTimeout(() => location.replace('/website-login/captcha'), 300);
    }
})().catch(() => {search.search.state._value = 'error'});
</script>`, keyword)
}
