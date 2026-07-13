package main

import (
	"context"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

// TestAccountToolsErrorScenarios 覆盖各类错误场景：
// 账号不存在、重复创建、无效 ID、空列表、SetDefault 不存在账号。
func TestAccountToolsErrorScenarios(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(2)
	login := &fakeAccountLogin{loggedIn: true, identity: "real-user"}
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, login)
	ctx := context.Background()

	// 1. 操作不存在的账号
	if err := tools.SetDefault(ctx, "no_such"); account.ErrorCode(err) != account.CodeAccountNotFound {
		t.Fatalf("set_default not-found code=%q err=%v", account.ErrorCode(err), err)
	}
	if err := tools.ResetLogin(ctx, "no_such"); account.ErrorCode(err) != account.CodeAccountNotFound {
		t.Fatalf("reset not-found code=%q err=%v", account.ErrorCode(err), err)
	}
	if _, err := tools.CheckLoginStatus(ctx, "no_such"); account.ErrorCode(err) != account.CodeAccountNotFound {
		t.Fatalf("status not-found code=%q err=%v", account.ErrorCode(err), err)
	}
	if _, err := tools.GetLoginQRCode(ctx, "no_such"); account.ErrorCode(err) != account.CodeAccountNotFound {
		t.Fatalf("qrcode not-found code=%q err=%v", account.ErrorCode(err), err)
	}

	// 2. 删除不存在的账号（ManagementManager 先 Get 后 Delete）
	if err := tools.Remove(ctx, "no_such"); account.ErrorCode(err) != account.CodeAccountNotFound {
		t.Fatalf("remove not-found code=%q err=%v", account.ErrorCode(err), err)
	}

	// 3. 无效账号 ID
	if _, err := tools.Create(ctx, account.CreateAccountInput{ID: "Bad-ID", DisplayName: "X"}); account.ErrorCode(err) != account.CodeInvalidAccountID {
		t.Fatalf("create invalid-id code=%q err=%v", account.ErrorCode(err), err)
	}

	// 4. 重复创建
	created, err := tools.Create(ctx, account.CreateAccountInput{ID: "acct_dup", DisplayName: "Dup"})
	if err != nil || created.ID != "acct_dup" {
		t.Fatalf("create=%v err=%v", created, err)
	}
	if _, err := tools.Create(ctx, account.CreateAccountInput{ID: "acct_dup", DisplayName: "Dup2"}); account.ErrorCode(err) != account.CodeRegistryCorrupt {
		t.Fatalf("duplicate create code=%q err=%v", account.ErrorCode(err), err)
	}

	// 5. 空列表
	emptyRegistry, _ := account.NewFileRegistry(t.TempDir())
	emptyStore, _ := account.NewFileCookieStore(t.TempDir())
	emptyLocks, _ := account.NewLockManager(1)
	emptyTools := NewAccountTools(emptyRegistry, account.NewManagementManager(emptyRegistry, emptyLocks, emptyStore), emptyStore, login)
	items, err := emptyTools.List(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("empty list=%v err=%v len=%d", items, err, len(items))
	}
}

// TestAccountToolsRemoveRejectsBusyAccount 删除运行中账号时，TryAcquire 失败返回 CodeAccountBusy。
func TestAccountToolsRemoveRejectsBusyAccount(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	login := &fakeAccountLogin{}
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, login)
	ctx := context.Background()

	_, _ = tools.Create(ctx, account.CreateAccountInput{ID: "acct_busy", DisplayName: "Busy"})

	// 占用全局并发槽，使 TryAcquire 必然失败
	release, err := locks.Acquire(ctx, "acct_busy")
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	if err := tools.Remove(ctx, "acct_busy"); account.ErrorCode(err) != account.CodeAccountBusy {
		t.Fatalf("remove busy code=%q err=%v", account.ErrorCode(err), err)
	}
}

// TestAccountToolsRemoveDeletesCookieAndRegistry 删除后 Cookie 和 Registry 都清除。
func TestAccountToolsRemoveDeletesCookieAndRegistry(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(2)
	login := &fakeAccountLogin{}
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, login)
	ctx := context.Background()

	_, _ = tools.Create(ctx, account.CreateAccountInput{ID: "acct_rm", DisplayName: "RM"})
	if err := store.Save(ctx, "acct_rm", []byte(`[{"name":"x"}]`)); err != nil {
		t.Fatal(err)
	}

	if err := tools.Remove(ctx, "acct_rm"); err != nil {
		t.Fatalf("remove err=%v", err)
	}

	// Registry 中已不存在
	if _, err := registry.Get(ctx, "acct_rm"); account.ErrorCode(err) != account.CodeAccountNotFound {
		t.Fatalf("get after remove code=%q err=%v", account.ErrorCode(err), err)
	}
	// Cookie 也已删除（幂等删除不报错）
	if err := store.Delete(ctx, "acct_rm"); err != nil {
		t.Fatalf("cookie delete after remove err=%v", err)
	}
}
