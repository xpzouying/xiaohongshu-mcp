package main

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

type toolWebCoverage struct {
	tool       string
	method     string
	webPath    string
	upstream   string
	page       string
	script     string
	status     string
	entryToken string
}

var mcpWebCoverage = []toolWebCoverage{
	{"list_accounts", http.MethodGet, "/api/web/accounts", "/api/v1/accounts", "accounts.html", "accounts.js", "complete", "refresh-accounts"},
	{"create_account", http.MethodPost, "/api/web/accounts", "/api/v1/accounts", "accounts.html", "accounts.js", "complete", "advanced-create"},
	{"remove_account", http.MethodDelete, "/api/web/accounts/acct_one", "/api/v1/accounts/acct_one", "accounts.html", "accounts.js", "complete", `data-action="remove"`},
	{"set_default_account", http.MethodPut, "/api/web/accounts/acct_one/default", "/api/v1/accounts/acct_one/default", "accounts.html", "accounts.js", "complete", `data-action="default"`},
	{"check_login_status", http.MethodPost, "/api/web/accounts/acct_one/login/status", "/api/v1/accounts/acct_one/login/status", "accounts.html", "accounts.js", "complete", `data-action="status"`},
	{"get_login_qrcode", http.MethodPost, "/api/web/accounts/acct_one/login/qrcode", "/api/v1/accounts/acct_one/login/qrcode", "accounts.html", "accounts.js", "complete", `data-action="qr"`},
	{"reset_login", http.MethodDelete, "/api/web/accounts/acct_one/login", "/api/v1/accounts/acct_one/login", "accounts.html", "accounts.js", "complete", `data-action="reset"`},
	{"publish_content", http.MethodPost, "/api/web/publish", "/api/v1/publish", "publish.html", "publish.js", "complete", "'publish_content'"},
	{"publish_with_video", http.MethodPost, "/api/web/publish_video", "/api/v1/publish_video", "publish.html", "publish.js", "complete", "'publish_with_video'"},
	{"list_feeds", http.MethodGet, "/api/web/feeds/list", "/api/v1/feeds/list", "discover.html", "discover.js", "complete", "'list_feeds'"},
	{"search_feeds", http.MethodPost, "/api/web/feeds/search", "/api/v1/feeds/search", "discover.html", "discover.js", "complete", "'search_feeds'"},
	{"get_feed_detail", http.MethodPost, "/api/web/feeds/detail", "/api/v1/feeds/detail", "detail.html", "detail.js", "complete", "'get_feed_detail'"},
	{"user_profile", http.MethodPost, "/api/web/user/profile", "/api/v1/user/profile", "profile.html", "profile.js", "complete", "'user_profile'"},
	{"post_comment_to_feed", http.MethodPost, "/api/web/feeds/comment", "/api/v1/feeds/comment", "detail.html", "detail.js", "complete", "'post_comment_to_feed'"},
	{"reply_comment_in_feed", http.MethodPost, "/api/web/feeds/comment/reply", "/api/v1/feeds/comment/reply", "detail.html", "detail.js", "complete", "'reply_comment_in_feed'"},
	{"like_feed", http.MethodPost, "/api/web/feeds/like", "/api/v1/feeds/like", "detail.html", "detail.js", "complete", "'like_feed'"},
	{"favorite_feed", http.MethodPost, "/api/web/feeds/favorite", "/api/v1/feeds/favorite", "detail.html", "detail.js", "complete", "'favorite_feed'"},
}

