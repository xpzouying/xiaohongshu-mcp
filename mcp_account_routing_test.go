package main

import (
	"context"
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
	return account.Account{}, nil
}
func (r *routingRegistry) Create(context.Context, account.CreateAccountInput) (account.Account, error) {
	return account.Account{}, nil
}
func (r *routingRegistry) SetDefault(context.Context, string) error { return nil }
func (r *routingRegistry) UpdateStatus(context.Context, string, account.Status, string) error {
	return nil
}

func TestWithAccountRoutingUsesExplicitAccountAndAddsContext(t *testing.T) {
	registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct_two", Status: account.StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
	called := false
	handler := withAccountRouting[SearchFeedsArgs](registry, func(ctx context.Context, _ *mcp.CallToolRequest, _ SearchFeedsArgs) (*mcp.CallToolResult, any, error) {
		called = true
		if got := accountIDFromContext(ctx); got != "acct_two" {
			t.Fatalf("context account = %q", got)
		}
		return &mcp.CallToolResult{}, nil, nil
	})

	result, _, err := handler(context.Background(), nil, SearchFeedsArgs{AccountID: "acct_two", Keyword: "test"})
	if err != nil || result.IsError || !called {
		t.Fatalf("result=%+v called=%v err=%v", result, called, err)
	}
	if registry.requested != "acct_two" {
		t.Fatalf("requested account = %q", registry.requested)
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
	handler := withAccountRouting[ListFeedsArgs](registry, func(context.Context, *mcp.CallToolRequest, ListFeedsArgs) (*mcp.CallToolResult, any, error) {
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
	if got := result.Content[0].(*mcp.TextContent).Text; got != "账号解析失败: ACCOUNT_REQUIRED: 必须明确指定账号" {
		t.Fatalf("error text = %q", got)
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
