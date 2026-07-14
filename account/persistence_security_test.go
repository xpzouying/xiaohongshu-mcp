package account

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCookieStoreDirFDDoesNotFollowReplacedAncestor(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileCookieStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), "acct_safe", []byte(`[]`)); err != nil {
		t.Fatal(err)
	}

	accounts := filepath.Join(root, "accounts")
	moved := filepath.Join(root, "accounts-old")
	outside := t.TempDir()
	if err := os.Rename(accounts, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, accounts); err != nil {
		t.Fatal(err)
	}

	if err := store.Save(context.Background(), "acct_safe", []byte(`[{"name":"new"}]`)); ErrorCode(err) != CodePersistenceFailed {
		t.Fatalf("save code=%q err=%v", ErrorCode(err), err)
	}
	if _, err := os.Stat(filepath.Join(outside, "acct_safe", "cookies.json")); !os.IsNotExist(err) {
		t.Fatalf("outside path was touched: %v", err)
	}
}
