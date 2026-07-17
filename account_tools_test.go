package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

type fakeAccountLogin struct {
	loggedIn bool
	identity string
	qr       string
	cookies  []byte
}

func (f *fakeAccountLogin) Status(context.Context, string) (bool, string, error) {
	return f.loggedIn, f.identity, nil
}
func (f *fakeAccountLogin) QRCode(context.Context, string) (string, bool, error) {
	return f.qr, f.loggedIn, nil
}
func (f *fakeAccountLogin) Cancel(string) {}

func TestAccountToolsLifecycleAndRealIdentity(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(2)
	login := &fakeAccountLogin{loggedIn: true, identity: "real-user-123"}
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, login)
	created, err := tools.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	if err != nil || created.ID != "acct_a" {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if err := tools.SetDefault(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	status, err := tools.CheckLoginStatus(context.Background(), "acct_a")
	if err != nil || !status.IsLoggedIn || status.Identity == nil || status.Identity.Nickname != "real-user-123" || status.AccountID != "acct_a" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if err := tools.ResetLogin(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	accountValue, _ := registry.Get(context.Background(), "acct_a")
	if accountValue.Status != account.StatusNeedsLogin {
		t.Fatalf("status=%s", accountValue.Status)
	}
}

func TestAccountToolsQRCodeDoesNotExposeItThroughErrors(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	_, _ = registry.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, &fakeAccountLogin{qr: "secret-qr-payload"})
	result, err := tools.GetLoginQRCode(context.Background(), "acct_a")
	if err != nil || result.Image != "secret-qr-payload" || result.AccountID != "acct_a" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAccountLoginStatusJSONIdentityContract(t *testing.T) {
	encoded, err := json.Marshal(AccountLoginStatus{
		AccountID:  "acct_a",
		IsLoggedIn: true,
		Identity:   &AccountIdentity{Nickname: "真实昵称"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	identity, ok := result["identity"].(map[string]any)
	if !ok || identity["nickname"] != "真实昵称" {
		t.Fatalf("identity = %#v, want nickname object", result["identity"])
	}

	empty, err := json.Marshal(AccountLoginStatus{AccountID: "acct_a"})
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `{"account_id":"acct_a","is_logged_in":false}` {
		t.Fatalf("empty identity JSON = %s", empty)
	}
}