func TestMCPWebCoverageBaseline(t *testing.T) {
	wantTools := []string{
		"check_login_status", "create_account", "favorite_feed", "get_feed_detail",
		"get_login_qrcode", "like_feed", "list_accounts", "list_feeds",
		"post_comment_to_feed", "publish_content", "publish_with_video", "remove_account",
		"reply_comment_in_feed", "reset_login", "search_feeds", "set_default_account", "user_profile",
	}
	gotTools := make([]string, 0, len(mcpWebCoverage))
	statusCounts := map[string]int{}
	for _, coverage := range mcpWebCoverage {
		gotTools = append(gotTools, coverage.tool)
		statusCounts[coverage.status]++
	}
	sort.Strings(gotTools)
	if strings.Join(gotTools, ",") != strings.Join(wantTools, ",") {
		t.Fatalf("MCP Web UI tool baseline = %v, want %v", gotTools, wantTools)
	}
	if statusCounts["complete"] != 17 || statusCounts["partial"] != 0 || statusCounts["missing"] != 0 {
		t.Fatalf("coverage status counts = %v, want complete=17 partial=0 missing=0", statusCounts)
	}
}

func TestAccountPageExposesCompleteCreateAndLoginStatusFlows(t *testing.T) {
	page, err := fs.ReadFile(staticFiles, "static/accounts.html")
	if err != nil {
		t.Fatal(err)
	}
	script, err := fs.ReadFile(staticFiles, "static/accounts.js")
	if err != nil {
		t.Fatal(err)
	}
	pageText, scriptText := string(page), string(script)
	for _, token := range []string{"advanced-create", `name="account_id"`, `name="display_name"`, `name="owner"`, `name="purpose"`} {
		if !strings.Contains(pageText, token) {
			t.Errorf("accounts.html missing %q", token)
		}
	}
	for _, token := range []string{"createAccount", "validateAccountId", `data-action="status"`, "checkLoginStatus", "'check_login_status'"} {
		if !strings.Contains(scriptText, token) {
			t.Errorf("accounts.js missing %q", token)
		}
	}
}

func TestMCPWebCoverageProxyContracts(t *testing.T) {
	for _, coverage := range mcpWebCoverage {
		t.Run(coverage.tool, func(t *testing.T) {
			got, allowed := proxyPath(coverage.method, coverage.webPath)
			if !allowed || got != coverage.upstream {
				t.Fatalf("proxyPath(%s, %s) = %q, %v; want %q, true", coverage.method, coverage.webPath, got, allowed, coverage.upstream)
			}
		})
	}
}

func TestMCPWebCoverageStaticContracts(t *testing.T) {
	for _, coverage := range mcpWebCoverage {
		t.Run(coverage.tool, func(t *testing.T) {
			page, err := fs.ReadFile(staticFiles, "static/"+coverage.page)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(page), `/static/`+coverage.script) {
				t.Fatalf("%s does not load %s", coverage.page, coverage.script)
			}
			if coverage.entryToken == "" {
				return
			}
			script, err := fs.ReadFile(staticFiles, "static/"+coverage.script)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(page), coverage.entryToken) && !strings.Contains(string(script), coverage.entryToken) {
				t.Fatalf("%s has no static entry token %q", coverage.tool, coverage.entryToken)
			}
		})
	}
}

func TestProfilePageMapsResponseAndEscapesDynamicValues(t *testing.T) {
	page, err := fs.ReadFile(staticFiles, "static/profile.html")
	if err != nil {
		t.Fatal(err)
	}
	script, err := fs.ReadFile(staticFiles, "static/profile.js")
	if err != nil {
		t.Fatal(err)
	}

	pageText, scriptText := string(page), string(script)
	for _, token := range []string{"profile-content", "profile-feeds", "/static/profile.js"} {
		if !strings.Contains(pageText, token) {
			t.Errorf("profile.html missing token %q", token)
		}
	}
	for _, token := range []string{
		"profileParams.get('user_id')", "profileParams.get('xsec_token')", "'user_profile'",
		"userBasicInfo", "interactions", "feeds", "imageb", "nickname", "redId", "desc", "ipLocation",
		"XHS.escapeHTML", "encodeURIComponent(String(feed.id", "encodeURIComponent(String(feed.xsecToken",
	} {
		if !strings.Contains(scriptText, token) {
			t.Errorf("profile.js missing contract token %q", token)
		}
	}
}

