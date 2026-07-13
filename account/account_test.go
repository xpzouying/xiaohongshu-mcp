package account

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateAccountID(t *testing.T) {
	valid := []string{"abc", "default", "xhs_brand_01", "a1234567890123456789012345678901"}
	for _, id := range valid {
		if err := ValidateAccountID(id); err != nil {
			t.Fatalf("ValidateAccountID(%q): %v", id, err)
		}
	}
	invalid := []string{"", "ab", "Aaa", " abc", "abc ", "中号", "../abc", "abc-def", "accounts", "system", "root", "null", "unknown", "a12345678901234567890123456789012"}
	for _, id := range invalid {
		if ErrorCode(ValidateAccountID(id)) != CodeInvalidAccountID {
			t.Errorf("ValidateAccountID(%q) code = %q", id, ErrorCode(ValidateAccountID(id)))
		}
	}
}

func TestCookieStoreIsolationAtomicWriteAndDelete(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileCookieStore(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Save(ctx, "acct_a", []byte(`[{"name":"a"}]`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, "acct_b", []byte(`[{"name":"b"}]`)); err != nil {
		t.Fatal(err)
	}
	a, err := store.Load(ctx, "acct_a")
	if err != nil || string(a) != `[{"name":"a"}]` {
		t.Fatalf("load A = %s, %v", a, err)
	}
	p, err := store.Path("acct_a")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cookie mode = %o", info.Mode().Perm())
	}
	if err := store.Save(ctx, "acct_a", []byte(`not-json`)); ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("invalid JSON code = %q", ErrorCode(err))
	}
	a, _ = store.Load(ctx, "acct_a")
	if string(a) != `[{"name":"a"}]` {
		t.Fatalf("old cookie changed: %s", a)
	}
	if err := store.Delete(ctx, "acct_a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "acct_a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx, "acct_a"); ErrorCode(err) != CodeCookieNotFound {
		t.Fatalf("missing code = %q", ErrorCode(err))
	}
	if _, err := store.Path("../acct_b"); ErrorCode(err) != CodeInvalidAccountID {
		t.Fatalf("escape code = %q", ErrorCode(err))
	}
}

