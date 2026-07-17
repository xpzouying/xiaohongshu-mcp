package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

// TestAccountHandlersNilProtection 所有管理 handler 在 accountTools 未初始化时返回错误。
func TestAccountHandlersNilProtection(t *testing.T) {
	s := &AppServer{}

	// nil tools 的 handler 返回 IsError=true
	result := s.handleListAccounts(context.Background())
	if !result.IsError {
		t.Fatal("list nil should error")
	}
	result = s.handleCreateAccount(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	if !result.IsError {
		t.Fatal("create nil should error")
	}
	result = s.handleRemoveAccount(context.Background(), "acct_a")
	if !result.IsError {
		t.Fatal("remove nil should error")
	}
	result = s.handleSetDefaultAccount(context.Background(), "acct_a")
	if !result.IsError {
		t.Fatal("set_default nil should error")
	}
	result = s.handleAccountLoginStatus(context.Background(), "acct_a")
	if !result.IsError {
		t.Fatal("status nil should error")
	}
	result = s.handleAccountLoginQRCode(context.Background(), "acct_a")
	if !result.IsError {
		t.Fatal("qrcode nil should error")
	}
	result = s.handleResetAccountLogin(context.Background(), "acct_a")
	if !result.IsError {
		t.Fatal("reset nil should error")
	}
}

// TestAccountHandlerLoginStatusReturnsRealIdentity handler 返回 JSON 含真实 identity 而非固定用户名。
func TestAccountHandlerLoginStatusReturnsRealIdentity(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	login := &fakeAccountLogin{loggedIn: true, identity: "real-xhs-user"}
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, login)
	s := &AppServer{accountTools: tools}

	_, _ = tools.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})

	result := s.handleAccountLoginStatus(context.Background(), "acct_a")
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	var status AccountLoginStatus
	if err := json.Unmarshal([]byte(result.Content[0].Text), &status); err != nil {
		t.Fatal(err)
	}
	if status.Identity == nil || status.Identity.Nickname != "real-xhs-user" {
		t.Fatalf("identity=%+v", status.Identity)
	}
	if !status.IsLoggedIn || status.AccountID != "acct_a" {
		t.Fatalf("status=%+v", status)
	}
}

// TestAccountHandlerQRCodeDoesNotLeakQRInError QR 错误路径不含 QR payload。
func TestAccountHandlerQRCodeDoesNotLeakQRInError(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)

	// 故意让 login 返回不存在的账号错误——QRCode 先 Get 再调 login，所以会报 AccountNotFound
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, &fakeAccountLogin{qr: "secret-qr"})
	s := &AppServer{accountTools: tools}

	result := s.handleAccountLoginQRCode(context.Background(), "no_such")
	if !result.IsError {
		t.Fatal("should error")
	}
	for _, c := range result.Content {
		if strings.Contains(c.Text, "secret-qr") {
			t.Fatalf("QR leaked in error: %s", c.Text)
		}
	}
}

// TestAccountHandlerQRCodeSuccessContainsImage QR 成功返回含 image content block。
func TestAccountHandlerQRCodeSuccessContainsImage(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	login := &fakeAccountLogin{qr: "dGVzdA=="} // base64 "test"
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, login)
	s := &AppServer{accountTools: tools}

	_, _ = tools.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	result := s.handleAccountLoginQRCode(context.Background(), "acct_a")
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}
	if result.Content[1].Type != "image" || result.Content[1].MimeType != "image/png" {
		t.Fatalf("content[1] = %+v", result.Content[1])
	}
	if result.Content[1].Data != "dGVzdA==" {
		t.Fatalf("image data = %s", result.Content[1].Data)
	}
}

// TestAccountHandlerRemoveReturnsID remove 成功返回 JSON 含 account_id。
func TestAccountHandlerRemoveReturnsID(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(2)
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, &fakeAccountLogin{})
	s := &AppServer{accountTools: tools}

	_, _ = tools.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	result := s.handleRemoveAccount(context.Background(), "acct_a")
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["account_id"] != "acct_a" {
		t.Fatalf("account_id=%s", payload["account_id"])
	}
}
