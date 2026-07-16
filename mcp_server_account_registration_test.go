package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

func TestAccountModeKeepsBusinessToolsRegistered(t *testing.T) {
	root := t.TempDir()
	registry, err := account.NewFileRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := account.NewFileCookieStore(root)
	if err != nil {
		t.Fatal(err)
	}
	locks, err := account.NewLockManager(1)
	if err != nil {
		t.Fatal(err)
	}
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, &fakeAccountLogin{})
	app := &AppServer{accountTools: tools}
	server := InitMCPServer(app)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "registration-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	registered := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		registered[tool.Name] = true
	}
	wantTools := []string{
		"list_accounts", "create_account", "remove_account", "set_default_account",
		"check_login_status", "get_login_qrcode", "reset_login", "publish_content",
		"list_feeds", "search_feeds", "get_feed_detail", "user_profile",
		"post_comment_to_feed", "reply_comment_in_feed", "publish_with_video",
		"like_feed", "favorite_feed",
	}
	for _, name := range wantTools {
		if !registered[name] {
			t.Errorf("tool %q is not registered in account mode", name)
		}
	}
	if registered["delete_cookies"] {
		t.Error("legacy delete_cookies is registered in account mode")
	}
	if got, want := len(result.Tools), len(wantTools); got != want {
		t.Errorf("registered tool count = %d, want %d", got, want)
	}

	cancel()
	<-serverErr
}
