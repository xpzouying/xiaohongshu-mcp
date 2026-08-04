package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsLikelyStockChrome(t *testing.T) {
	if !IsLikelyStockChrome("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome") {
		t.Fatal("expected stock chrome")
	}
	if IsLikelyStockChrome(filepath.Join(os.TempDir(), "xiaohongshu-mcp/browser/148/Chromium.app/Contents/MacOS/Chromium")) {
		t.Fatal("cache chromium should not count as stock")
	}
}

func TestResolveBrowserBin_envOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip path mode bits on windows")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-chrome")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XHS_BROWSER_BIN", fake)
	t.Setenv("ROD_BROWSER_BIN", "")

	p, src, err := ResolveBrowserBin()
	if err != nil {
		t.Fatal(err)
	}
	if p != fake {
		t.Fatalf("path=%s", p)
	}
	if src != "env:XHS_BROWSER_BIN" {
		t.Fatalf("source=%s", src)
	}
}
