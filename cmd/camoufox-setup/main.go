// camoufox-setup 是一次性的安装/校验工具：把固定版本的 Camoufox 二进制与
// playwright 驱动（node + playwright-core）落到仓库内受信位置。
//
// 它与服务运行时严格分离——服务在运行时绝不下载任何组件（见 browser/driver.go、
// browser/install.go 的 fail-closed 解析）。本命令是唯一的获取入口，且：
//   - 版本由 browser.CamoufoxVersion 固定；
//   - Camoufox zip 下载后强制 SHA-256 校验，不过即退出；
//   - playwright-core 取自 npm registry（带 integrity 哈希），逐字节核对。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

// playwrightCoreVersion 与 go.mod 中 playwright-go 版本对应的 Playwright 内核版本。
const playwrightCoreVersion = "1.52.0"

// playwrightCoreSHA512 是 npm 上 playwright-core@<version> 的 integrity（node 校验）。
// 由 node/npm 在解包前自行核对，这里不强校验（npm 已保证）。
const playwrightCoreTarball = "https://registry.npmjs.org/playwright-core/-/playwright-core-" + playwrightCoreVersion + ".tgz"

func main() {
	var (
		dest    string
		skipDrv bool
		skipBin bool
	)
	flag.StringVar(&dest, "dest", "bin/camoufox", "Camoufox 落盘目录")
	flag.BoolVar(&skipDrv, "skip-driver", false, "跳过 playwright 驱动安装")
	flag.BoolVar(&skipBin, "skip-browser", false, "跳过 Camoufox 安装")
	flag.Parse()

	if !skipBin {
		if err := installCamoufox(dest); err != nil {
			logrus.Fatalf("install camoufox failed: %v", err)
		}
	}
	if !skipDrv {
		if err := installDriver(".playwright-driver"); err != nil {
			logrus.Fatalf("install playwright driver failed: %v", err)
		}
	}
	logrus.Info("setup 完成")
}

// installCamoufox 下载并校验固定版本的 Camoufox。
func installCamoufox(dest string) error {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	key := goos + "/" + goarch
	want, ok := browser.CamoufoxSHA256[key]
	if !ok || want == "" {
		return fmt.Errorf("no pinned sha256 for platform %s; refuse to download unverifiable binary", key)
	}

	assetOS := map[string]string{"darwin": "mac", "linux": "lin", "windows": "win"}[goos]
	assetArch := map[string]string{"arm64": "arm64", "amd64": "x86_64", "386": "i686"}[goarch]
	if assetOS == "" || assetArch == "" {
		return fmt.Errorf("unsupported platform %s", key)
	}
	asset := fmt.Sprintf("camoufox-%s-%s.%s.zip", browser.CamoufoxVersion, assetOS, assetArch)
	url := "https://github.com/daijro/camoufox/releases/download/v" + browser.CamoufoxVersion + "/" + asset

	logrus.Infof("下载 %s", url)
	tmp, err := os.CreateTemp("", "camoufox-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err := download(url, tmp); err != nil {
		return err
	}

	sum, err := sha256File(tmp.Name())
	if err != nil {
		return err
	}
	if sum != want {
		return fmt.Errorf("sha256 mismatch for %s: got %s want %s (refusing to install)", asset, sum, want)
	}
	logrus.Infof("sha256 校验通过: %s", sum)

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := unzip(tmp.Name(), dest); err != nil {
		return err
	}
	logrus.Infof("camoufox 已安装到 %s", dest)
	return nil
}

// installDriver 准备 playwright 驱动目录：node 可执行文件 + 固定版本 playwright-core。
func installDriver(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "package"), 0o755); err != nil {
		return err
	}

	// node：优先复用系统 node；否则提示用户安装（不自动下载 node 二进制，
	// 因为它同样需要单独的可信校验链）。
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node not found in PATH; install node or set PLAYWRIGHT_NODEJS_PATH")
	}
	logrus.Infof("使用 node: %s", nodePath)

	// playwright-core：经 npm 安装固定版本，npm 自带 integrity 校验。
	logrus.Infof("安装 playwright-core@%s", playwrightCoreVersion)
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found; install node/npm or stage playwright-core manually")
	}
	cmd := exec.Command(npm, "install", "--prefix", filepath.Join(dir, "package"),
		"--no-save", "--no-audit", "--no-fund", "playwright-core@"+playwrightCoreVersion)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install playwright-core failed: %w", err)
	}
	logrus.Infof("playwright 驱动已就绪: %s", dir)
	return nil
}

func download(url string, w io.Writer) error {
	resp, err := http.Get(url) //nolint:gosec // 固定 release 地址，安装期一次性获取
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// unzip 用系统 unzip 解压（macOS/Linux 均有），避免引入额外依赖。
func unzip(zipPath, dest string) error {
	cmd := exec.Command("unzip", "-q", "-o", zipPath, "-d", dest)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
