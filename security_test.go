package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func TestProtectedBackendRoutesRequireBearerToken(t *testing.T) {
	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret"})
	for _, path := range []string{"/api/v1", "/api/v1/not-found", "/mcp", "/mcp/session"} {
		for _, test := range []struct {
			name  string
			token string
			want  int
		}{{"missing", "", http.StatusUnauthorized}, {"wrong", "wrong", http.StatusUnauthorized}, {"correct", "backend-secret", 0}} {
			t.Run(path+" "+test.name, func(t *testing.T) {
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if test.token != "" {
					request.Header.Set("Authorization", "Bearer "+test.token)
				}
				router.ServeHTTP(response, request)
				if test.want != 0 && response.Code != test.want {
					t.Fatalf("status = %d, want %d", response.Code, test.want)
				}
				if test.want == 0 && response.Code == http.StatusUnauthorized {
					t.Fatalf("correct token was rejected: %s", response.Body.String())
				}
			})
		}
	}
}

func TestBackendBearerAuthorizationParsing(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   int
	}{
		{name: "canonical scheme", values: []string{"Bearer backend-secret"}, want: http.StatusNotFound},
		{name: "lowercase scheme", values: []string{"bearer backend-secret"}, want: http.StatusNotFound},
		{name: "uppercase scheme", values: []string{"BEARER backend-secret"}, want: http.StatusNotFound},
		{name: "mixed case scheme", values: []string{"BeArEr backend-secret"}, want: http.StatusNotFound},
		{name: "token remains case sensitive", values: []string{"Bearer BACKEND-SECRET"}, want: http.StatusUnauthorized},
		{name: "duplicate correct then wrong", values: []string{"Bearer backend-secret", "Bearer wrong"}, want: http.StatusUnauthorized},
		{name: "duplicate wrong then correct", values: []string{"Bearer wrong", "Bearer backend-secret"}, want: http.StatusUnauthorized},
		{name: "duplicate correct", values: []string{"Bearer backend-secret", "Bearer backend-secret"}, want: http.StatusUnauthorized},
		{name: "comma joined", values: []string{"Bearer backend-secret, Bearer backend-secret"}, want: http.StatusUnauthorized},
		{name: "multiple spaces", values: []string{"Bearer  backend-secret"}, want: http.StatusUnauthorized},
		{name: "tab separator", values: []string{"Bearer	backend-secret"}, want: http.StatusUnauthorized},
		{name: "trailing whitespace", values: []string{"Bearer backend-secret "}, want: http.StatusUnauthorized},
	}

	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret"})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
			for _, value := range test.values {
				request.Header.Add("Authorization", value)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestBackendHealthDoesNotRequireTokenOrExposeDetails(t *testing.T) {
	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret"})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "exact GET", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK, wantBody: "{\"status\":\"ok\"}"},
		{name: "exact HEAD", method: http.MethodHead, path: "/health", wantStatus: http.StatusOK},
		{name: "POST rejected", method: http.MethodPost, path: "/health", wantStatus: http.StatusNotFound},
		{name: "trailing slash GET", method: http.MethodGet, path: "/health/", wantStatus: http.StatusNotFound},
		{name: "trailing slash HEAD", method: http.MethodHead, path: "/health/", wantStatus: http.StatusNotFound},
		{name: "case variant GET", method: http.MethodGet, path: "/Health", wantStatus: http.StatusNotFound},
		{name: "case variant HEAD", method: http.MethodHead, path: "/HEALTH", wantStatus: http.StatusNotFound},
		{name: "double slash", method: http.MethodGet, path: "//health", wantStatus: http.StatusNotFound},
		{name: "encoded slash", method: http.MethodGet, path: "/health%2F", wantStatus: http.StatusNotFound},
		{name: "double trailing slash", method: http.MethodGet, path: "/health//", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != test.wantStatus {
				t.Fatalf("%s %s: status=%d, want %d", test.method, test.path, response.Code, test.wantStatus)
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Fatalf("%s %s: body=%q, want %q", test.method, test.path, response.Body.String(), test.wantBody)
			}
			if test.name == "exact HEAD" && response.Body.Len() != 0 {
				t.Fatalf("HEAD body=%q, want empty", response.Body.String())
			}
			if location := response.Header().Get("Location"); location != "" {
				t.Fatalf("%s %s exposed redirect Location %q", test.method, test.path, location)
			}
		})
	}
}

