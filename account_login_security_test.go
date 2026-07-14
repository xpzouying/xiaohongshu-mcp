package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

type failingStatusRegistry struct {
	account.Registry
	err error
}

func (r failingStatusRegistry) UpdateStatus(context.Context, string, account.Status, string) error {
	return r.err
}

func TestPersistLoginActivatesAccount(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	_, _ = registry.Create(context.Background(), account.CreateAccountInput{ID: "acct_login", DisplayName: "Login"})
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	login := newBrowserAccountLogin(store, registry, locks).(*browserAccountLogin)
	generation := login.beginSession("acct_login")

	if err := login.persistLogin(context.Background(), "acct_login", generation, []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	got, _ := registry.Get(context.Background(), "acct_login")
	if got.Status != account.StatusActive {
		t.Fatalf("status=%q", got.Status)
	}
}

func TestPersistLoginRemovesCookiesWhenRegistryPersistenceFails(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	_, _ = registry.Create(context.Background(), account.CreateAccountInput{ID: "acct_login", DisplayName: "Login"})
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	want := errors.New("registry unavailable")
	login := newBrowserAccountLogin(store, failingStatusRegistry{Registry: registry, err: want}, locks).(*browserAccountLogin)
	generation := login.beginSession("acct_login")

	if err := login.persistLogin(context.Background(), "acct_login", generation, []byte(`[]`)); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.Load(context.Background(), "acct_login"); account.ErrorCode(err) != account.CodeCookieNotFound {
		t.Fatalf("cookie remained: %v", err)
	}
}

func TestPersistLoginDoesNotReviveCanceledSession(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	_, _ = registry.Create(context.Background(), account.CreateAccountInput{ID: "acct_login", DisplayName: "Login"})
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	login := newBrowserAccountLogin(store, registry, locks).(*browserAccountLogin)
	generation := login.beginSession("acct_login")
	login.Cancel("acct_login")

	if err := login.persistLogin(context.Background(), "acct_login", generation, []byte(`[]`)); err == nil {
		t.Fatal("canceled session persisted")
	}
	if _, err := store.Load(context.Background(), "acct_login"); account.ErrorCode(err) != account.CodeCookieNotFound {
		t.Fatalf("cookie revived: %v", err)
	}
}

type blockingCookieStore struct {
	account.CookieStore
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingCookieStore) Save(ctx context.Context, id string, data []byte) error {
	s.once.Do(func() { close(s.started) })
	select {
	case <-s.release:
		return s.CookieStore.Save(ctx, id, data)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestPersistLoginSerializesWithRemove(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	_, _ = registry.Create(context.Background(), account.CreateAccountInput{ID: "acct_login", DisplayName: "Login"})
	store, _ := account.NewFileCookieStore(root)
	blocking := &blockingCookieStore{CookieStore: store, started: make(chan struct{}), release: make(chan struct{})}
	locks, _ := account.NewLockManager(1)
	login := newBrowserAccountLogin(blocking, registry, locks).(*browserAccountLogin)
	generation := login.beginSession("acct_login")
	done := make(chan error, 1)
	go func() { done <- login.persistLogin(context.Background(), "acct_login", generation, []byte(`[]`)) }()
	<-blocking.started
	manager := account.NewManagementManager(registry, locks, store)
	if err := manager.Remove(context.Background(), "acct_login"); account.ErrorCode(err) != account.CodeAccountBusy {
		t.Fatalf("remove code=%q err=%v", account.ErrorCode(err), err)
	}
	close(blocking.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
