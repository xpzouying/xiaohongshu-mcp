package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
)

func scopedTestConfig() backendSecurityConfig {
	return backendSecurityConfig{mode: authModeEnforce, credentials: []scopedCredential{
		newScopedCredential("read-token", scopeRead),
		newScopedCredential("write-token", scopeWrite),
		newScopedCredential("admin-token", scopeAdmin),
		newScopedCredential("old-read-token", scopeRead),
	}}
}

func TestRESTScopeMatrix(t *testing.T) {
	tests := []struct {
		name, method, path, token string
		want                      int
	}{
		{"missing authentication", http.MethodGet, "/api/v1/accounts", "", http.StatusUnauthorized},
		{"read allows read", http.MethodGet, "/api/v1/accounts", "read-token", http.StatusServiceUnavailable},
		{"old read overlap allows read", http.MethodGet, "/api/v1/accounts", "old-read-token", http.StatusServiceUnavailable},
		{"write denied read", http.MethodGet, "/api/v1/accounts", "write-token", http.StatusForbidden},
		{"admin denied read", http.MethodGet, "/api/v1/accounts", "admin-token", http.StatusForbidden},
		{"read denied write", http.MethodPost, "/api/v1/publish", "read-token", http.StatusForbidden},
		{"admin denied write", http.MethodPost, "/api/v1/publish", "admin-token", http.StatusForbidden},
		{"write reaches write handler", http.MethodPost, "/api/v1/publish", "write-token", http.StatusInternalServerError},
		{"read denied admin", http.MethodPost, "/api/v1/accounts", "read-token", http.StatusForbidden},
		{"write denied admin", http.MethodPost, "/api/v1/accounts", "write-token", http.StatusForbidden},
		{"admin reaches admin handler", http.MethodPost, "/api/v1/accounts", "admin-token", http.StatusServiceUnavailable},
	}
	router := setupRoutesWithSecurity(&AppServer{}, scopedTestConfig())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s, want=%d", response.Code, response.Body.String(), test.want)
			}
		})
	}
}

func TestAuthenticationFailuresAreAudited(t *testing.T) {
	tests := []struct {
		name   string
		config backendSecurityConfig
		token  string
	}{
		{"missing token", scopedTestConfig(), ""},
		{"invalid token", scopedTestConfig(), "invalid-never-log"},
		{"credential configuration failure", backendSecurityConfig{mode: authModeEnforce, tokenFileError: true}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := logrus.StandardLogger().Out
			logrus.SetOutput(&logs)
			defer logrus.SetOutput(previous)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			setupRoutesWithSecurity(&AppServer{}, test.config).ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d", response.Code)
			}
			output := logs.String()
			for _, required := range []string{"event=security_audit", "operation=authentication", "outcome=failure"} {
				if !strings.Contains(output, required) {
					t.Fatalf("missing %q in log=%s", required, output)
				}
			}
			if count := strings.Count(output, "event=security_audit"); count != 1 {
				t.Fatalf("authentication audit count=%d log=%s", count, output)
			}
			if test.token != "" && strings.Contains(output, test.token) {
				t.Fatalf("authentication log leaked token: %s", output)
			}
		})
	}
}

func TestMCPToolAuthorizationMatrix(t *testing.T) {
	for _, test := range []struct {
		name      string
		toolScope accessScope
		principal requestPrincipal
		allowed   bool
	}{
		{"read allows read", scopeRead, requestPrincipal{scopes: map[accessScope]struct{}{scopeRead: {}}}, true},
		{"write denies read", scopeRead, requestPrincipal{scopes: map[accessScope]struct{}{scopeWrite: {}}}, false},
		{"write allows write", scopeWrite, requestPrincipal{scopes: map[accessScope]struct{}{scopeWrite: {}}}, true},
		{"admin denies write", scopeWrite, requestPrincipal{scopes: map[accessScope]struct{}{scopeAdmin: {}}}, false},
		{"admin allows admin", scopeAdmin, requestPrincipal{scopes: map[accessScope]struct{}{scopeAdmin: {}}}, true},
		{"read denies admin", scopeAdmin, requestPrincipal{scopes: map[accessScope]struct{}{scopeRead: {}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := withMCPAuthorization("test", test.toolScope, func(context.Context, *mcp.CallToolRequest, any) (*mcp.CallToolResult, any, error) {
				called = true
				return &mcp.CallToolResult{}, nil, nil
			})
			ctx := context.WithValue(context.Background(), principalContextKey{}, test.principal)
			result, _, err := handler(ctx, nil, nil)
			if err != nil || called != test.allowed || result.IsError == test.allowed {
				t.Fatalf("called=%t result=%+v err=%v", called, result, err)
			}
		})
	}
}

func TestMCPRequestExtraCarriesAuthenticatedPrincipalAndRejectsSpoofedContext(t *testing.T) {
	header := make(http.Header)
	header.Set(internalActorHeader, "actor-hash")
	header.Set(internalScopeHeader, "read")
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: header}}
	principal, ok := principalFromMCPRequest(context.Background(), req)
	if !ok || principal.actor != "actor-hash" || !principal.allows(scopeRead) || principal.allows(scopeWrite) {
		t.Fatalf("principal=%+v ok=%t", principal, ok)
	}
	if _, ok := principalFromMCPRequest(context.Background(), &mcp.CallToolRequest{}); ok {
		t.Fatal("missing transport identity must fail closed")
	}
}

