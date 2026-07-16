package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestEmbeddedStaticFiles(t *testing.T) {
	h := newHandler(handlerConfig{})
	for _, path := range []string{
		"/", "/accounts.html", "/search.html", "/publish.html", "/detail.html",
		"/static/app.css", "/static/app.js", "/static/accounts.js",
	} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, body = %s", path, response.Code, response.Body.String())
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
