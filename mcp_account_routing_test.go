package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

type routingRegistry struct {
	resolved  account.ResolvedAccount
	err       error
	requested string
}

func (r *routingRegistry) Resolve(_ context.Context, requested string) (account.ResolvedAccount, error) {
	r.requested = requested
	return r.resolved, r.err
}

func (r *routingRegistry) List(context.Context) ([]account.Account, error) { return nil, nil }
func (r *routingRegistry) Get(context.Context, string) (account.Account, error) {
	return r.resolved.Account, r.err
}
func (r *routingRegistry) Create(context.Context, account.CreateAccountInput) (account.Account, error) {
	return account.Account{}, nil
}
func (r *routingRegistry) SetDefault(context.Context, string) error { return nil }
func (r *routingRegistry) UpdateStatus(context.Context, string, account.Status, string) error {
	return nil
}

type routingBrowser struct{ closeCount int }

func (b *routingBrowser) Close() { b.closeCount++ }

type routingFactory struct{ browser account.Browser }

func (f routingFactory) New(context.Context, account.Account) (account.Browser, error) {
	return f.browser, nil
}

func TestWithAccountRoutingUsesExplicitAccountAndAddsContext(t *testing.T) {
	registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct_two", Status: account.StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
	locks, err := account.NewLockManager(1)
	if err != nil {
		t.Fatal(err)
	}
	browser := &routingBrowser{}
	manager := account.NewAccountManager(registry, locks, routingFactory{browser: browser})
	called := false
	handler := withAccountRouting[SearchFeedsArgs](manager, account.OperationRead, func(ctx context.Context, _ *mcp.CallToolRequest, _ SearchFeedsArgs) (*mcp.CallToolResult, any, error) {
		called = true
		if got := accountIDFromContext(ctx); got != "acct_two" {
			t.Fatalf("context account = %q", got)
		}
		if got := accountBrowserFromContext(ctx); got != browser {
			t.Fatalf("context browser = %#v", got)
		}
		closeBrowser(ctx, browser)
		return &mcp.CallToolResult{}, nil, nil
	})

	result, _, err := handler(context.Background(), nil, SearchFeedsArgs{AccountID: "acct_two", Keyword: "test"})
	if err != nil || result.IsError || !called {
		t.Fatalf("result=%+v called=%v err=%v", result, called, err)
	}
	if registry.requested != "acct_two" {
		t.Fatalf("requested account = %q", registry.requested)
	}
	if browser.closeCount != 1 {
		t.Fatalf("browser close count = %d, want 1", browser.closeCount)
	}
}

func TestCloseBrowserClosesLegacyBrowser(t *testing.T) {
	browser := &routingBrowser{}
	closeBrowser(context.Background(), browser)
	if browser.closeCount != 1 {
		t.Fatalf("browser close count = %d, want 1", browser.closeCount)
	}
}

func TestWithAccountRoutingReturnsResolutionErrorWithoutCallingHandler(t *testing.T) {
	root := t.TempDir()
	registry, err := account.NewFileRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"acct_one", "acct_two"} {
		if _, err := registry.Create(context.Background(), account.CreateAccountInput{ID: id, DisplayName: id}); err != nil {
			t.Fatal(err)
		}
	}
	called := false
	locks, _ := account.NewLockManager(1)
	manager := account.NewAccountManager(registry, locks, routingFactory{browser: &routingBrowser{}})
	handler := withAccountRouting[ListFeedsArgs](manager, account.OperationRead, func(context.Context, *mcp.CallToolRequest, ListFeedsArgs) (*mcp.CallToolResult, any, error) {
		called = true
		return &mcp.CallToolResult{}, nil, nil
	})

	result, _, err := handler(context.Background(), nil, ListFeedsArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || called {
		t.Fatalf("result=%+v called=%v", result, called)
	}
	if got := result.Content[0].(*mcp.TextContent).Text; got != "账号执行失败: ACCOUNT_REQUIRED: 必须明确指定账号" {
		t.Fatalf("error text = %q", got)
	}
}

func TestAccountBrowserFactoryUsesOnlyResolvedAccountCookie(t *testing.T) {
	root := t.TempDir()
	store, err := account.NewFileCookieStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), "acct_one", []byte(`[{"name":"one"}]`)); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(t.TempDir(), "cookies.json")
	if err := os.WriteFile(legacy, []byte(`[{"name":"legacy"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COOKIES_PATH", legacy)

	var paths []string
	factory := newAccountBrowserFactory(store, func(path string) account.Browser {
		paths = append(paths, path)
		return &routingBrowser{}
	})
	if _, err := factory.New(context.Background(), account.Account{ID: "acct_one"}); err != nil {
		t.Fatal(err)
	}
	want, _ := store.Path("acct_one")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("cookie paths = %#v, want %q", paths, want)
	}

	if _, err := factory.New(context.Background(), account.Account{ID: "acct_two"}); account.ErrorCode(err) != account.CodeCookieNotFound {
		t.Fatalf("missing cookie code = %q, err=%v", account.ErrorCode(err), err)
	}
	if len(paths) != 1 {
		t.Fatalf("browser created for missing cookie: %#v", paths)
	}
	if _, err := factory.New(context.Background(), account.Account{ID: "../legacy"}); account.ErrorCode(err) != account.CodeInvalidAccountID {
		t.Fatalf("invalid path code = %q, err=%v", account.ErrorCode(err), err)
	}

	registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct_two", Status: account.StatusActive}}}
	locks, _ := account.NewLockManager(1)
	manager := account.NewAccountManager(registry, locks, factory)
	if _, err := manager.WithAccount(context.Background(), "acct_two", account.OperationRead, func(context.Context, account.Account, account.Browser) error { return nil }); account.ErrorCode(err) != account.CodeAccountLoginRequired {
		t.Fatalf("business missing cookie code = %q, err=%v", account.ErrorCode(err), err)
	}
}

func TestBusinessToolArgsExposeOptionalAccountID(t *testing.T) {
	types := []any{
		PublishContentArgs{}, PublishVideoArgs{}, ListFeedsArgs{}, SearchFeedsArgs{}, FeedDetailArgs{},
		UserProfileArgs{}, PostCommentArgs{}, ReplyCommentArgs{}, LikeFeedArgs{}, FavoriteFeedArgs{},
	}
	for _, value := range types {
		typeOf := reflect.TypeOf(value)
		field, ok := typeOf.FieldByName("AccountID")
		if !ok {
			t.Errorf("%s lacks AccountID", typeOf.Name())
			continue
		}
		if got := field.Tag.Get("json"); got != "account_id,omitempty" {
			t.Errorf("%s AccountID json tag = %q", typeOf.Name(), got)
		}
	}
}
