package account

import (
	"context"
	"errors"
	"os"
	"testing"
)

type failingRemoveRegistry struct {
	Registry
	removeErr   error
	removeCalls int
}

func (r *failingRemoveRegistry) Remove(context.Context, string) error {
	r.removeCalls++
	return r.removeErr
}

type failingDeleteCookieStore struct {
	CookieStore
	deleteErr   error
	deleteCalls int
}

func (s *failingDeleteCookieStore) StageRemove(context.Context, string) (CookieRemoval, error) {
	s.deleteCalls++
	return nil, s.deleteErr
}

type failingCookieRemoval struct {
	CookieRemoval
	commitErr   error
	rollbackErr error
	completeErr error
}

func (r *failingCookieRemoval) Commit(ctx context.Context) error {
	if r.commitErr != nil {
		return r.commitErr
	}
	return r.CookieRemoval.Commit(ctx)
}

func (r *failingCookieRemoval) Rollback(ctx context.Context) error {
	err := r.CookieRemoval.Rollback(ctx)
	return errors.Join(err, r.rollbackErr)
}

func (r *failingCookieRemoval) Complete() error {
	if r.completeErr != nil {
		return r.completeErr
	}
	return r.CookieRemoval.Complete()
}

type failingRemovalCookieStore struct {
	CookieStore
	commitErr   error
	rollbackErr error
	completeErr error
}