func TestBackendHealthGETAndHEADShareRepresentationHeadersOnHTTPServer(t *testing.T) {
	server := httptest.NewServer(setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret"}))
	t.Cleanup(server.Close)

	responses := make(map[string]*http.Response)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request, err := http.NewRequest(method, server.URL+"/health", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		responses[method] = response
		t.Cleanup(func() { response.Body.Close() })
	}

	getBody, err := io.ReadAll(responses[http.MethodGet].Body)
	if err != nil {
		t.Fatal(err)
	}
	headBody, err := io.ReadAll(responses[http.MethodHead].Body)
	if err != nil {
		t.Fatal(err)
	}
	getResponse, headResponse := responses[http.MethodGet], responses[http.MethodHead]
	if getResponse.StatusCode != http.StatusOK || headResponse.StatusCode != getResponse.StatusCode {
		t.Fatalf("GET status=%d HEAD status=%d", getResponse.StatusCode, headResponse.StatusCode)
	}
	if string(getBody) != `{"status":"ok"}` || len(headBody) != 0 {
		t.Fatalf("GET body=%q HEAD body=%q", getBody, headBody)
	}
	if headResponse.Header.Get("Content-Type") != getResponse.Header.Get("Content-Type") || headResponse.ContentLength != getResponse.ContentLength {
		t.Fatalf("GET content-type=%q length=%d; HEAD content-type=%q length=%d", getResponse.Header.Get("Content-Type"), getResponse.ContentLength, headResponse.Header.Get("Content-Type"), headResponse.ContentLength)
	}
}

func TestBackendCORSExactAllowlistOnly(t *testing.T) {
	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret", allowedOrigins: map[string]struct{}{"https://ui.example.test": {}}})
	for _, test := range []struct {
		origin     string
		wantOrigin string
	}{{"https://ui.example.test", "https://ui.example.test"}, {"https://evil.example.test", ""}} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Authorization", "Bearer backend-secret")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin || response.Code != http.StatusNotFound {
			t.Fatalf("origin %q: status=%d allow-origin=%q; want status=%d origin=%q", test.origin, response.Code, got, http.StatusNotFound, test.wantOrigin)
		}
	}
}

func TestBackendCORSDisabledByDefault(t *testing.T) {
	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
	request.Header.Set("Origin", "https://ui.example.test")
	request.Header.Set("Authorization", "Bearer backend-secret")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("status=%d allow-origin=%q", response.Code, response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBackendAllowlistedBrowserPreflightDoesNotRequireBearerToken(t *testing.T) {
	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret", allowedOrigins: map[string]struct{}{"https://ui.example.test": {}}})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	request, err := http.NewRequest(http.MethodOptions, server.URL+"/api/v1/accounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "https://ui.example.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	request.Header.Set("Sec-Fetch-Mode", "cors")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != "https://ui.example.test" {
		t.Fatalf("status=%d allow-origin=%q", response.StatusCode, response.Header.Get("Access-Control-Allow-Origin"))
	}
	if vary := response.Header.Get("Vary"); !strings.Contains(vary, "Origin") || !strings.Contains(vary, "Access-Control-Request-Method") || !strings.Contains(vary, "Access-Control-Request-Headers") {
		t.Fatalf("Vary=%q", vary)
	}
}

func TestBackendPreflightDoesNotBroadenCORSOrAuthentication(t *testing.T) {
	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: "backend-secret", allowedOrigins: map[string]struct{}{"https://ui.example.test": {}}})
	tests := []struct {
		name          string
		origin        string
		requestMethod string
		wantStatus    int
		wantOrigin    string
	}{
		{name: "denied origin", origin: "https://evil.example.test", requestMethod: http.MethodGet, wantStatus: http.StatusUnauthorized},
		{name: "missing origin", requestMethod: http.MethodGet, wantStatus: http.StatusUnauthorized},
		{name: "ordinary OPTIONS", origin: "https://ui.example.test", wantStatus: http.StatusUnauthorized, wantOrigin: "https://ui.example.test"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/api/v1/accounts", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.requestMethod != "" {
				request.Header.Set("Access-Control-Request-Method", test.requestMethod)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus || response.Header().Get("Access-Control-Allow-Origin") != test.wantOrigin {
				t.Fatalf("status=%d allow-origin=%q; want status=%d origin=%q", response.Code, response.Header().Get("Access-Control-Allow-Origin"), test.wantStatus, test.wantOrigin)
			}
			vary := response.Header().Get("Vary")
			for _, header := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
				if !strings.Contains(vary, header) {
					t.Fatalf("Vary=%q, missing %q", vary, header)
				}
			}
		})
	}
}

