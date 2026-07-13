package account

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRegistryRejectsUnsupportedAndInvalidDocuments(t *testing.T) {
	now := "2026-07-13T01:02:03Z"
	tests := []struct {
		name string
		doc  string
		code Code
	}{
		{"unsupported version", `{"schema_version":2,"default_account_id":null,"accounts":[]}`, CodeRegistryVersionUnsupported},
		{"duplicate ID", `{"schema_version":1,"default_account_id":null,"accounts":[{"id":"acct_a","display_name":"A","status":"active","created_at":"` + now + `","updated_at":"` + now + `"},{"id":"acct_a","display_name":"B","status":"active","created_at":"` + now + `","updated_at":"` + now + `"}]}`, CodeRegistryCorrupt},
		{"missing default", `{"schema_version":1,"default_account_id":"acct_b","accounts":[{"id":"acct_a","display_name":"A","status":"active","created_at":"` + now + `","updated_at":"` + now + `"}]}`, CodeRegistryCorrupt},
		{"disabled default", `{"schema_version":1,"default_account_id":"acct_a","accounts":[{"id":"acct_a","display_name":"A","status":"disabled","created_at":"` + now + `","updated_at":"` + now + `"}]}`, CodeRegistryCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "accounts.json"), []byte(tt.doc), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewFileRegistry(root); ErrorCode(err) != tt.code {
				t.Fatalf("code = %q, want %q (err=%v)", ErrorCode(err), tt.code, err)
			}
		})
	}
}

func TestRegistryPublicMethodsHonorCanceledContext(t *testing.T) {
	r, err := NewFileRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checks := []func() error{
		func() error { _, err := r.List(ctx); return err },
		func() error { _, err := r.Get(ctx, "acct_a"); return err },
		func() error { _, err := r.Resolve(ctx, "acct_a"); return err },
		func() error { _, err := r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "A"}); return err },
		func() error { return r.SetDefault(ctx, "acct_a") },
		func() error { return r.UpdateStatus(ctx, "acct_a", StatusActive, "test") },
	}
	for i, check := range checks {
		if err := check(); ErrorCode(err) != CodeOperationCanceled || !errors.Is(err, context.Canceled) {
			t.Errorf("check %d: code=%q err=%v", i, ErrorCode(err), err)
		}
	}
}

func TestCookieStoreRejectsSymlinkAndKeepsAccountsIsolated(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileCookieStore(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Save(ctx, "acct_b", []byte(`[{"name":"b"}]`)); err != nil {
		t.Fatal(err)
	}
	aPath, err := store.Path("acct_a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(aPath), 0700); err != nil {
		t.Fatal(err)
	}
	bPath, _ := store.Path("acct_b")
	if err := os.Symlink(bPath, aPath); err != nil {
		t.Skipf("filesystem does not support symlinks: %v", err)
	}
	if _, err := store.Load(ctx, "acct_a"); ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("symlink load code = %q, err=%v", ErrorCode(err), err)
	}
	if err := store.Delete(ctx, "acct_a"); ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("symlink delete code = %q, err=%v", ErrorCode(err), err)
	}
	data, err := store.Load(ctx, "acct_b")
	if err != nil || string(data) != `[{"name":"b"}]` {
		t.Fatalf("account B changed: %s, %v", data, err)
	}
}

func TestCookieStoreRejectsSymlinkedParentDirectories(t *testing.T) {
	ctx := context.Background()
	data := []byte(`[{"name":"safe"}]`)
	operations := []struct {
		name string
		run  func(*FileCookieStore) error
	}{
		{"save", func(store *FileCookieStore) error { return store.Save(ctx, "acct_a", data) }},
		{"load", func(store *FileCookieStore) error { _, err := store.Load(ctx, "acct_a"); return err }},
		{"delete", func(store *FileCookieStore) error { return store.Delete(ctx, "acct_a") }},
	}
	for _, linkLevel := range []string{"accounts", "account"} {
		for _, operation := range operations {
			t.Run(linkLevel+"/"+operation.name, func(t *testing.T) {
				root := t.TempDir()
				outside := t.TempDir()
				if linkLevel == "accounts" {
					if err := os.Symlink(outside, filepath.Join(root, "accounts")); err != nil {
						t.Skipf("filesystem does not support symlinks: %v", err)
					}
				} else {
					if err := os.Mkdir(filepath.Join(root, "accounts"), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outside, filepath.Join(root, "accounts", "acct_a")); err != nil {
						t.Skipf("filesystem does not support symlinks: %v", err)
					}
				}
				store, err := NewFileCookieStore(root)
				if err != nil {
					t.Fatal(err)
				}
				if err := operation.run(store); ErrorCode(err) != CodePersistenceFailed {
					t.Fatalf("code=%q err=%v", ErrorCode(err), err)
				}
				if _, err := os.Stat(filepath.Join(outside, "cookies.json")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("outside cookie touched: %v", err)
				}
			})
		}
	}
}