func TestStructuredAuditRedactsSensitiveValues(t *testing.T) {
	const secret = "secret-never-log"
	const xsec = "xsec-never-log"
	var logs bytes.Buffer
	previous := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(previous) })

	router := setupRoutesWithSecurity(&AppServer{}, backendSecurityConfig{mode: authModeEnforce, credentials: []scopedCredential{newScopedCredential(secret, scopeWrite)}})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/feeds/comment", strings.NewReader(`{"account_id":"acct_sensitive","feed_id":"feed_sensitive","xsec_token":"`+xsec+`","content":"private body"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("X-Request-ID", "audit-request")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	output := logs.String()
	for _, forbidden := range []string{secret, xsec, "acct_sensitive", "feed_sensitive", "private body"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("audit log leaked %q: %s", forbidden, output)
		}
	}
	for _, required := range []string{"security_audit", "audit-request", "feeds.comment", "account_id_hash", "target_hash", "duration_ms"} {
		if !strings.Contains(output, required) {
			t.Fatalf("audit log missing %q: %s", required, output)
		}
	}
}

func TestProtectedRouteLogsNeverContainRequestSecretsAndAuditOnce(t *testing.T) {
	for _, test := range []struct{ name, token string }{{"allowed", "read-token"}, {"denied", "write-token"}} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := logrus.StandardLogger().Out
			logrus.SetOutput(&logs)
			t.Cleanup(func() { logrus.SetOutput(previous) })
			request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts?keyword=query-never-log&account_id=account-never-log", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			setupRoutesWithSecurity(&AppServer{}, scopedTestConfig()).ServeHTTP(response, request)
			output := logs.String()
			for _, forbidden := range []string{"query-never-log", "account-never-log", "/api/v1/accounts", test.token} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("log leaked %q: %s", forbidden, output)
				}
			}
			if count := strings.Count(output, `event=security_audit`); count != 1 {
				t.Fatalf("audit count=%d, log=%s", count, output)
			}
		})
	}
}

func TestMCPUncertainWriteAuditOutcome(t *testing.T) {
	for _, handlerErr := range []error{context.Canceled, context.DeadlineExceeded} {
		var logs bytes.Buffer
		previous := logrus.StandardLogger().Out
		logrus.SetOutput(&logs)
		handler := withMCPAuthorization("publish_content", scopeWrite, func(context.Context, *mcp.CallToolRequest, any) (*mcp.CallToolResult, any, error) {
			return nil, nil, handlerErr
		})
		principal := requestPrincipal{actor: "actor", scopes: map[accessScope]struct{}{scopeWrite: {}}}
		_, _, _ = handler(context.WithValue(context.Background(), principalContextKey{}, principal), nil, nil)
		logrus.SetOutput(previous)
		if !strings.Contains(logs.String(), "outcome=UNKNOWN") {
			t.Fatalf("err=%v log=%s", handlerErr, logs.String())
		}
	}
}

func TestUncertainRESTWriteErrorsUse408Or504AndAuditUnknown(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{{"canceled", context.Canceled, http.StatusRequestTimeout}, {"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout}} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := logrus.StandardLogger().Out
			logrus.SetOutput(&logs)
			defer logrus.SetOutput(previous)
			router := gin.New()
			router.Use(requestAuditMiddleware())
			router.POST("/write", func(c *gin.Context) {
				c.Set("operation", "publish.content")
				c.Set("scope", string(scopeWrite))
				status, _ := uncertainHTTPStatus(test.err)
				respondError(c, status, "WRITE_FAILED", "写操作状态未知", nil)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/write", nil))
			if response.Code != test.want || !strings.Contains(logs.String(), "outcome=UNKNOWN") {
				t.Fatalf("status=%d log=%s", response.Code, logs.String())
			}
		})
	}
}
