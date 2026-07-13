package account

import (
	"context"
	"testing"
)

func TestRegistryRemovePersistsAndClearsDefault(t *testing.T) {
	root := t.TempDir()
	r, err := NewFileRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefault(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(context.Background(), "acct_a"); ErrorCode(err) != CodeAccountNotFound {
		t.Fatalf("removed account code=%q err=%v", ErrorCode(err), err)
	}
	reloaded, err := NewFileRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if accounts, err := reloaded.List(context.Background()); err != nil || len(accounts) != 0 {
		t.Fatalf("reloaded accounts=%v err=%v", accounts, err)
	}
}

func TestManagerRemoveProtectsRunningAccountAndDeletesCookie(t *testing.T) {
	root := t.TempDir()
	r, _ := NewFileRegistry(root)
	if _, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	if err := store.Save(context.Background(), "acct_a", []byte(`[{"name":"session"}]`)); err != nil {
		t.Fatal(err)
	}
	locks, _ := NewLockManager(2)
	release, err := locks.Acquire(context.Background(), "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManagementManager(r, locks, store)
	if err := manager.Remove(context.Background(), "acct_a"); ErrorCode(err) != CodeAccountBusy {
		t.Fatalf("running remove code=%q err=%v", ErrorCode(err), err)
	}
	release()
	if err := manager.Remove(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), "acct_a"); ErrorCode(err) != CodeCookieNotFound {
		t.Fatalf("cookie code=%q err=%v", ErrorCode(err), err)
	}
}