func TestDetailPageMapsAdvancedCommentOptionsAndProfileLinks(t *testing.T) {
	page, err := fs.ReadFile(staticFiles, "static/detail.html")
	if err != nil {
		t.Fatal(err)
	}
	script, err := fs.ReadFile(staticFiles, "static/detail.js")
	if err != nil {
		t.Fatal(err)
	}

	pageText, scriptText := string(page), string(script)
	for _, parameter := range []string{
		"load_all_comments", "click_more_replies", "limit", "reply_limit", "scroll_speed",
	} {
		if !strings.Contains(pageText, `name="`+parameter+`"`) {
			t.Errorf("detail.html missing input for %s", parameter)
		}
		if !strings.Contains(scriptText, parameter+":") {
			t.Errorf("detail.js missing request mapping for %s", parameter)
		}
	}
	for _, token := range []string{
		"'get_feed_detail'", "/profile.html?user_id=", "encodeURIComponent(userId)",
		"encodeURIComponent(detailState.token)", "XHS.escapeHTML(comment.content || '')", "AbortController",
		"cancel-detail", "detail-error", "setPending", "validateReply",
	} {
		if !strings.Contains(scriptText, token) {
			t.Errorf("detail.js missing contract token %q", token)
		}
	}
}

func TestHealth(t *testing.T) {
	h := newHandler(handlerConfig{})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/web/health", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"healthy"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestSecurityHeadersAllowHTTPSMediaAndKeepImagesStrict(t *testing.T) {
	h := newHandler(handlerConfig{})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/detail.html", nil))

	csp := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "media-src 'self' https:") {
		t.Fatalf("CSP missing HTTPS media policy: %q", csp)
	}
	if !strings.Contains(csp, "img-src 'self' data: https:") || strings.Contains(csp, "img-src 'self' data: http:") {
		t.Fatalf("CSP image policy is not strict: %q", csp)
	}
}

func TestEmbeddedStaticFiles(t *testing.T) {
	h := newHandler(handlerConfig{})
	for _, path := range []string{
		"/", "/accounts.html", "/discover.html", "/publish.html", "/detail.html", "/profile.html",
		"/static/app.css", "/static/app.js", "/static/mcp-contract.js", "/static/accounts.js", "/static/discover.css", "/static/discover.js", "/static/profile.js",
	} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, body = %s", path, response.Code, response.Body.String())
		}
	}
}

func TestLegacySearchPageRedirectsToDiscover(t *testing.T) {
	h := newHandler(handlerConfig{})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/search.html", nil))

	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTemporaryRedirect)
	}
	if location := response.Header().Get("Location"); location != "/discover.html" {
		t.Fatalf("Location = %q, want /discover.html", location)
	}
}

func TestDiscoverStaticContracts(t *testing.T) {
	page, err := fs.ReadFile(staticFiles, "static/discover.html")
	if err != nil {
		t.Fatal(err)
	}
	script, err := fs.ReadFile(staticFiles, "static/discover.js")
	if err != nil {
		t.Fatal(err)
	}
	pageText, scriptText := string(page), string(script)
	for _, token := range []string{
		"recommended-panel", "search-panel", "sort_by", "note_type", "publish_time", "search_scope", "location",
		">综合<", ">最新<", ">最多点赞<", ">最多评论<", ">最多收藏<",
		">视频<", ">图文<", ">一天内<", ">一周内<", ">半年内<",
		">已看过<", ">未看过<", ">已关注<", ">同城<", ">附近<",
	} {
		if !strings.Contains(pageText, token) {
			t.Errorf("discover.html missing %q", token)
		}
	}
	for _, token := range []string{"'list_feeds'", "'search_feeds'", "XHS.escapeHTML", "encodeURIComponent(feed.id", "encodeURIComponent(user.userId", "encodeURIComponent(feed.xsecToken"} {
		if !strings.Contains(scriptText, token) {
			t.Errorf("discover.js missing %q", token)
		}
	}
}