func TestMigrateLegacyRejectsSymlinkedDestination(t *testing.T) {
	for _, linkLevel := range []string{"accounts", "account"} {
		t.Run(linkLevel, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			legacy := filepath.Join(t.TempDir(), "cookies.json")
			if err := os.WriteFile(legacy, []byte(`[{"name":"legacy"}]`), 0600); err != nil {
				t.Fatal(err)
			}
			if linkLevel == "accounts" {
				if err := os.Symlink(outside, filepath.Join(root, "accounts")); err != nil {
					t.Skipf("filesystem does not support symlinks: %v", err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(root, "accounts"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "accounts", "default")); err != nil {
					t.Skipf("filesystem does not support symlinks: %v", err)
				}
			}
			if _, err := MigrateLegacy(context.Background(), MigrationOptions{Root: root, Candidates: []string{legacy}}); ErrorCode(err) != CodePersistenceFailed {
				t.Fatalf("code=%q err=%v", ErrorCode(err), err)
			}
			if _, err := os.Stat(filepath.Join(outside, "cookies.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("migration escaped root: %v", err)
			}
		})
	}
}

func TestLockManagerCancellationAndIdempotentRelease(t *testing.T) {
	lm, err := NewLockManager(1)
	if err != nil {
		t.Fatal(err)
	}
	release, err := lm.Acquire(context.Background(), "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lm.Acquire(ctx, "acct_b"); ErrorCode(err) != CodeOperationCanceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel code=%q err=%v", ErrorCode(err), err)
	}
	release()
	release()
	next, err := lm.Acquire(context.Background(), "acct_b")
	if err != nil {
		t.Fatalf("slot leaked: %v", err)
	}
	next()
}

type blockingRegistry struct {
	mu        sync.Mutex
	account   Account
	resolved  chan struct{}
	getPermit chan struct{}
}

func (r *blockingRegistry) List(context.Context) ([]Account, error) { return []Account{r.account}, nil }
func (r *blockingRegistry) Get(context.Context, string) (Account, error) {
	<-r.getPermit
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.account, nil
}
func (r *blockingRegistry) Resolve(context.Context, string) (ResolvedAccount, error) {
	r.mu.Lock()
	account := r.account
	r.mu.Unlock()
	close(r.resolved)
	return ResolvedAccount{Account: account, SelectionSource: SelectionDefault}, nil
}
func (r *blockingRegistry) Create(context.Context, CreateAccountInput) (Account, error) {
	panic("unexpected Create")
}
func (r *blockingRegistry) SetDefault(context.Context, string) error { panic("unexpected SetDefault") }
func (r *blockingRegistry) UpdateStatus(context.Context, string, Status, string) error {
	panic("unexpected UpdateStatus")
}

func TestAccountManagerFreezesResolvedAccountAndRechecksItsStatus(t *testing.T) {
	r := &blockingRegistry{
		account:   Account{ID: "acct_a", Status: StatusActive},
		resolved:  make(chan struct{}),
		getPermit: make(chan struct{}),
	}
	lm, _ := NewLockManager(1)
	factory := &fakeFactory{}
	m := NewAccountManager(r, lm, factory)
	done := make(chan error, 1)
	go func() {
		resolved, err := m.WithAccount(context.Background(), "", OperationWrite, func(context.Context, Account, Browser) error { return nil })
		if resolved.Account.ID != "acct_a" {
			done <- errors.New("resolved account changed")
			return
		}
		done <- err
	}()
	<-r.resolved
	r.mu.Lock()
	r.account.Status = StatusRiskHold
	r.mu.Unlock()
	close(r.getPermit)
	select {
	case err := <-done:
		if ErrorCode(err) != CodeAccountRiskHold {
			t.Fatalf("code=%q err=%v", ErrorCode(err), err)
		}
	case <-time.After(time.Second):
		t.Fatal("manager did not finish")
	}
	if factory.made != 0 {
		t.Fatal("browser created after status changed while queued")
	}
}

func TestMigrateLegacyNoSourceAndIdenticalSourcesAreIdempotent(t *testing.T) {
	ctx := context.Background()
	emptyRoot := t.TempDir()
	result, err := MigrateLegacy(ctx, MigrationOptions{Root: emptyRoot, Candidates: []string{filepath.Join(t.TempDir(), "missing.json")}})
	if err != nil || result.Migrated {
		t.Fatalf("empty migration=%#v err=%v", result, err)
	}
	if _, err := NewFileRegistry(emptyRoot); err != nil {
		t.Fatalf("empty registry invalid: %v", err)
	}

	root := t.TempDir()
	data := []byte(`[{"name":"same"}]`)
	var candidates []string
	for i := 0; i < 2; i++ {
		path := filepath.Join(t.TempDir(), "cookies.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, path)
	}
	result, err = MigrateLegacy(ctx, MigrationOptions{Root: root, Candidates: candidates})
	if err != nil || !result.Migrated || result.AccountID != "default" {
		t.Fatalf("identical migration=%#v err=%v", result, err)
	}
	result, err = MigrateLegacy(ctx, MigrationOptions{Root: root, Candidates: candidates})
	if err != nil || result.Migrated {
		t.Fatalf("repeat migration=%#v err=%v", result, err)
	}
}
