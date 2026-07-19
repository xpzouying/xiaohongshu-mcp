package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func securedTestHandler(config handlerConfig) http.Handler {
	config.authMode = authModeEnforce
	config.username = "lan-user"
	config.password = "lan-password"
	config.upstreamToken = "backend-secret"
	return newHandler(config)
}

func TestWebUIBasicProtectsPagesStaticAndProxyButNotHealth(t *testing.T) {
	h := securedTestHandler(handlerConfig{})
	for _, path := range []string{"/", "/static/app.css", "/api/web/accounts"} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusUnauthorized || !strings.HasPrefix(response.Header().Get("WWW-Authenticate"), "Basic ") {
			t.Fatalf("GET %s: status=%d challenge=%q", path, response.Code, response.Header().Get("WWW-Authenticate"))
		}
	}
	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/web/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status=%d", health.Code)
	}
	wrong := httptest.NewRequest(http.MethodGet, "/", nil)
	wrong.SetBasicAuth("lan-user", "wrong-password")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, wrong)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d", response.Code)
	}
	page := httptest.NewRequest(http.MethodGet, "/", nil)
	page.SetBasicAuth("lan-user", "lan-password")
	response = httptest.NewRecorder()
	h.ServeHTTP(response, page)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated page status=%d", response.Code)
	}
}

func TestWebUIReplacesClientAuthorizationWithBackendBearer(t *testing.T) {
	gotAuthorization := ""
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	h := securedTestHandler(handlerConfig{upstreamURL: upstream.URL})
	request := httptest.NewRequest(http.MethodGet, "/api/web/accounts", nil)
	request.SetBasicAuth("lan-user", "lan-password")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || gotAuthorization != "Bearer backend-secret" {
		t.Fatalf("status=%d upstream Authorization=%q", response.Code, gotAuthorization)
	}
}

func TestWebUIRemovesCredentialHeadersFromUpstreamResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Authorization", "Bearer backend-secret")
		w.Header().Set("Proxy-Authenticate", "Basic secret")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	h := securedTestHandler(handlerConfig{upstreamURL: upstream.URL})
	request := httptest.NewRequest(http.MethodGet, "/api/web/accounts", nil)
	request.SetBasicAuth("lan-user", "lan-password")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Authorization") != "" || response.Header().Get("Proxy-Authenticate") != "" {
		t.Fatalf("status=%d Authorization=%q Proxy-Authenticate=%q", response.Code, response.Header().Get("Authorization"), response.Header().Get("Proxy-Authenticate"))
	}
}