func TestProxyForwardsValidatedRequest(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	h := newHandler(handlerConfig{upstreamURL: upstream.URL})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/web/accounts?source=ui", strings.NewReader(`{"id":"acct_one"}`))
	request.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Body.String() != `{"ok":true}` {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/accounts" || gotQuery != "source=ui" || gotBody != `{"id":"acct_one"}` {
		t.Fatalf("forwarded request = %s %s?%s %s", gotMethod, gotPath, gotQuery, gotBody)
	}
}

func TestProxyMapsBusinessRouteToUpstreamV1(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	h := newHandler(handlerConfig{upstreamURL: upstream.URL})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/web/feeds/search", strings.NewReader(`{"keyword":"test"}`)))

	if response.Code != http.StatusNoContent || gotPath != "/api/v1/feeds/search" {
		t.Fatalf("status = %d, upstream path = %q", response.Code, gotPath)
	}
}

func TestProxyRejectsOversizedBodyBeforeUpstream(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { upstreamCalls++ }))
	defer upstream.Close()
	h := newHandler(handlerConfig{upstreamURL: upstream.URL})
	response := httptest.NewRecorder()
	body := `{"keyword":"` + strings.Repeat("x", maxProxyRequestBody) + `"}`
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/web/feeds/search", strings.NewReader(body)))

	if response.Code != http.StatusRequestEntityTooLarge || upstreamCalls != 0 {
		t.Fatalf("status=%d body=%s upstream calls=%d", response.Code, response.Body.String(), upstreamCalls)
	}
	if !strings.Contains(response.Body.String(), `"code":"REQUEST_TOO_LARGE"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestProxyRejectsUnknownRoutesAndMethods(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { upstreamCalls++ }))
	defer upstream.Close()
	h := newHandler(handlerConfig{upstreamURL: upstream.URL})

	for _, test := range []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/web/accounts"},
		{http.MethodGet, "/api/web/admin/secrets"},
		{http.MethodDelete, "/api/web/accounts/a/b"},
		{http.MethodGet, "/api/web/../health"},
	} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status = %d", test.method, test.path, response.Code)
		}
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls)
	}
}

func TestCORSAllowsOnlySameOrigin(t *testing.T) {
	h := newHandler(handlerConfig{})

	sameOrigin := httptest.NewRequest(http.MethodGet, "/api/web/health", nil)
	sameOrigin.Host = "ui.example.test"
	sameOrigin.Header.Set("Origin", "http://ui.example.test")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, sameOrigin)
	if response.Code != http.StatusOK || response.Header().Get("Access-Control-Allow-Origin") != "http://ui.example.test" {
		t.Fatalf("same-origin status = %d, allow-origin = %q", response.Code, response.Header().Get("Access-Control-Allow-Origin"))
	}

	crossOrigin := httptest.NewRequest(http.MethodGet, "/api/web/health", nil)
	crossOrigin.Host = "ui.example.test"
	crossOrigin.Header.Set("Origin", "https://evil.example.test")
	response = httptest.NewRecorder()
	h.ServeHTTP(response, crossOrigin)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.Code)
	}
}

func TestCORSRejectsSameHostWithDifferentScheme(t *testing.T) {
	h := newHandler(handlerConfig{})
	request := httptest.NewRequest(http.MethodGet, "/api/web/health", nil)
	request.Host = "ui.example.test"
	request.Header.Set("Origin", "https://ui.example.test")
	request.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCORSUsesConfiguredExternalSchemeBehindProxy(t *testing.T) {
	h := newHandler(handlerConfig{externalScheme: "https"})
	request := httptest.NewRequest(http.MethodGet, "/api/web/health", nil)
	request.Host = "ui.example.test"
	request.Header.Set("Origin", "https://ui.example.test")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