func TestCORSMiddlewarePreservesExistingVaryValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Header("Vary", "Accept-Encoding")
		c.Next()
	})
	router.Use(corsMiddleware(map[string]struct{}{"https://ui.example.test": {}}))
	router.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("Origin", "https://ui.example.test")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	vary := response.Header().Values("Vary")
	for _, header := range []string{"Accept-Encoding", "Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
		if !strings.Contains(strings.Join(vary, ","), header) {
			t.Fatalf("Vary=%q, missing %q", vary, header)
		}
	}
}

func TestReadSecretFileRejectsMissingOrEmptySecret(t *testing.T) {
	if _, err := readSecretFile(""); err == nil {
		t.Fatal("empty path should fail closed")
	}
	empty := t.TempDir() + "/empty"
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecretFile(empty); err == nil {
		t.Fatal("empty secret should fail closed")
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
			path := dir + "/token-" + test.name
			if err := os.WriteFile(path, []byte("backend-secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, test.mode); err != nil {
				t.Fatal(err)
			}
			got, err := readSecretFile(path)
			if (err != nil) != test.wantErr {
				t.Fatalf("readSecretFile() error=%v, wantErr=%t", err, test.wantErr)
			}
			if !test.wantErr && got != "backend-secret" {
				t.Fatalf("readSecretFile()=%q", got)
			}
		})
	}

	if _, err := readSecretFile(dir); err == nil {
		t.Fatal("directory should fail closed")
	}
	target := dir + "/target"
	if err := os.WriteFile(target, []byte("backend-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := dir + "/token-link"
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecretFile(link); err == nil {
		t.Fatal("symlink should fail closed")
	}
}

func TestReadSecretFileRejectsPathReplacedBySymlinkBeforeOpen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/token"
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

func TestReadSecretFileValidatesRFC6750BearerTokenSyntax(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "valid alphabet", value: "AZaz09-._~+/==\n", want: "AZaz09-._~+/=="},
		{name: "minimal token", value: "a", want: "a"},
		{name: "minimal token with one padding", value: "a=", want: "a="},
		{name: "minimal token with two padding", value: "a==", want: "a=="},
		{name: "padding only", value: "=", wantErr: true},
		{name: "two padding only", value: "==", wantErr: true},
		{name: "three padding only", value: "===", wantErr: true},
		{name: "comma", value: "token,other", wantErr: true},
		{name: "ASCII space", value: "token value", wantErr: true},
		{name: "ASCII tab", value: "token	value", wantErr: true},
		{name: "Unicode whitespace", value: "token\u00a0value", wantErr: true},
		{name: "control", value: "token\x00value", wantErr: true},
		{name: "quote", value: `token"value`, wantErr: true},
		{name: "backslash", value: `token\value`, wantErr: true},
		{name: "padding in middle", value: "token=value", wantErr: true},
		{name: "non b64token symbol", value: "token:value", wantErr: true},
		{name: "leading newline", value: "\ntoken", wantErr: true},
		{name: "multiple trailing newlines", value: "token\n\n", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := t.TempDir() + "/token"
			if err := os.WriteFile(path, []byte(test.value), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := readSecretFile(path)
			if (err != nil) != test.wantErr {
				t.Fatalf("error=%v, wantErr=%t", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("token=%q, want %q", got, test.want)
			}
			if err != nil && strings.Contains(err.Error(), test.value) {
				t.Fatal("error exposed secret")
			}
		})
	}
}

func TestInvalidConfiguredBearerTokenFailsClosedInEveryMode(t *testing.T) {
	for _, mode := range []authMode{authModeOff, authModeWarn, authModeEnforce} {
		t.Run(string(mode), func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
			request.Header.Set("Authorization", "Bearer invalid,token")
			response := httptest.NewRecorder()
			setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: mode, token: "invalid,token"}).ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "invalid,token") {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
		})
	}
}

