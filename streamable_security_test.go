package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

type testBearerContextKey struct{}
type testBearerTransport struct{}

func (testBearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header = request.Header.Clone()
	request.Header.Set("Authorization", "Bearer "+request.Context().Value(testBearerContextKey{}).(string))
	request.Header.Set(internalActorHeader, "forged")
	request.Header.Set(internalScopeHeader, "read,write,admin")
	return http.DefaultTransport.RoundTrip(request)
}

func TestStreamableHTTPSessionReauthenticatesEveryConcurrentRequest(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "security-test", Version: "1"}, nil)
	for _, item := range []struct {
		name  string
		scope accessScope
	}{{"read", scopeRead}, {"write", scopeWrite}, {"admin", scopeAdmin}} {
		mcp.AddTool(server, &mcp.Tool{Name: item.name}, withMCPAuthorization(item.name, item.scope, func(context.Context, *mcp.CallToolRequest, any) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{}, nil, nil
		}))
	}
	httpServer := httptest.NewServer(setupRoutesWithSecurity(&AppServer{mcpServer: server}, scopedTestConfig()))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "security-client", Version: "1"}, nil)
	connectContext := context.WithValue(context.Background(), testBearerContextKey{}, "read-token")
	session, err := client.Connect(connectContext, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp", HTTPClient: &http.Client{Transport: testBearerTransport{}}, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	var wait sync.WaitGroup
	for _, token := range []string{"read-token", "write-token", "admin-token"} {
		for _, tool := range []string{"read", "write", "admin"} {
			wait.Add(1)
			go func(token, tool string) {
				defer wait.Done()
				ctx := context.WithValue(context.Background(), testBearerContextKey{}, token)
				result, callErr := session.CallTool(ctx, &mcp.CallToolParams{Name: tool})
				if callErr != nil {
					t.Errorf("%s/%s: %v", token, tool, callErr)
					return
				}
				wantAllowed := strings.TrimSuffix(token, "-token") == tool
				if result.IsError == wantAllowed {
					t.Errorf("%s/%s isError=%t", token, tool, result.IsError)
				}
			}(token, tool)
		}
	}
	wait.Wait()
}

type countingRegistry struct {
	account.Registry
	resolveCalls atomic.Int64
}

func (r *countingRegistry) Resolve(ctx context.Context, id string) (account.ResolvedAccount, error) {
	r.resolveCalls.Add(1)
	return r.Registry.Resolve(ctx, id)
}