func TestRegistryCreateResolvePersistAndStrictLoad(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 13, 1, 2, 3, 0, time.UTC)
	r, err := NewFileRegistry(root, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := r.Create(ctx, CreateAccountInput{ID: "acct_b", DisplayName: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != StatusNeedsLogin || !created.CreatedAt.Equal(now) {
		t.Fatalf("created = %#v", created)
	}
	if _, err := r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefault(ctx, "acct_b"); err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve(ctx, "")
	if err != nil || resolved.Account.ID != "acct_b" || resolved.SelectionSource != SelectionDefault {
		t.Fatalf("resolve = %#v, %v", resolved, err)
	}
	resolved, err = r.Resolve(ctx, "acct_a")
	if err != nil || resolved.SelectionSource != SelectionExplicit {
		t.Fatalf("explicit = %#v, %v", resolved, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || string(data) == "{}" {
		t.Fatal("registry not persisted")
	}
	if filepath.Base(filepath.Dir(filepath.Join(root, "accounts.json"))) == "accounts" {
		t.Fatal("bad registry path")
	}
	var doc registryDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Accounts[0].ID != "acct_a" || doc.Accounts[1].ID != "acct_b" {
		t.Fatalf("not sorted: %#v", doc.Accounts)
	}
	info, _ := os.Stat(filepath.Join(root, "accounts.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatalf("registry mode = %o", info.Mode().Perm())
	}

	badRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(badRoot, "accounts.json"), []byte(`{"schema_version":1,"accounts":[],"typo":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileRegistry(badRoot); ErrorCode(err) != CodeRegistryCorrupt {
		t.Fatalf("unknown field code = %q", ErrorCode(err))
	}
}

func TestRegistryResolutionRequiresUnambiguousAccount(t *testing.T) {
	r, err := NewFileRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := r.Resolve(ctx, ""); ErrorCode(err) != CodeAccountRequired {
		t.Fatalf("empty code = %q", ErrorCode(err))
	}
	if _, err := r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve(ctx, "")
	if err != nil || resolved.SelectionSource != SelectionSingleAvailable {
		t.Fatalf("single = %#v, %v", resolved, err)
	}
	if _, err := r.Create(ctx, CreateAccountInput{ID: "acct_b", DisplayName: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve(ctx, ""); ErrorCode(err) != CodeAccountRequired {
		t.Fatalf("ambiguous code = %q", ErrorCode(err))
	}
	if _, err := r.Resolve(ctx, "missing"); ErrorCode(err) != CodeAccountNotFound {
		t.Fatalf("missing code = %q", ErrorCode(err))
	}
}

func TestRegistryStatusTransitionsAndDefaultProtection(t *testing.T) {
	r, _ := NewFileRegistry(t.TempDir())
	ctx := context.Background()
	_, _ = r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	if err := r.SetDefault(ctx, "acct_a"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateStatus(ctx, "acct_a", StatusActive, "logged in"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateStatus(ctx, "acct_a", StatusRiskHold, "challenge"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateStatus(ctx, "acct_a", StatusActive, ""); err == nil {
		t.Fatal("risk hold released without reason")
	}
	if err := r.UpdateStatus(ctx, "acct_a", StatusDisabled, "retired"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve(ctx, ""); ErrorCode(err) != CodeAccountRequired {
		t.Fatalf("disabled default should not resolve: %v", err)
	}
}

func TestLockManagerSerializesSameAccountAndLimitsGlobalConcurrency(t *testing.T) {
	lm, err := NewLockManager(2)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	releaseA, err := lm.Acquire(ctx, "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	acquiredA := make(chan func(), 1)
	go func() {
		release, e := lm.Acquire(ctx, "acct_a")
		if e == nil {
			acquiredA <- release
		}
	}()
	select {
	case <-acquiredA:
		t.Fatal("same account acquired concurrently")
	case <-time.After(30 * time.Millisecond):
	}
	releaseA()
	select {
	case release := <-acquiredA:
		release()
	case <-time.After(time.Second):
		t.Fatal("waiter not released")
	}
	releaseA, err = lm.Acquire(ctx, "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseA()
	releaseB, err := lm.Acquire(ctx, "acct_b")
	if err != nil {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := lm.Acquire(cancelCtx, "acct_c"); ErrorCode(err) != CodeAccountBusy {
		t.Fatalf("timeout code = %q", ErrorCode(err))
	}
	releaseB()
}

func TestLockManagerDifferentAccountsRunInParallel(t *testing.T) {
	lm, _ := NewLockManager(2)
	var current, maximum int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, id := range []string{"acct_a", "acct_b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			release, err := lm.Acquire(context.Background(), id)
			if err != nil {
				t.Error(err)
				return
			}
			defer release()
			n := atomic.AddInt32(&current, 1)
			for {
				old := atomic.LoadInt32(&maximum)
				if n <= old || atomic.CompareAndSwapInt32(&maximum, old, n) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			atomic.AddInt32(&current, -1)
		}(id)
	}
	close(start)
	wg.Wait()
	if maximum != 2 {
		t.Fatalf("maximum concurrency = %d", maximum)
	}
}

type fakeBrowser struct{ closed bool }
type fakeFactory struct {
	made    int
	browser *fakeBrowser
	err     error
}

func (f *fakeFactory) New(context.Context, Account) (Browser, error) {
	f.made++
	if f.err != nil {
		return nil, f.err
	}
	f.browser = &fakeBrowser{}
	return f.browser, nil
}
func (b *fakeBrowser) Close() { b.closed = true }

func TestAccountManagerGatesStatusRechecksAndClosesBrowser(t *testing.T) {
	r, _ := NewFileRegistry(t.TempDir())
	ctx := context.Background()
	_, _ = r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "A"})
	lm, _ := NewLockManager(1)
	factory := &fakeFactory{}
	m := NewAccountManager(r, lm, factory)
	if _, err := m.WithAccount(ctx, "acct_a", OperationWrite, func(context.Context, Account, Browser) error { return nil }); ErrorCode(err) != CodeAccountLoginRequired {
		t.Fatalf("gate code = %q", ErrorCode(err))
	}
	if factory.made != 0 {
		t.Fatal("browser created before gate")
	}
	_ = r.UpdateStatus(ctx, "acct_a", StatusActive, "login")
	want := errors.New("operation failed")
	resolved, err := m.WithAccount(ctx, "acct_a", OperationRead, func(_ context.Context, got Account, _ Browser) error {
		if got.ID != "acct_a" {
			t.Errorf("id = %s", got.ID)
		}
		return want
	})
	if !errors.Is(err, want) || resolved.Account.ID != "acct_a" {
		t.Fatalf("result = %#v, %v", resolved, err)
	}
	if factory.browser == nil || !factory.browser.closed {
		t.Fatal("browser not closed")
	}
}

func TestMigrateLegacyCookie(t *testing.T) {
	root := t.TempDir()
	legacyDir := t.TempDir()
	legacy := filepath.Join(legacyDir, "cookies.json")
	data := []byte(`[{"name":"legacy"}]`)
	if err := os.WriteFile(legacy, data, 0644); err != nil {
		t.Fatal(err)
	}
	result, err := MigrateLegacy(context.Background(), MigrationOptions{Root: root, Candidates: []string{legacy}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Migrated || result.AccountID != "default" {
		t.Fatalf("result = %#v", result)
	}
	stored, err := os.ReadFile(filepath.Join(root, "accounts", "default", "cookies.json"))
	if err != nil || string(stored) != string(data) {
		t.Fatalf("stored = %s, %v", stored, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("legacy source removed")
	}
	r, err := NewFileRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve(context.Background(), "")
	if err != nil || resolved.Account.ID != "default" {
		t.Fatalf("resolved = %#v, %v", resolved, err)
	}
	result, err = MigrateLegacy(context.Background(), MigrationOptions{Root: root, Candidates: []string{legacy}})
	if err != nil || result.Migrated {
		t.Fatalf("second migration = %#v, %v", result, err)
	}
}

func TestMigrateLegacyRejectsConflictingSourcesWithoutRegistry(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(t.TempDir(), "cookies.json")
	two := filepath.Join(t.TempDir(), "cookies.json")
	_ = os.WriteFile(one, []byte(`[{"name":"one"}]`), 0600)
	_ = os.WriteFile(two, []byte(`[{"name":"two"}]`), 0600)
	if _, err := MigrateLegacy(context.Background(), MigrationOptions{Root: root, Candidates: []string{one, two}}); ErrorCode(err) != CodeLegacyCookieAmbiguous {
		t.Fatalf("code = %q", ErrorCode(err))
	}
	if _, err := os.Stat(filepath.Join(root, "accounts.json")); !os.IsNotExist(err) {
		t.Fatal("registry persisted after conflict")
	}
}