func TestWebUISecuredProxySupportsAll17Contracts(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if got := r.Header.Get("Authorization"); got != "Bearer backend-secret" {
			t.Errorf("upstream Authorization=%q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	h := securedTestHandler(handlerConfig{upstreamURL: upstream.URL})
	for _, contract := range mcpWebCoverage {
		t.Run(contract.tool, func(t *testing.T) {
			request := httptest.NewRequest(contract.method, contract.webPath, strings.NewReader(`{}`))
			request.SetBasicAuth("lan-user", "lan-password")
			if contract.method != http.MethodGet && contract.method != http.MethodHead {
				request.Header.Set("Origin", "http://example.com")
				request.Header.Set("Sec-Fetch-Site", "same-origin")
			}
			response := httptest.NewRecorder()
			h.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if upstreamCalls != len(mcpWebCoverage) {
		t.Fatalf("upstream calls=%d, want %d", upstreamCalls, len(mcpWebCoverage))
	}
}

func TestWebUICSRFRequiresSameOriginOrFetchMetadataForProxyPOST(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { upstreamCalls++; w.WriteHeader(http.StatusNoContent) }))
	defer upstream.Close()
	h := securedTestHandler(handlerConfig{upstreamURL: upstream.URL})
	tests := []struct {
		name, origin, fetchSite string
		want                    int
	}{
		{"same origin", "http://ui.example.test", "same-origin", http.StatusNoContent},
		{"cross origin", "https://evil.example.test", "cross-site", http.StatusForbidden},
		{"conflicting metadata", "https://evil.example.test", "same-origin", http.StatusForbidden},
		{"same site is not same origin", "", "same-site", http.StatusForbidden},
		{"no origin", "", "", http.StatusForbidden},
		{"metadata without origin", "", "same-origin", http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/web/feeds/search", strings.NewReader(`{"keyword":"test"}`))
			request.Host = "ui.example.test"
			request.SetBasicAuth("lan-user", "lan-password")
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			response := httptest.NewRecorder()
			h.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstream calls=%d", upstreamCalls)
	}
}

func TestWebUIFailsClosedWhenSecretsAreMissing(t *testing.T) {
	for _, mode := range []authMode{authModeOff, authModeWarn, authModeEnforce} {
		for _, config := range []handlerConfig{{authMode: mode, username: "lan-user", password: "lan-password"}, {authMode: mode, upstreamToken: "backend-secret"}} {
			response := httptest.NewRecorder()
			newHandler(config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("mode=%s status=%d, want 503", mode, response.Code)
			}
		}
	}
}

func TestLoadHandlerConfigAllowsCurrentScopedTokensWithEmptyLegacyEnvironment(t *testing.T) {
	dir := t.TempDir()
	secrets := map[string]string{
		"WEBUI_PASSWORD_FILE":  "webui-password",
		"XHS_READ_TOKEN_FILE":  "read-token",
		"XHS_WRITE_TOKEN_FILE": "write-token",
		"XHS_ADMIN_TOKEN_FILE": "admin-token",
	}
	for name, value := range secrets {
		path := dir + "/" + strings.ToLower(name)
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
	t.Setenv("XHS_API_TOKEN_FILE", "")
	t.Setenv("WEBUI_USERNAME", "lan-user")
	t.Setenv("WEBUI_AUTH_MODE", "enforce")

	config := loadHandlerConfig()
	if config.securityError || config.upstreamToken != "" || config.readToken != "read-token" || config.writeToken != "write-token" || config.adminToken != "admin-token" {
		t.Fatalf("security-error=%t legacy-loaded=%t scoped=%q/%q/%q", config.securityError, config.upstreamToken != "", config.readToken, config.writeToken, config.adminToken)
	}
}

func TestReadSecretFileRequiresPrivateRegularFile(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		name    string
		mode    os.FileMode
		wantErr bool
	}{
		{name: "0400", mode: 0o400},
		{name: "0600", mode: 0o600},
		{name: "0644", mode: 0o644, wantErr: true},
		{name: "0444", mode: 0o444, wantErr: true},
		{name: "0660", mode: 0o660, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := dir + "/secret-" + test.name
			if err := os.WriteFile(path, []byte("secret-value"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, test.mode); err != nil {
				t.Fatal(err)
			}
			got, err := readSecretFile(path)
			if (err != nil) != test.wantErr {
				t.Fatalf("readSecretFile() error=%v, wantErr=%t", err, test.wantErr)
			}
			if !test.wantErr && got != "secret-value" {
				t.Fatalf("readSecretFile()=%q", got)
			}
		})
	}

	if _, err := readSecretFile(dir); err == nil {
		t.Fatal("directory should fail closed")
	}
	target := dir + "/target"
	if err := os.WriteFile(target, []byte("secret-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := dir + "/secret-link"
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecretFile(link); err == nil {
		t.Fatal("symlink should fail closed")
	}
}

func TestReadSecretFileRejectsPathReplacedBySymlinkBeforeOpen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/secret"
	target := dir + "/target"
	if err := os.WriteFile(path, []byte("original-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("replacement-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := readSecretFileFrom(path, func(name string) (*os.File, error) {
		if err := os.Remove(name); err != nil {
			return nil, err
		}
		if err := os.Symlink(target, name); err != nil {
			return nil, err
		}
		return openSecretFileNoFollow(name)
	})
	if err == nil {
		t.Fatal("path replacement with symlink should fail closed")
	}
}

func TestLoadHandlerConfigRejectsNonEmptyInvalidLegacyPathWithCurrentTokens(t *testing.T) {
	dir := t.TempDir()
	for name, value := range map[string]string{
		"WEBUI_PASSWORD_FILE":  "webui-password",
		"XHS_READ_TOKEN_FILE":  "read-token",
		"XHS_WRITE_TOKEN_FILE": "write-token",
		"XHS_ADMIN_TOKEN_FILE": "admin-token",
	} {
		path := dir + "/" + strings.ToLower(name)
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
	t.Setenv("XHS_API_TOKEN_FILE", dir+"/missing-legacy-token")
	t.Setenv("WEBUI_USERNAME", "lan-user")

	config := loadHandlerConfig()
	if !config.securityError {
		t.Fatal("non-empty invalid legacy path was not retained as a configuration error")
	}
	response := httptest.NewRecorder()
	newHandler(config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", response.Code)
	}
}

func TestLoadHandlerConfigRejectsNonEmptyInvalidCurrentScopedPaths(t *testing.T) {
	for _, badPath := range []struct {
		name string
		path func(string) string
	}{
		{"missing", func(dir string) string { return dir + "/missing-token" }},
		{"unreadable", func(dir string) string { return dir }},
	} {
		for _, badEnv := range []string{"XHS_READ_TOKEN_FILE", "XHS_WRITE_TOKEN_FILE", "XHS_ADMIN_TOKEN_FILE"} {
			t.Run(badEnv+"/"+badPath.name, func(t *testing.T) {
				dir := t.TempDir()
				for name, value := range map[string]string{
					"WEBUI_PASSWORD_FILE":  "webui-password",
					"XHS_API_TOKEN_FILE":   "legacy-token",
					"XHS_READ_TOKEN_FILE":  "read-token",
					"XHS_WRITE_TOKEN_FILE": "write-token",
					"XHS_ADMIN_TOKEN_FILE": "admin-token",
				} {
					path := dir + "/" + strings.ToLower(name)
					if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
						t.Fatal(err)
					}
					t.Setenv(name, path)
				}
				t.Setenv(badEnv, badPath.path(dir))
				t.Setenv("WEBUI_USERNAME", "lan-user")
				t.Setenv("WEBUI_AUTH_MODE", "enforce")

				config := loadHandlerConfig()
				if !config.securityError {
					t.Fatal("non-empty invalid scoped path was not retained as a configuration error")
				}
				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.SetBasicAuth("lan-user", "webui-password")
				response := httptest.NewRecorder()
				newHandler(config).ServeHTTP(response, request)
				if response.Code != http.StatusServiceUnavailable {
					t.Fatalf("status=%d, want 503", response.Code)
				}
				body := response.Body.String()
				if strings.Contains(body, dir) || strings.Contains(body, "legacy-token") || strings.Contains(body, "read-token") || strings.Contains(body, "write-token") || strings.Contains(body, "admin-token") {
					t.Fatalf("security response leaked secret material: %s", body)
				}
			})
		}
	}
}

func TestLoadHandlerConfigTreatsBlankCurrentScopedPathsAsAbsentWithLegacyToken(t *testing.T) {
	for _, blank := range []string{"", " \t\n "} {
		t.Run(strings.ReplaceAll(blank, " ", "space"), func(t *testing.T) {
			dir := t.TempDir()
			for name, value := range map[string]string{
				"WEBUI_PASSWORD_FILE": "webui-password",
				"XHS_API_TOKEN_FILE":  "legacy-token",
			} {
				path := dir + "/" + strings.ToLower(name)
				if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv(name, path)
			}
			for _, name := range []string{"XHS_READ_TOKEN_FILE", "XHS_WRITE_TOKEN_FILE", "XHS_ADMIN_TOKEN_FILE"} {
				t.Setenv(name, blank)
			}
			t.Setenv("WEBUI_USERNAME", "lan-user")
			t.Setenv("WEBUI_AUTH_MODE", "enforce")

			config := loadHandlerConfig()
			if config.securityError || config.upstreamToken != "legacy-token" {
				t.Fatalf("security-error=%t legacy-token=%q", config.securityError, config.upstreamToken)
			}
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.SetBasicAuth("lan-user", "webui-password")
			response := httptest.NewRecorder()
			newHandler(config).ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200", response.Code)
			}
		})
	}
}

func TestWebUIOnlySafeHealthRequestIsExempt(t *testing.T) {
	h := securedTestHandler(handlerConfig{})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/web/health", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("POST health status=%d, want 401", response.Code)
	}
}

func TestWebUICSRFRejectsMalformedOrigin(t *testing.T) {
	h := securedTestHandler(handlerConfig{})
	for _, origin := range []string{"http://ui.example.test/path", "http://user@ui.example.test"} {
		request := httptest.NewRequest(http.MethodPost, "/api/web/feeds/search", strings.NewReader(`{"keyword":"test"}`))
		request.Host = "ui.example.test"
		request.SetBasicAuth("lan-user", "lan-password")
		request.Header.Set("Origin", origin)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("origin %q: status=%d, want 403", origin, response.Code)
		}
	}
}