func TestRegisteredStreamableHTTPToolsAuthorizeBeforeAccountRouting(t *testing.T) {
	registry, err := account.NewFileRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Create(context.Background(), account.CreateAccountInput{ID: "acct_test", DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	counting := &countingRegistry{Registry: registry}
	locks, err := account.NewLockManager(1)
	if err != nil {
		t.Fatal(err)
	}
	manager := account.NewAccountManager(counting, locks, nil)
	app := &AppServer{
		accountTools:    &AccountTools{registry: counting},
		accountRegistry: counting,
		accountManager:  manager,
	}
	app.mcpServer = InitMCPServer(app)

	httpServer := httptest.NewServer(setupRoutesWithSecurity(app, scopedTestConfig()))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "registered-security-client", Version: "1"}, nil)
	connectContext := context.WithValue(context.Background(), testBearerContextKey{}, "read-token")
	session, err := client.Connect(connectContext, &mcp.StreamableClientTransport{
		Endpoint:   httpServer.URL + "/mcp",
		HTTPClient: &http.Client{Transport: testBearerTransport{}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tests := []struct {
		name, token, tool string
		arguments         map[string]any
		wantForbidden     bool
		wantRoute         bool
	}{
		{"read token reaches read route", "read-token", "list_feeds", map[string]any{"account_id": "acct_test"}, false, true},
		{"write token denied read before route", "write-token", "list_feeds", map[string]any{"account_id": "acct_test"}, true, false},
		{"admin token denied read before route", "admin-token", "list_feeds", map[string]any{"account_id": "acct_test"}, true, false},
		{"write token reaches write route", "write-token", "publish_content", map[string]any{"account_id": "acct_test", "title": "test", "content": "test", "images": []string{"/not-used"}}, false, true},
		{"read token denied write before route", "read-token", "publish_content", map[string]any{"account_id": "acct_test", "title": "test", "content": "test", "images": []string{"/not-used"}}, true, false},
		{"admin token denied write before route", "admin-token", "publish_content", map[string]any{"account_id": "acct_test", "title": "test", "content": "test", "images": []string{"/not-used"}}, true, false},
		{"admin token reaches admin handler", "admin-token", "create_account", map[string]any{"account_id": "acct_new", "display_name": "New"}, false, false},
		{"read token denied admin", "read-token", "create_account", map[string]any{"account_id": "acct_read", "display_name": "Read"}, true, false},
		{"write token denied admin", "write-token", "create_account", map[string]any{"account_id": "acct_write", "display_name": "Write"}, true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := counting.resolveCalls.Load()
			ctx := context.WithValue(context.Background(), testBearerContextKey{}, test.token)
			result, callErr := session.CallTool(ctx, &mcp.CallToolParams{Name: test.tool, Arguments: test.arguments})
			if callErr != nil {
				t.Fatal(callErr)
			}
			forbidden := result.IsError && strings.Contains(mcpResultText(result), "FORBIDDEN")
			if forbidden != test.wantForbidden {
				t.Fatalf("forbidden=%t result=%+v", forbidden, result)
			}
			gotRoutes := counting.resolveCalls.Load() - before
			if test.wantRoute && gotRoutes != 1 {
				t.Fatalf("account route calls=%d, want 1", gotRoutes)
			}
			if !test.wantRoute && gotRoutes != 0 {
				t.Fatalf("account route calls=%d, want 0", gotRoutes)
			}
		})
	}

	t.Run("same session concurrent tokens stay isolated", func(t *testing.T) {
		before := counting.resolveCalls.Load()
		calls := []struct {
			token, tool string
			arguments   map[string]any
		}{
			{"admin-token", "list_feeds", map[string]any{"account_id": "acct_test"}},
			{"read-token", "publish_content", map[string]any{"account_id": "acct_test", "title": "test", "content": "test", "images": []string{"/not-used"}}},
			{"write-token", "create_account", map[string]any{"account_id": "acct_concurrent", "display_name": "Concurrent"}},
		}
		var wait sync.WaitGroup
		for _, call := range calls {
			wait.Add(1)
			go func() {
				defer wait.Done()
				ctx := context.WithValue(context.Background(), testBearerContextKey{}, call.token)
				result, callErr := session.CallTool(ctx, &mcp.CallToolParams{Name: call.tool, Arguments: call.arguments})
				if callErr != nil {
					t.Errorf("%s/%s: %v", call.token, call.tool, callErr)
					return
				}
				if !result.IsError || !strings.Contains(mcpResultText(result), "FORBIDDEN") {
					t.Errorf("%s/%s result=%+v", call.token, call.tool, result)
				}
			}()
		}
		wait.Wait()
		if got := counting.resolveCalls.Load() - before; got != 0 {
			t.Fatalf("account route calls=%d, want 0", got)
		}
	})
}

func mcpResultText(result *mcp.CallToolResult) string {
	var text strings.Builder
	for _, content := range result.Content {
		switch item := content.(type) {
		case *mcp.TextContent:
			text.WriteString(item.Text)
		}
	}
	return text.String()
}

func TestRegisteredWriteErrorsPreserveToolSemanticsAndAuditOutcome(t *testing.T) {
	registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct_test", Status: account.StatusActive}}}
	locks, err := account.NewLockManager(1)
	if err != nil {
		t.Fatal(err)
	}
	manager := account.NewAccountManager(registry, locks, routingFactory{browser: &routingBrowser{}})
	currentErr := error(context.DeadlineExceeded)
	app := &AppServer{
		accountManager: manager,
		publishContent: func(context.Context, *PublishRequest) (*PublishResponse, error) {
			return nil, currentErr
		},
		publishVideo: func(context.Context, *PublishVideoRequest) (*PublishVideoResponse, error) {
			return nil, currentErr
		},
		postComment: func(context.Context, string, string, string) (*PostCommentResponse, error) {
			return nil, currentErr
		},
		replyComment: func(context.Context, string, string, string, string, string) (*ReplyCommentResponse, error) {
			return nil, currentErr
		},
	}
	app.mcpServer = InitMCPServer(app)

	var logs bytes.Buffer
	previous := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(previous) })

	httpServer := httptest.NewServer(setupRoutesWithSecurity(app, scopedTestConfig()))
	t.Cleanup(httpServer.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "unknown-write-test", Version: "1"}, nil)
	connectContext := context.WithValue(context.Background(), testBearerContextKey{}, "write-token")
	session, err := client.Connect(connectContext, &mcp.StreamableClientTransport{
		Endpoint:   httpServer.URL + "/mcp",
		HTTPClient: &http.Client{Transport: testBearerTransport{}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	tests := []struct {
		tool, action string
		arguments    map[string]any
	}{
		{"publish_content", "发布", map[string]any{"account_id": "acct_test", "title": "test", "content": "test", "images": []string{"/not-used"}}},
		{"publish_with_video", "发布", map[string]any{"account_id": "acct_test", "title": "test", "content": "test", "video": "/not-used"}},
		{"post_comment_to_feed", "发表评论", map[string]any{"account_id": "acct_test", "feed_id": "feed", "xsec_token": "xsec", "content": "test"}},
		{"reply_comment_in_feed", "回复评论", map[string]any{"account_id": "acct_test", "feed_id": "feed", "xsec_token": "xsec", "comment_id": "comment", "content": "test"}},
	}
	for _, test := range tests {
		for _, uncertainErr := range []error{context.DeadlineExceeded, context.Canceled} {
			logs.Reset()
			currentErr = uncertainErr
			result, callErr := session.CallTool(connectContext, &mcp.CallToolParams{Name: test.tool, Arguments: test.arguments})
			if callErr != nil {
				t.Fatalf("%s uncertain write returned protocol error: %v", test.tool, callErr)
			}
			text := mcpResultText(result)
			if !result.IsError || !strings.Contains(text, "状态未知") || !strings.Contains(text, "请勿自动重试") {
				t.Fatalf("%s uncertain result=%+v text=%q", test.tool, result, text)
			}
			output := logs.String()
			if !strings.Contains(output, "operation=mcp."+test.tool) || !strings.Contains(output, "outcome=UNKNOWN") || strings.Count(output, "event=security_audit") != 1 {
				t.Fatalf("%s uncertain audit log=%s", test.tool, output)
			}
		}

		logs.Reset()
		currentErr = errors.New("deterministic failure")
		result, callErr := session.CallTool(connectContext, &mcp.CallToolParams{Name: test.tool, Arguments: test.arguments})
		if callErr != nil {
			t.Fatalf("%s deterministic write returned protocol error: %v", test.tool, callErr)
		}
		text := mcpResultText(result)
		if !result.IsError || !strings.Contains(text, test.action+"失败") || strings.Contains(text, "状态未知") {
			t.Fatalf("%s deterministic result=%+v text=%q", test.tool, result, text)
		}
		output := logs.String()
		if !strings.Contains(output, "operation=mcp."+test.tool) || !strings.Contains(output, "outcome=failure") || strings.Contains(output, "outcome=UNKNOWN") || strings.Count(output, "event=security_audit") != 1 {
			t.Fatalf("%s deterministic audit log=%s", test.tool, output)
		}
	}
}
