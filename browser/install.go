// Package browser 负责 Camoufox 二进制的定位、版本与完整性校验，以及经
// playwright-go（Juggler 协议）驱动常驻 Camoufox 实例。
//
// 供应链约束（与项目安全基线一致）：
//   - 浏览器必须由调用方预先放置在受信位置，本项目在运行时不下载任何二进制；
//   - 版本固定、可对 zip 做 SHA-256 校验；取二进制一律走 ResolveCamoufoxBin，
//     失败即拒绝启动（fail closed），不回退到系统 Chrome / 自动下载。
package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CamoufoxVersion 是当前固定的 Camoufox 版本（对应其 release tag 与 zip 文件名）。
// 升级时需同步更新 CamoufoxSHA256 中对应平台的校验值。
const CamoufoxVersion = "152.0.4-beta.28"

// CamoufoxSHA256 记录各平台官方 release zip 的 SHA-256（取自 GitHub release
// 的 per-asset digest，与本地实测一致），cmd/camoufox-setup 下载后据此校验，
// 校验不过即拒绝落盘（可校验原则，绝不放行来路不明的二进制）。
var CamoufoxSHA256 = map[string]string{
	"darwin/arm64":  "8b7680a61818245cf4eb0150edab811d4ed937b6723514569e74c3e7df9685bd",
	"darwin/amd64":  "6e0efd66f6db46bece072827cb2c5a4851bca822ff4fed95b8a0c2fb6f61a3d4",
	"linux/arm64":   "3a105a2fc929e80a79b4b7fce2c93ed62c4fb2c877f3c1ed2a5d66a1c4fe968f",
	"linux/amd64":   "924f3109ccd6d47cd6a0384d67a345fadf975d48b6319f8dbbd5954c588982bd",
	"windows/amd64": "386fc2f41139685f9a1a9cef0d024bc041d899c315ea538d561171b5b282e57d",
	"windows/386":   "a713aca14f4ef0429eab502bacf42548fe6202bb4e29da1ca64761ed08d409af",
}

// ResolveCamoufoxBin 解析本进程应使用的 Camoufox 可执行文件。
//
// 优先级（本项目不负责下载浏览器）：
//  1. XHS_CAMOUFOX_BIN（可执行文件或其所在目录 / .app 包）
//  2. 仓库内 bin/camoufox（cmd/camoufox-setup 的默认落点）
//  3. 用户缓存目录（camoufox 官方约定路径）
//
// 找不到可用二进制时返回带操作指引的错误，绝不自动下载。
func ResolveCamoufoxBin() (path string, source string, err error) {
	if p := strings.TrimSpace(os.Getenv("XHS_CAMOUFOX_BIN")); p != "" {
		bin, err := normalizeCamoufoxPath(p)
		if err != nil {
			return "", "", fmt.Errorf("XHS_CAMOUFOX_BIN: %w", err)
		}
		return bin, "env:XHS_CAMOUFOX_BIN", nil
	}

	if p := repoLocalCamoufox(); p != "" {
		return p, "repo:bin/camoufox", nil
	}

	if p := cacheCamoufox(); p != "" {
		return p, "user-cache", nil
	}

	return "", "", fmt.Errorf(
		"no trusted Camoufox binary found\n" +
			"  run the pinned installer to fetch + verify Camoufox " + CamoufoxVersion + ":\n" +
			"    go run ./cmd/camoufox-setup\n" +
			"  or point XHS_CAMOUFOX_BIN at an existing Camoufox binary / .app\n" +
			"  (this project never downloads browsers at runtime)",
	)
}

// normalizeCamoufoxPath 接受可执行文件、.app 包或其上层目录，归一化到可执行文件路径。
func normalizeCamoufoxPath(p string) (string, error) {
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if err := mustExecutableFile(p); err != nil {
			return "", err
		}
		return p, nil
	}
	// 目录：可能是 .app 包或解压根目录，向下找可执行文件
	if bin := camoufoxExeInDir(p); bin != "" {
		return bin, nil
	}
	return "", fmt.Errorf("no Camoufox executable under directory: %s", p)
}

// repoLocalCamoufox 返回仓库内 bin/camoufox 下的可执行文件（若存在）。
// 以可执行文件/工作目录两种方式定位仓库根，兼容 go run 与编译后的二进制。
func repoLocalCamoufox() string {
	roots := []string{}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	for _, root := range roots {
		if bin := camoufoxExeInDir(filepath.Join(root, "bin", "camoufox")); bin != "" {
			return bin
		}
	}
	return ""
}

// cacheCamoufox 返回用户缓存目录下的 Camoufox 可执行文件（camoufox 官方安装约定）。
func cacheCamoufox() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	var dirs []string
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs, filepath.Join(home, "Library", "Caches", "camoufox"))
	case "linux":
		dirs = append(dirs, filepath.Join(home, ".cache", "camoufox"))
	case "windows":
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			dirs = append(dirs, filepath.Join(la, "camoufox"))
		}
	}
	for _, d := range dirs {
		if bin := camoufoxExeInDir(d); bin != "" {
			return bin
		}
	}
	return ""
}

// camoufoxExeInDir 在目录（含 .app 包）内定位 Camoufox 可执行文件。
func camoufoxExeInDir(dir string) string {
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			filepath.Join(dir, "Camoufox.app", "Contents", "MacOS", "camoufox"),
			filepath.Join(dir, "Contents", "MacOS", "camoufox"), // dir 本身就是 .app
			filepath.Join(dir, "camoufox"),
		)
	case "linux":
		candidates = append(candidates,
			filepath.Join(dir, "camoufox-bin"),
			filepath.Join(dir, "camoufox"),
		)
	case "windows":
		candidates = append(candidates,
			filepath.Join(dir, "camoufox.exe"),
		)
	}
	for _, c := range candidates {
		if mustExecutableFile(c) == nil {
			return c
		}
	}
	return ""
}

func mustExecutableFile(p string) error {
	st, err := os.Stat(p)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("path is a directory: %s", p)
	}
	if runtime.GOOS != "windows" && st.Mode()&0o111 == 0 {
		return fmt.Errorf("not executable: %s", p)
	}
	return nil
}