func (s *failingRemovalCookieStore) StageRemove(ctx context.Context, accountID string) (CookieRemoval, error) {
	removal, err := s.CookieStore.StageRemove(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &failingCookieRemoval{
		CookieRemoval: removal,
		commitErr:     s.commitErr,
		rollbackErr:   s.rollbackErr,
		completeErr:   s.completeErr,
	}, nil
}

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

func TestRegistryCreateReturnsStableCallerErrorCodes(t *testing.T) {
	r, err := NewFileRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := r.Create(ctx, CreateAccountInput{ID: "acct_a"}); ErrorCode(err) != CodeInvalidDisplayName {
		t.Fatalf("empty display name code=%q err=%v", ErrorCode(err), err)
	}
	if _, err := r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(ctx, CreateAccountInput{ID: "acct_a", DisplayName: "Again"}); ErrorCode(err) != CodeAccountAlreadyExists {
		t.Fatalf("duplicate account code=%q err=%v", ErrorCode(err), err)
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

func TestManagerRemoveRestoresCookieWhenRegistryRemoveFails(t *testing.T) {
	root := t.TempDir()
	r, _ := NewFileRegistry(root)
	if _, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	cookie := []byte(`[{"name":"session"}]`)
	if err := store.Save(context.Background(), "acct_a", cookie); err != nil {
		t.Fatal(err)
	}
	locks, _ := NewLockManager(1)
	wantErr := errors.New("registry remove failed")
	failingRegistry := &failingRemoveRegistry{Registry: r, removeErr: wantErr}
	manager := NewManagementManager(failingRegistry, locks, store)

	if err := manager.Remove(context.Background(), "acct_a"); !errors.Is(err, wantErr) {
		t.Fatalf("remove err=%v, want %v", err, wantErr)
	}
	if failingRegistry.removeCalls != 1 {
		t.Fatalf("registry remove calls=%d", failingRegistry.removeCalls)
	}
	if _, err := r.Get(context.Background(), "acct_a"); err != nil {
		t.Fatalf("account was partially removed: %v", err)
	}
	got, err := store.Load(context.Background(), "acct_a")
	if err != nil {
		t.Fatalf("cookie was not restored: %v", err)
	}
	if string(got) != string(cookie) {
		t.Fatalf("restored cookie=%s, want %s", got, cookie)
	}
}

func TestManagerRemoveDoesNotTouchRegistryWhenCookieDeleteFails(t *testing.T) {
	root := t.TempDir()
	r, _ := NewFileRegistry(root)
	if _, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	cookie := []byte(`[{"name":"session"}]`)
	if err := store.Save(context.Background(), "acct_a", cookie); err != nil {
		t.Fatal(err)
	}
	locks, _ := NewLockManager(1)
	wantErr := errors.New("cookie delete failed")
	failingStore := &failingDeleteCookieStore{CookieStore: store, deleteErr: wantErr}
	trackingRegistry := &failingRemoveRegistry{Registry: r}
	manager := NewManagementManager(trackingRegistry, locks, failingStore)

	if err := manager.Remove(context.Background(), "acct_a"); !errors.Is(err, wantErr) {
		t.Fatalf("remove err=%v, want %v", err, wantErr)
	}
	if failingStore.deleteCalls != 1 {
		t.Fatalf("cookie delete calls=%d", failingStore.deleteCalls)
	}
	if trackingRegistry.removeCalls != 0 {
		t.Fatalf("registry remove calls=%d", trackingRegistry.removeCalls)
	}
	if _, err := r.Get(context.Background(), "acct_a"); err != nil {
		t.Fatalf("account was partially removed: %v", err)
	}
	got, err := store.Load(context.Background(), "acct_a")
	if err != nil || string(got) != string(cookie) {
		t.Fatalf("cookie changed: data=%s err=%v", got, err)
	}
}

func TestManagerRemoveReportsRealRollbackFailureAndAllowsRecovery(t *testing.T) {
	root := t.TempDir()
	r, _ := NewFileRegistry(root)
	wantAccount, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A", Owner: "owner", Purpose: "publish"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefault(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	wantCookie := []byte(`[{"name":"session","value":"original"}]`)
	if err := store.Save(context.Background(), "acct_a", wantCookie); err != nil {
		t.Fatal(err)
	}
	locks, _ := NewLockManager(1)
	removeErr := errors.New("registry remove failed")
	cookiePath, err := store.Path("acct_a")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), "blocker", []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	blockerPath, err := store.Path("blocker")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManagementManager(&registryRemoveHook{
		Registry: r,
		err:      removeErr,
		hook: func() error {
			return moveDirectory(blockerPath, cookiePath)
		},
	}, locks, store)

	err = manager.Remove(context.Background(), "acct_a")
	if !errors.Is(err, removeErr) || ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("remove err=%v, want registry and real rollback errors", err)
	}
	if _, statErr := os.Stat(cookiePath + ".removing"); statErr != nil {
		t.Fatalf("staged cookie lost after rollback failure: %v", statErr)
	}
	gotCookie, loadErr := store.Load(context.Background(), "acct_a")
	if loadErr != nil || string(gotCookie) != string(wantCookie) {
		t.Fatalf("cookie unavailable after rollback failure: data=%s err=%v", gotCookie, loadErr)
	}
	if err := os.Remove(cookiePath); err != nil {
		t.Fatal(err)
	}
	removal := &fileCookieRemoval{path: cookiePath, staged: cookiePath + ".removing"}
	if err := removal.Rollback(context.Background()); err != nil {
		t.Fatalf("retry rollback: %v", err)
	}
	assertOriginalAccountState(t, r, store, wantAccount, wantCookie)
}

type registryRemoveHook struct {
	Registry
	err  error
	hook func() error
}

func (r *registryRemoveHook) Remove(context.Context, string) error {
	return errors.Join(r.err, r.hook())
}

func moveDirectory(from, to string) error {
	if err := os.Remove(from); err != nil {
		return err
	}
	if err := os.Mkdir(to, 0o700); err != nil {
		return err
	}
	return nil
}

func TestManagerRemoveKeepsDefaultAccountAndCookieWhenCookieCommitFails(t *testing.T) {
	root := t.TempDir()
	r, _ := NewFileRegistry(root)
	wantAccount, err := r.Create(context.Background(), CreateAccountInput{ID: "acct_a", DisplayName: "A", Owner: "owner", Purpose: "publish"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefault(context.Background(), "acct_a"); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileCookieStore(root)
	wantCookie := []byte(`[{"name":"session","value":"original"}]`)
	if err := store.Save(context.Background(), "acct_a", wantCookie); err != nil {
		t.Fatal(err)
	}
	locks, _ := NewLockManager(1)
	commitErr := errors.New("cookie commit failed")
	trackingRegistry := &failingRemoveRegistry{Registry: r}
	manager := NewManagementManager(
		trackingRegistry,
		locks,
		&failingRemovalCookieStore{CookieStore: store, commitErr: commitErr},
	)

	if err := manager.Remove(context.Background(), "acct_a"); !errors.Is(err, commitErr) {
		t.Fatalf("remove err=%v, want %v", err, commitErr)
	}
	if trackingRegistry.removeCalls != 0 {
		t.Fatalf("registry remove calls=%d", trackingRegistry.removeCalls)
	}
	assertOriginalAccountState(t, r, store, wantAccount, wantCookie)
}

func assertOriginalAccountState(t *testing.T, registry Registry, store CookieStore, wantAccount Account, wantCookie []byte) {
	t.Helper()
	gotAccount, err := registry.Get(context.Background(), wantAccount.ID)
	if err != nil || gotAccount != wantAccount {
		t.Fatalf("account=%+v err=%v, want %+v", gotAccount, err, wantAccount)
	}
	resolved, err := registry.Resolve(context.Background(), "")
	if err != nil || resolved.Account != wantAccount || resolved.SelectionSource != SelectionDefault {
		t.Fatalf("default=%+v err=%v, want account=%+v source=%q", resolved, err, wantAccount, SelectionDefault)
	}
	gotCookie, err := store.Load(context.Background(), wantAccount.ID)
	if err != nil || string(gotCookie) != string(wantCookie) {
		t.Fatalf("cookie=%s err=%v, want %s", gotCookie, err, wantCookie)
	}
}
