package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveBrowserBin 解析本进程应使用的浏览器可执行文件。
//
// 优先级（本项目不负责下载浏览器）：
//  1. XHS_BROWSER_BIN
//  2. ROD_BROWSER_BIN
//  3. 本机系统 Google Chrome / Chromium 常见路径
//
// 浏览器必须由用户或操作系统包管理器自行安装，并通过路径或系统标准位置提供。
func ResolveBrowserBin() (path string, source string, err error) {
	if p := strings.TrimSpace(os.Getenv("XHS_BROWSER_BIN")); p != "" {
		if err := mustExecutable(p); err != nil {
			return "", "", fmt.Errorf("XHS_BROWSER_BIN: %w", err)
		}
		return p, "env:XHS_BROWSER_BIN", nil
	}
	if p := strings.TrimSpace(os.Getenv("ROD_BROWSER_BIN")); p != "" {
		if err := mustExecutable(p); err != nil {
			return "", "", fmt.Errorf("ROD_BROWSER_BIN: %w", err)
		}
		return p, "env:ROD_BROWSER_BIN", nil
	}

	if p := findSystemChrome(); p != "" {
		return p, "system-chrome", nil
	}

	return "", "", fmt.Errorf(
		"no trusted browser binary found\n" +
			"  set XHS_BROWSER_BIN to a local Chrome/Chromium (or CloakBrowser you installed yourself), e.g.\n" +
			"    export XHS_BROWSER_BIN=\"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\"\n" +
			"  or install Google Chrome on this machine\n" +
			"  (this project does not download browsers automatically)",
	)
}

func mustExecutable(p string) error {
	st, err := os.Stat(p)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("path is a directory: %s", p)
	}
	// On Unix, check any execute bit; on Windows Stat is enough for existence.
	if runtime.GOOS != "windows" && st.Mode()&0o111 == 0 {
		return fmt.Errorf("not executable: %s", p)
	}
	return nil
}

func findSystemChrome() string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
		// Homebrew cask sometimes lands under user Applications
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates,
				filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
			)
		}
	case "linux":
		candidates = []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
	case "windows":
		// Best-effort; prefer env override on Windows.
		pf := os.Getenv("PROGRAMFILES")
		pf86 := os.Getenv("PROGRAMFILES(X86)")
		if pf != "" {
			candidates = append(candidates, filepath.Join(pf, "Google/Chrome/Application/chrome.exe"))
		}
		if pf86 != "" {
			candidates = append(candidates, filepath.Join(pf86, "Google/Chrome/Application/chrome.exe"))
		}
	}
	for _, c := range candidates {
		if mustExecutable(c) == nil {
			return c
		}
	}
	return ""
}

// IsLikelyStockChrome reports whether path looks like Google Chrome / stock Chromium
// (not a fingerprint-patched build). Used to disable Cloak-only launch flags.
func IsLikelyStockChrome(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "chrome") || base == "chromium" || base == "chromium.exe" {
		// Cloak extracts often still named Chromium; treat cache path as non-stock.
		if strings.Contains(path, "xiaohongshu-mcp/browser") || strings.Contains(path, ".cloakbrowser") {
			return false
		}
		return true
	}
	return false
}
