package account

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerRemoveRetriesStagedCookieAfterCrash(t *testing.T) {
	root := t.TempDir()
	registry, _ := NewFileRegistry(root)
	if _, err := registry.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	if err := store.Save(context.Background(), "acct_a", []byte(`[{"name":"session"}]`)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StageRemove(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}

	restarted, _ := NewFileCookieStore(root)
	locks, _ := NewLockManager(1)
	if err := NewManagementManager(registry, locks, restarted).Remove(context.Background(), "acct_a"); err != nil {
		t.Fatalf("retry remove: %v", err)
	}
	if _, err := registry.Get(context.Background(), "acct_a"); ErrorCode(err) != CodeAccountNotFound {
		t.Fatalf("account code=%q err=%v", ErrorCode(err), err)
	}
	path, _ := restarted.Path("acct_a")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cookie still exists: %v", err)
	}
	if _, err := os.Stat(path + ".removing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged cookie still exists: %v", err)
	}
}

func TestManagerRemoveCleansStagedCookieAfterRegistryCrash(t *testing.T) {
	root := t.TempDir()
	registry, _ := NewFileRegistry(root)
	if _, err := registry.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	if err := store.Save(context.Background(), "acct_a", []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	removal, err := store.StageRemove(context.Background(), "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	if err := removal.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := registry.Remove(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}

	locks, _ := NewLockManager(1)
	err = NewManagementManager(registry, locks, store).Remove(context.Background(), "acct_a")
	if ErrorCode(err) != CodeAccountNotFound {
		t.Fatalf("remove code=%q err=%v", ErrorCode(err), err)
	}
	path, _ := store.Path("acct_a")
	if _, err := os.Stat(path + ".removing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged cookie still exists: %v", err)
	}
}

func TestCookieStoreStageRemoveRejectsExistingStagedConflict(t *testing.T) {
	store, _ := NewFileCookieStore(t.TempDir())
	want := []byte(`[{"name":"current"}]`)
	if err := store.Save(context.Background(), "acct_a", want); err != nil {
		t.Fatal(err)
	}
	path, _ := store.Path("acct_a")
	staged := []byte(`[{"name":"stale"}]`)
	if err := os.WriteFile(path+".removing", staged, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := store.StageRemove(context.Background(), "acct_a"); ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("stage conflict code=%q err=%v", ErrorCode(err), err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Fatalf("current cookie=%s err=%v", got, err)
	}
	got, err = os.ReadFile(path + ".removing")
	if err != nil || string(got) != string(staged) {
		t.Fatalf("staged cookie=%s err=%v", got, err)
	}
}

func TestCookieRemovalCompletePropagatesRealRemoveFailure(t *testing.T) {
	root := t.TempDir()
	staged := filepath.Join(root, "cookies.json.removing")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "blocker"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	removal := &fileCookieRemoval{path: filepath.Join(root, "cookies.json"), staged: staged}

	if err := removal.Complete(); ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("complete code=%q err=%v", ErrorCode(err), err)
	}
	if removal.staged != staged {
		t.Fatalf("staged state cleared after failed cleanup: %q", removal.staged)
	}
}

func TestManagerRemovePropagatesCookieCleanupFailure(t *testing.T) {
	root := t.TempDir()
	r, _ := NewFileRegistry(root)
	if _, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	if err := store.Save(context.Background(), "acct_a", []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	cleanupErr := errors.New("cleanup failed")
	locks, _ := NewLockManager(1)
	manager := NewManagementManager(r, locks, &failingRemovalCookieStore{CookieStore: store, completeErr: cleanupErr})

	if err := manager.Remove(context.Background(), "acct_a"); !errors.Is(err, cleanupErr) {
		t.Fatalf("remove err=%v, want %v", err, cleanupErr)
	}
	if _, err := r.Get(context.Background(), "acct_a"); ErrorCode(err) != CodeAccountNotFound {
		t.Fatalf("account code=%q err=%v", ErrorCode(err), err)
	}
}
