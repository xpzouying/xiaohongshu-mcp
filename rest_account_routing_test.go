package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

func TestRESTAccountRoutingSelectsAccountFromRequestAndInjectsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name        string
		method      string
		target      string
		body        string
		wantAccount string
	}{
		{name: "query", method: http.MethodGet, target: "/feeds?account_id=acct_one", wantAccount: "acct_one"},
		{name: "json", method: http.MethodPost, target: "/publish", body: `{"account_id":"acct_two","title":"t"}`, wantAccount: "acct_two"},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: test.wantAccount, Status: account.StatusActive}}}
			locks, _ := account.NewLockManager(1)
			browser := &routingBrowser{}
			manager := account.NewAccountManager(registry, locks, routingFactory{browser: browser})
			router := gin.New()
			router.Handle(test.method, strings.Split(test.target, "?")[0], withRESTAccountRouting(manager, account.OperationRead, func(c *gin.Context) {
				if got := accountIDFromContext(c.Request.Context()); got != test.wantAccount {
					t.Fatalf("context account = %q", got)
				}
				if got := accountBrowserFromContext(c.Request.Context()); got != browser {
					t.Fatalf("context browser = %#v", got)
				}
				closeBrowser(c.Request.Context(), browser)
				if test.body != "" {
					var payload map[string]any
					if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
						t.Fatalf("body was not restored: %v", err)
					}
				}
				c.Status(http.StatusNoContent)
			}))

			req := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			if test.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			if registry.requested != test.wantAccount {
				t.Fatalf("requested account = %q", registry.requested)
			}
			if browser.closeCount != 1 {
				t.Fatalf("browser close count = %d, want 1", browser.closeCount)
			}
		})
	}
}

func TestRESTAccountRoutingMapsAccountErrorsWithoutCallingHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		requested  string
		resolved   account.Account
		resolveErr error
		operation  account.OperationKind
		wantStatus int
		wantCode   string
	}{
		{name: "missing account", resolveErr: &account.Error{Code: account.CodeAccountRequired, Message: "必须明确指定账号"}, operation: account.OperationRead, wantStatus: http.StatusBadRequest, wantCode: "ACCOUNT_REQUIRED"},
		{name: "unknown account", requested: "acct_none", resolveErr: &account.Error{Code: account.CodeAccountNotFound, Message: "账号不存在"}, operation: account.OperationRead, wantStatus: http.StatusNotFound, wantCode: "ACCOUNT_NOT_FOUND"},
		{name: "paused write", requested: "acct_one", resolved: account.Account{ID: "acct_one", Status: account.StatusPaused}, operation: account.OperationWrite, wantStatus: http.StatusConflict, wantCode: "ACCOUNT_PAUSED"},
		{name: "risk hold read", requested: "acct_one", resolved: account.Account{ID: "acct_one", Status: account.StatusRiskHold}, operation: account.OperationRead, wantStatus: http.StatusConflict, wantCode: "ACCOUNT_RISK_HOLD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := &routingRegistry{resolved: account.ResolvedAccount{Account: test.resolved}, err: test.resolveErr}
			locks, _ := account.NewLockManager(1)
			manager := account.NewAccountManager(registry, locks, routingFactory{browser: &routingBrowser{}})
			called := false
			router := gin.New()
			router.GET("/business", withRESTAccountRouting(manager, test.operation, func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			}))
			target := "/business"
			if test.requested != "" {
				target += "?account_id=" + test.requested
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
			if called {
				t.Fatal("handler was called")
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			var payload ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Code != test.wantCode {
				t.Fatalf("code = %q", payload.Code)
			}
		})
	}
}

func TestRESTAccountRoutingPropagatesMissingCookieAndCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct_one", Status: account.StatusActive}}}
	locks, _ := account.NewLockManager(1)
	manager := account.NewAccountManager(registry, locks, routingFactoryError{err: &account.Error{Code: account.CodeCookieNotFound, Message: "Cookie 不存在"}})
	router := gin.New()
	router.GET("/business", withRESTAccountRouting(manager, account.OperationRead, func(c *gin.Context) { c.Status(http.StatusNoContent) }))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/business?account_id=acct_one", nil))
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "ACCOUNT_LOGIN_REQUIRED") {
		t.Fatalf("missing cookie response: status=%d body=%s", response.Code, response.Body.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/business?account_id=acct_one", nil).WithContext(ctx))
	if response.Code != http.StatusRequestTimeout || !strings.Contains(response.Body.String(), "OPERATION_CANCELED") {
		t.Fatalf("canceled response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRESTAccountRoutingUsesManagerConcurrencyGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct_one", Status: account.StatusActive}}}
	locks, _ := account.NewLockManager(1)
	manager := account.NewAccountManager(registry, locks, routingFactory{browser: &routingBrowser{}})
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	handler := withRESTAccountRouting(manager, account.OperationRead, func(c *gin.Context) {
		once.Do(func() { close(entered) })
		<-release
		c.Status(http.StatusNoContent)
	})
	router := gin.New()
	router.GET("/business", handler)
	go router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/business?account_id=acct_one", nil))
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/business?account_id=acct_one", nil).WithContext(ctx))
	close(release)
	if response.Code != http.StatusTooManyRequests || !strings.Contains(response.Body.String(), "ACCOUNT_BUSY") {
		t.Fatalf("busy response: status=%d body=%s", response.Code, response.Body.String())
	}
}

type routingFactoryError struct{ err error }

func (f routingFactoryError) New(context.Context, account.Account) (account.Browser, error) {
	return nil, f.err
}

func TestBusinessRESTRoutesUseAccountRouting(t *testing.T) {
	source, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`api.POST("/publish", withRESTAccountRouting`,
		`api.POST("/publish_video", withRESTAccountRouting`,
		`api.GET("/feeds/list", withRESTAccountRouting`,
		`api.GET("/feeds/search", withRESTAccountRouting`,
		`api.POST("/feeds/search", withRESTAccountRouting`,
		`api.POST("/feeds/detail", withRESTAccountRouting`,
		`api.POST("/user/profile", withRESTAccountRouting`,
		`api.POST("/feeds/comment", withRESTAccountRouting`,
		`api.POST("/feeds/comment/reply", withRESTAccountRouting`,
		`api.POST("/feeds/like", withRESTAccountRouting`,
		`api.POST("/feeds/favorite", withRESTAccountRouting`,
		`api.GET("/user/me", withRESTAccountRouting`,
	}
	for _, route := range want {
		if !strings.Contains(string(source), route) {
			t.Errorf("business route is not account-routed: %s", route)
		}
	}
}
