package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebUIUsesLeastPrivilegeTokenPerRoute(t *testing.T) {
	got := map[string]string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got[r.Method+" "+r.URL.Path] = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	h := securedTestHandler(handlerConfig{
		upstreamURL: upstream.URL, upstreamToken: "",
		readToken: "read-token", writeToken: "write-token", adminToken: "admin-token",
	})
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/web/accounts"},
		{http.MethodPost, "/api/web/feeds/search"},
		{http.MethodPost, "/api/web/feeds/comment"},
		{http.MethodPost, "/api/web/accounts"},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
		request.SetBasicAuth("lan-user", "lan-password")
		if test.method == http.MethodPost {
			request.Header.Set("Origin", "http://example.com")
			request.Header.Set("Sec-Fetch-Site", "same-origin")
		}
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
	want := map[string]string{
		"GET /api/v1/accounts":       "Bearer read-token",
		"POST /api/v1/feeds/search":  "Bearer read-token",
		"POST /api/v1/feeds/comment": "Bearer write-token",
		"POST /api/v1/accounts":      "Bearer admin-token",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s authorization=%q want=%q", key, got[key], value)
		}
	}
}

func TestWebUIProxyStripsClientIdentityHeaders(t *testing.T) {
	got := make(http.Header)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	h := securedTestHandler(handlerConfig{upstreamURL: upstream.URL, readToken: "read-token", writeToken: "write-token", adminToken: "admin-token"})
	request := httptest.NewRequest(http.MethodGet, "/api/web/accounts", nil)
	request.SetBasicAuth("lan-user", "lan-password")
	request.Header.Set("Proxy-Authorization", "proxy-secret")
	request.Header.Set("X-XHS-Authenticated-Actor", "forged-actor")
	request.Header.Set("X-XHS-Authenticated-Scopes", "admin")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || got.Get("Authorization") != "Bearer read-token" {
		t.Fatalf("status=%d authorization=%q", response.Code, got.Get("Authorization"))
	}
	for _, name := range []string{"Proxy-Authorization", "X-XHS-Authenticated-Actor", "X-XHS-Authenticated-Scopes"} {
		if got.Get(name) != "" {
			t.Fatalf("upstream received %s=%q", name, got.Get(name))
		}
	}
}
