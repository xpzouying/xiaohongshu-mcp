package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// driverCacheDirName 是 playwright-go 在默认缓存目录下寻找驱动时使用的子目录名。
// 与 playwright-go run.go 中 getDefaultCacheDirectory + "ms-playwright-go"/<version> 对应。
const driverCacheDirName = "ms-playwright-go"

// ensureDriverEnv 解析 playwright 驱动（node + playwright-core）位置并注入环境变量。
//
// 绝不调用 playwright.Install()：那会从 CDN 下载驱动与浏览器，违反
// 「运行时不得自动下载未审计组件」的约束。驱动必须由 cmd/camoufox-setup
// 预先放置，或由环境变量显式指定。解析失败即返回错误（fail closed）。
func ensureDriverEnv() error {
	// 已显式配置则尊重调用方
	if os.Getenv("PLAYWRIGHT_DRIVER_PATH") != "" {
		return ensureNode()
	}

	if dir := findDriverDir(); dir != "" {
		os.Setenv("PLAYWRIGHT_DRIVER_PATH", dir)
		return ensureNode()
	}

	return fmt.Errorf(
		"playwright driver not found\n" +
			"  run the installer to fetch node + pinned playwright-core:\n" +
			"    go run ./cmd/camoufox-setup\n" +
			"  or set PLAYWRIGHT_DRIVER_PATH to an existing driver directory\n" +
			"  (this project never downloads the driver at runtime)",
	)
}

// ensureNode 确认有可用 node 可执行文件。优先尊重已设的 PLAYWRIGHT_NODEJS_PATH；
// 否则依次看驱动目录自带 node、PATH 中的 node，找到则显式注入，避免
// playwright-go 再去触发下载。
func ensureNode() error {
	if p := os.Getenv("PLAYWRIGHT_NODEJS_PATH"); p != "" {
		if mustExecutableFile(p) == nil {
			return nil
		}
		return fmt.Errorf("PLAYWRIGHT_NODEJS_PATH not executable: %s", p)
	}

	if dir := os.Getenv("PLAYWRIGHT_DRIVER_PATH"); dir != "" {
		node := filepath.Join(dir, nodeExeName())
		if mustExecutableFile(node) == nil {
			os.Setenv("PLAYWRIGHT_NODEJS_PATH", node)
			return nil
		}
	}

	if p, err := lookPathNode(); err == nil {
		os.Setenv("PLAYWRIGHT_NODEJS_PATH", p)
		return nil
	}

	return fmt.Errorf(
		"node runtime not found\n" +
			"  install node, or set PLAYWRIGHT_NODEJS_PATH to a node executable\n" +
			"  (cmd/camoufox-setup can stage a node binary into the driver directory)",
	)
}

// findDriverDir 在仓库内与缓存目录中定位已就位的 playwright 驱动目录。
func findDriverDir() string {
	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, ".playwright-driver"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".playwright-driver"))
	}
	if cache, err := os.UserCacheDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(cache, driverCacheDirName, driverVersion()),
			filepath.Join(cache, driverCacheDirName),
		)
	}

	for _, dir := range candidates {
		if isDriverDir(dir) {
			return dir
		}
	}
	return ""
}

// isDriverDir 判断目录是否为可用的 playwright 驱动（含 package/cli.js）。
func isDriverDir(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "package", "cli.js"))
	return err == nil && !st.IsDir()
}

// driverVersion 返回与当前 playwright-go 依赖绑定的驱动版本。
// 与 go.mod 中 playwright-go v0.5200.1（Playwright 1.52.0）保持一致。
func driverVersion() string {
	return "1.52.0"
}

func nodeExeName() string {
	if runtime.GOOS == "windows" {
		return "node.exe"
	}
	return "node"
}

var lookPathNode = func() (string, error) {
	return lookPath(nodeExeName())
}
