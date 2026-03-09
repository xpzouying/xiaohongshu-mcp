package downloader

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsImageURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://example.com/image.jpg", true},
		{"http://example.com/image.png", true},
		{"HTTPS://example.com/image.gif", true},
		{"/local/path/image.jpg", false},
		{"./relative/path/image.png", false},
		{"image.jpg", false},
		{"ftp://example.com/image.jpg", false},
		{"", false},
	}

	for _, test := range tests {
		result := IsImageURL(test.input)
		if result != test.expected {
			t.Errorf("IsImageURL(%q) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestNewImageDownloader(t *testing.T) {
	tempDir := os.TempDir()
	testPath := filepath.Join(tempDir, "test_downloader")
	defer os.RemoveAll(testPath)

	downloader := NewImageDownloader(testPath)

	if downloader == nil {
		t.Fatal("NewImageDownloader returned nil")
	}

	if downloader.savePath != testPath {
		t.Errorf("savePath = %q, expected %q", downloader.savePath, testPath)
	}

	// 验证目录是否创建
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("save path directory was not created: %s", testPath)
	}
}

func TestImageDownloader_isValidImageURL(t *testing.T) {
	downloader := NewImageDownloader(os.TempDir())

	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com/image.jpg", true},
		{"http://example.com/image.png", true},
		{"https://", false},
		{"http://", false},
		{"invalid-url", false},
		{"ftp://example.com/image.jpg", false},
		{"", false},
	}

	for _, test := range tests {
		result := downloader.isValidImageURL(test.url)
		if result != test.expected {
			t.Errorf("isValidImageURL(%q) = %v, expected %v", test.url, result, test.expected)
		}
	}
}

func TestImageDownloader_generateFileName(t *testing.T) {
	downloader := NewImageDownloader(os.TempDir())

	url := "https://example.com/image.jpg"
	extension := "jpg"

	fileName1 := downloader.generateFileName(url, extension)

	// 文件名应该包含扩展名
	if filepath.Ext(fileName1) != "."+extension {
		t.Errorf("fileName should end with .%s, got %s", extension, fileName1)
	}

	// 文件名应该包含img_前缀
	if !strings.HasPrefix(filepath.Base(fileName1), "img_") {
		t.Errorf("fileName should start with img_, got %s", fileName1)
	}

	// 不同URL应该生成不同的文件名
	url2 := "https://example.com/different.jpg"
	fileName2 := downloader.generateFileName(url2, extension)
	if fileName1 == fileName2 {
		t.Errorf("different URLs should generate different file names")
	}
}

// TestDownloadImage_AntiHotlink 测试下载防盗链图片
// 验证添加 User-Agent 和 Referer 解决 403 问题
func TestDownloadImage_AntiHotlink(t *testing.T) {
	// 1x1 透明 PNG，避免依赖外部网络资源导致测试不稳定
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+2X8AAAAASUVORK5CYII="
	pngData, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		t.Fatalf("解析测试图片失败: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			http.Error(w, "missing user-agent", http.StatusForbidden)
			return
		}

		expectedReferer := fmt.Sprintf("%s/", server.URL)
		if got := r.Header.Get("Referer"); got != expectedReferer {
			http.Error(w, "invalid referer", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	downloader := NewImageDownloader(tempDir)

	filePath, err := downloader.DownloadImage(server.URL + "/image.png")
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("文件不存在: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("下载文件为空")
	}
}

// TestDownloadImage_AntiHotlink_External 集成测试：真实外网防盗链场景
// 默认跳过，设置 XHS_RUN_NETWORK_TESTS=1 后执行。
func TestDownloadImage_AntiHotlink_External(t *testing.T) {
	if os.Getenv("XHS_RUN_NETWORK_TESTS") != "1" {
		t.Skip("skip external network test; set XHS_RUN_NETWORK_TESTS=1 to enable")
	}

	testURL := "https://img1.mydrivers.com/img/20260213/s_fdac2d21214147019e629fa7f2c8802e.png"

	tempDir := t.TempDir()
	downloader := NewImageDownloader(tempDir)

	filePath, err := downloader.DownloadImage(testURL)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("文件不存在: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("下载文件为空")
	}
}
