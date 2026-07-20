package browser

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

// 内置浏览器的下载分发地址。
const browserCDNBase = "https://cdn.one-world.ai/browsers"

// browserVersion 是内置浏览器的唯一版本源。升级只改 browser_version.txt 一处，Go 与 Dockerfile 同读。
//
//go:embed browser_version.txt
var browserVersionRaw string

var browserVersion = strings.TrimSpace(browserVersionRaw)

func browserURL(name string) string {
	return browserCDNBase + "/" + browserVersion + "/" + name
}

// platformAsset 返回当前 OS/arch 对应的下载文件名与解压后二进制文件名。
// 第三个返回值为 false 表示当前平台无预编译二进制。
func platformAsset() (assetName, binName string, ok bool) {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH != "arm64" {
			return "", "", false
		}
		return "macos-arm64.dmg", "Chromium", true
	case "linux":
		if runtime.GOARCH != "amd64" {
			return "", "", false
		}
		return "linux-x64.tar.xz", "chrome", true
	case "windows":
		if runtime.GOARCH != "amd64" {
			return "", "", false
		}
		return "windows-x64.zip", "chrome.exe", true
	}
	return "", "", false
}

func browserCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "xiaohongshu-mcp", "browser", browserVersion), nil
}

// EnsureBrowser 确保本地存在内置浏览器二进制，返回其路径。
// 已缓存则直接返回；否则下载 → 校验 SHA256 → 解压。当前平台无预编译二进制时返回 error。
func EnsureBrowser() (string, error) {
	asset, binName, ok := platformAsset()
	if !ok {
		return "", fmt.Errorf("当前平台 %s/%s 无预编译浏览器，请手动指定 --bin", runtime.GOOS, runtime.GOARCH)
	}

	cacheDir, err := browserCacheDir()
	if err != nil {
		return "", err
	}

	// 已缓存：遍历查找二进制
	if bin := findBinary(cacheDir, binName); bin != "" {
		return bin, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	// 下载（重试 3 次）
	logrus.Infof("首次运行：下载内置浏览器 %s（%s，约 140-190MB，仅一次）...", browserVersion, asset)
	archivePath := filepath.Join(cacheDir, asset)
	var dlErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if dlErr = downloadFile(browserURL(asset), archivePath); dlErr == nil {
			break
		}
		logrus.Warnf("下载失败（第 %d/3 次）: %v", attempt, dlErr)
		_ = os.Remove(archivePath)
		time.Sleep(2 * time.Second)
	}
	if dlErr != nil {
		return "", fmt.Errorf("下载内置浏览器失败: %w\n"+
			"  浏览器是反检测核心，缺它等于裸奔，故不继续。请任选其一：\n"+
			"  1) 检查网络后重试；\n"+
			"  2) 手动下载 %s，解压后用 --bin 指定其中的浏览器二进制。", dlErr, browserURL(asset))
	}
	defer os.Remove(archivePath)

	// 校验 SHA256（本地已成分发点，必须校验完整性）
	if err := verifySHA256(archivePath, asset); err != nil {
		return "", fmt.Errorf("校验失败: %w", err)
	}

	logrus.Infof("解压内置浏览器 ...")
	if err := extractArchive(archivePath, cacheDir); err != nil {
		return "", fmt.Errorf("解压失败: %w", err)
	}

	bin := findBinary(cacheDir, binName)
	if bin == "" {
		return "", fmt.Errorf("解压后未找到二进制 %s", binName)
	}
	logrus.Infof("内置浏览器就绪: %s", bin)
	return bin, nil
}

// verifySHA256 下载同目录的 SHA256SUMS，校验 asset 的哈希。
func verifySHA256(archivePath, asset string) error {
	want, err := fetchExpectedSHA(asset)
	if err != nil {
		return err
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s SHA256 不匹配：期望 %s，实际 %s", asset, want, got)
	}
	return nil
}

func fetchExpectedSHA(asset string) (string, error) {
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(browserURL("SHA256SUMS"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取 SHA256SUMS: HTTP %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		// 格式：<hash>␠␠<filename>
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS 中未找到 %s", asset)
}

func findBinary(dir, binName string) string {
	var found string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == binName {
			found = path
			return io.EOF // 提前结束
		}
		return nil
	})
	if found != "" {
		if err := os.Chmod(found, 0o755); err != nil {
			logrus.Debugf("chmod %s: %v", found, err)
		}
	}
	return found
}

func downloadFile(url, dst string) error {
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractArchive(archivePath, destDir string) error {
	switch {
	case strings.HasSuffix(archivePath, ".tar.xz"):
		return extractTarXz(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".dmg"):
		return extractDmg(archivePath, destDir)
	}
	return fmt.Errorf("不支持的压缩格式: %s", archivePath)
}

func extractTarXz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	xzr, err := xz.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(xzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			_ = os.Symlink(hdr.Linkname, target)
		}
	}
	return nil
}

func extractZip(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		target := filepath.Join(destDir, zf.Name)
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractDmg(archivePath, destDir string) error {
	mountPoint, err := os.MkdirTemp("", "bx-dmg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountPoint)

	if out, err := exec.Command("hdiutil", "attach", archivePath, "-nobrowse", "-mountpoint", mountPoint).CombinedOutput(); err != nil {
		return fmt.Errorf("hdiutil attach: %v: %s", err, out)
	}
	defer exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()

	var appPath string
	entries, _ := os.ReadDir(mountPoint)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".app") {
			appPath = filepath.Join(mountPoint, e.Name())
			break
		}
	}
	if appPath == "" {
		return fmt.Errorf("dmg 内未找到 .app")
	}
	dstApp := filepath.Join(destDir, filepath.Base(appPath))
	if out, err := exec.Command("cp", "-R", appPath, dstApp).CombinedOutput(); err != nil {
		return fmt.Errorf("拷贝 .app: %v: %s", err, out)
	}
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", dstApp).Run()
	return nil
}