func TestCommaJoinedAuthorizationFailsClosedInEveryMode(t *testing.T) {
	tests := []struct {
		name   string
		config backendSecurityConfig
	}{
		{name: "off configured", config: backendSecurityConfig{mode: authModeOff, token: "backend-secret"}},
		{name: "warn configured", config: backendSecurityConfig{mode: authModeWarn, token: "backend-secret"}},
		{name: "enforce configured", config: backendSecurityConfig{mode: authModeEnforce, token: "backend-secret"}},
		{name: "off missing", config: backendSecurityConfig{mode: authModeOff}},
		{name: "warn missing", config: backendSecurityConfig{mode: authModeWarn}},
		{name: "enforce missing", config: backendSecurityConfig{mode: authModeEnforce}},
		{name: "explicit insecure test mode", config: backendSecurityConfig{mode: authModeOff, allowInsecure: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
			request.Header.Set("Authorization", "Bearer backend-secret, Bearer backend-secret")
			response := httptest.NewRecorder()
			setupRoutesWithSecurity(&AppServer{}, test.config).ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d, want 401", response.Code)
			}
		})
	}
}

func TestLoadBackendSecurityConfigReadsTokenFileAndDefaultsToEnforce(t *testing.T) {
	secretFile := t.TempDir() + "/token"
	if err := os.WriteFile(secretFile, []byte("file-secret\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XHS_AUTH_MODE", "")
	t.Setenv("XHS_API_TOKEN_FILE", secretFile)
	t.Setenv("XHS_ALLOW_INSECURE_TEST_MODE", "")

	config := loadBackendSecurityConfig()
	if config.mode != authModeEnforce || config.token != "file-secret" || config.allowInsecure {
		t.Fatalf("mode=%q token-loaded=%t allow-insecure=%t", config.mode, config.token != "", config.allowInsecure)
	}
}

func TestLoadBackendSecurityConfigAllowsCurrentScopedTokensWithEmptyLegacyEnvironment(t *testing.T) {
	dir := t.TempDir()
	for _, item := range []struct {
		name, token string
	}{
		{"XHS_READ_TOKEN_FILE", "read-token"},
		{"XHS_WRITE_TOKEN_FILE", "write-token"},
		{"XHS_ADMIN_TOKEN_FILE", "admin-token"},
	} {
		path := dir + "/" + strings.ToLower(item.name)
		if err := os.WriteFile(path, []byte(item.token), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(item.name, path)
	}
	for _, name := range []string{
		"XHS_API_TOKEN_FILE",
		"XHS_READ_TOKEN_PREVIOUS_FILE",
		"XHS_WRITE_TOKEN_PREVIOUS_FILE",
		"XHS_ADMIN_TOKEN_PREVIOUS_FILE",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("XHS_AUTH_MODE", "enforce")

	config := loadBackendSecurityConfig()
	if config.tokenFileError || len(config.credentials) != 3 {
		t.Fatalf("token-file-error=%t credentials=%d", config.tokenFileError, len(config.credentials))
	}
	router := setupRoutesWithSecurity(&AppServer{}, config)
	for _, test := range []struct {
		method, path, token string
	}{
		{http.MethodGet, "/api/v1/accounts", "read-token"},
		{http.MethodPost, "/api/v1/publish", "write-token"},
		{http.MethodPost, "/api/v1/accounts", "admin-token"},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer "+test.token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code == http.StatusUnauthorized || response.Code == http.StatusForbidden {
			t.Fatalf("current token %q was rejected with status %d", test.token, response.Code)
		}
	}
}

func TestLoadBackendSecurityConfigRejectsNonEmptyInvalidLegacyPathWithCurrentTokens(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"XHS_READ_TOKEN_FILE", "XHS_WRITE_TOKEN_FILE", "XHS_ADMIN_TOKEN_FILE"} {
		path := dir + "/" + strings.ToLower(name)
		if err := os.WriteFile(path, []byte(name+"-token"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
	t.Setenv("XHS_API_TOKEN_FILE", dir+"/missing-legacy-token")
	t.Setenv("XHS_AUTH_MODE", "enforce")

	config := loadBackendSecurityConfig()
	if !config.tokenFileError || len(config.credentials) != 3 {
		t.Fatalf("token-file-error=%t credentials=%d", config.tokenFileError, len(config.credentials))
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
	request.Header.Set("Authorization", "Bearer XHS_READ_TOKEN_FILE-token")
	response := httptest.NewRecorder()
	setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", response.Code)
	}
}

func TestLoadBackendSecurityConfigInsecureSwitchIsExplicit(t *testing.T) {
	t.Setenv("XHS_AUTH_MODE", "off")
	unsetEnvForTest(t, "XHS_API_TOKEN_FILE")
	t.Setenv("XHS_ALLOW_INSECURE_TEST_MODE", "true")

	config := loadBackendSecurityConfig()
	if config.mode != authModeOff || !config.allowInsecure {
		t.Fatalf("mode=%q allow-insecure=%t", config.mode, config.allowInsecure)
	}
}

func TestBackendTokenFileConfigurationMatrix(t *testing.T) {
	validPath := t.TempDir() + "/token"
	if err := os.WriteFile(validPath, []byte("backend-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalidPath := t.TempDir() + "/missing-token"
	paths := []struct {
		name  string
		value string
		set   bool
		valid bool
	}{
		{name: "unset"},
		{name: "empty", set: true},
		{name: "spaces", value: "   ", set: true},
		{name: "tab", value: "	", set: true},
		{name: "CR LF", value: "\r\n", set: true},
		{name: "Unicode whitespace", value: "\u00a0\u2003", set: true},
		{name: "control character", value: "\x01", set: true},
		{name: "invalid path", value: invalidPath, set: true},
		{name: "valid path", value: validPath, set: true, valid: true},
	}

	for _, path := range paths {
		for _, mode := range []authMode{authModeOff, authModeWarn, authModeEnforce} {
			for _, allowInsecure := range []bool{false, true} {
				allowValue := map[bool]string{false: "false", true: "true"}[allowInsecure]
				t.Run(path.name+"/"+string(mode)+"/allow-insecure="+allowValue, func(t *testing.T) {
					t.Setenv("XHS_AUTH_MODE", string(mode))
					t.Setenv("XHS_ALLOW_INSECURE_TEST_MODE", allowValue)
					if path.set {
						t.Setenv("XHS_API_TOKEN_FILE", path.value)
					} else {
						unsetEnvForTest(t, "XHS_API_TOKEN_FILE")
					}

					config := loadBackendSecurityConfig()
					response := httptest.NewRecorder()
					setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))

					emptyPath := strings.TrimSpace(path.value) == ""
					wantBypass := (path.valid && mode != authModeEnforce) || (mode == authModeOff && (!path.set || emptyPath) && allowInsecure)
					if wantBypass && response.Code == http.StatusUnauthorized {
						t.Fatalf("status=%d, expected explicit off-mode bypass", response.Code)
					}
					if !wantBypass && response.Code != http.StatusUnauthorized {
						t.Fatalf("status=%d, want 401", response.Code)
					}
					if path.set && !emptyPath && !path.valid && !config.tokenFileError {
						t.Fatal("explicit invalid token path was not retained as a configuration error")
					}
					if (!path.set || emptyPath) && config.tokenFileError {
						t.Fatal("unset or empty token path was misclassified as an explicit configuration error")
					}
				})
			}
		}
	}
}

func TestBackendNULTokenFilePathFailsClosedWithoutDisclosure(t *testing.T) {
	const invalidPath = "secret-path\x00must-not-appear"
	if _, err := readSecretFile(invalidPath); err == nil || strings.Contains(err.Error(), invalidPath) {
		t.Fatalf("error=%q", err)
	}

	for _, mode := range []authMode{authModeOff, authModeWarn, authModeEnforce} {
		for _, allowInsecure := range []bool{false, true} {
			response := httptest.NewRecorder()
			config := backendSecurityConfig{mode: mode, allowInsecure: allowInsecure, tokenFileError: true}
			setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))
			if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), invalidPath) {
				t.Fatalf("mode=%s allow-insecure=%t status=%d body=%q", mode, allowInsecure, response.Code, response.Body.String())
			}
		}
	}
}

