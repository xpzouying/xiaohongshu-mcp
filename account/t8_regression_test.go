package account

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockManagerWaiterDoesNotHoldGlobalSlot(t *testing.T) {
	lm, _ := NewLockManager(2)
	releaseA, err := lm.Acquire(context.Background(), "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseA()
	waitCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		release, acquireErr := lm.Acquire(waitCtx, "acct_a")
		if acquireErr == nil {
			release()
		}
	}()
	time.Sleep(20 * time.Millisecond)
	releaseB, err := lm.Acquire(context.Background(), "acct_b")
	if err != nil {
		t.Fatalf("different account blocked by waiter: %v", err)
	}
	releaseB()
}

func TestRegistryRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{"schema_version":1,"default_account_id":null,"accounts":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "accounts.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileRegistry(root); ErrorCode(err) != CodeRegistryCorrupt {
		t.Fatalf("code=%q err=%v", ErrorCode(err), err)
	}
}