func TestBackendInvalidTokenFilePathNotDisclosedInLogs(t *testing.T) {
	const invalidPath = "missing-token-path-must-not-appear"
	var logs bytes.Buffer
	previousOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(previousOutput) })
	t.Setenv("XHS_AUTH_MODE", "warn")
	t.Setenv("XHS_API_TOKEN_FILE", invalidPath)
	t.Setenv("XHS_ALLOW_INSECURE_TEST_MODE", "true")

	config := loadBackendSecurityConfig()
	response := httptest.NewRecorder()
	setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))
	if response.Code != http.StatusUnauthorized || strings.Contains(logs.String(), invalidPath) || strings.Contains(response.Body.String(), invalidPath) {
		t.Fatalf("status=%d log-disclosed=%t body-disclosed=%t", response.Code, strings.Contains(logs.String(), invalidPath), strings.Contains(response.Body.String(), invalidPath))
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	value, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var err error
		if existed {
			err = os.Setenv(key, value)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Errorf("restore %s: %v", key, err)
		}
	})
}

func TestBackendMissingSecretFailsClosedUnlessTestSwitchEnabled(t *testing.T) {
	for _, config := range []backendSecurityConfig{
		{mode: authModeEnforce},
		{mode: authModeWarn},
		{mode: authModeOff},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
		setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("mode %q without secret: status=%d, want 401", config.mode, response.Code)
		}
	}

	response := httptest.NewRecorder()
	setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeOff, allowInsecure: true}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))
	if response.Code == http.StatusUnauthorized {
		t.Fatal("explicit test-only insecure switch should bypass authentication")
	}
}

func TestBackendExplicitTokenFileFailureAlwaysFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		content *string
	}{
		{name: "missing file"},
		{name: "unreadable path", content: stringPointer("directory")},
		{name: "empty file", content: stringPointer("\n")},
		{name: "invalid file", content: stringPointer("invalid,token")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := t.TempDir() + "/token"
			if test.content != nil {
				var err error
				if *test.content == "directory" {
					err = os.Mkdir(path, 0o700)
				} else {
					err = os.WriteFile(path, []byte(*test.content), 0o600)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, mode := range []authMode{authModeOff, authModeWarn, authModeEnforce} {
				t.Run(string(mode), func(t *testing.T) {
					t.Setenv("XHS_AUTH_MODE", string(mode))
					t.Setenv("XHS_API_TOKEN_FILE", path)
					t.Setenv("XHS_ALLOW_INSECURE_TEST_MODE", "true")
					config := loadBackendSecurityConfig()
					response := httptest.NewRecorder()
					setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))
					if response.Code != http.StatusUnauthorized {
						t.Fatalf("status=%d, want 401", response.Code)
					}
				})
			}
		})
	}
}

func stringPointer(value string) *string { return &value }

func TestBackendOffModeRequiresConfiguredSecretOrTestSwitch(t *testing.T) {
	for _, config := range []backendSecurityConfig{
		{mode: authModeOff, token: "configured-secret"},
		{mode: authModeOff, allowInsecure: true},
	} {
		response := httptest.NewRecorder()
		setupRoutesWithSecurity(&AppServer{}, config).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))
		if response.Code == http.StatusUnauthorized {
			t.Fatalf("explicit off mode was rejected for config %+v", config)
		}
	}
}

func TestBackendWarnLogDoesNotExposeCredential(t *testing.T) {
	const credential = "credential-must-not-appear"
	var logs bytes.Buffer
	previousOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(previousOutput) })

	request := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
	request.Header.Set("Authorization", "Bearer "+credential)
	response := httptest.NewRecorder()
	setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeWarn, token: "expected-secret"}).ServeHTTP(response, request)
	if strings.Contains(logs.String(), credential) || strings.Contains(response.Body.String(), credential) {
		t.Fatal("credential leaked through logs or response")
	}
}

func TestBackendUnauthorizedResponseDoesNotExposeSecret(t *testing.T) {
	const secret = "expected-secret-must-not-appear"
	response := httptest.NewRecorder()
	setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, token: secret}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil))
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusUnauthorized || strings.Contains(string(body), secret) {
		t.Fatalf("status=%d body=%q", response.Code, body)
	}
}

func TestParseOriginAllowlistRejectsWildcardAndMalformedOrigins(t *testing.T) {
	allowed := parseOriginAllowlist("*, https://*.example.test, https://ui.example.test, https://ui.example.test/path, ftp://ui.example.test")
	if len(allowed) != 1 {
		t.Fatalf("allowlist=%v", allowed)
	}
	if _, ok := allowed["https://ui.example.test"]; !ok {
		t.Fatal("exact HTTPS origin was rejected")
	}
}
