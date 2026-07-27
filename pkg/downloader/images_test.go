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
	// 子目录尚不存在，用于验证 NewImageDownloader 会创建它；t.TempDir 自动清理。
	testPath := filepath.Join(t.TempDir(), "test_downloader")

	downloader := NewImageDownloader(testPath)

	if downloader == nil {
		t.Fatal("NewImageDownloader returned nil")
	}

	if downloader.savePath != testPath {
		t.Errorf("savePath = %q, expected %q", downloader.savePath, testPath)
	}

	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("save path directory was not created: %s", testPath)
	}
}

func TestImageDownloader_isValidImageURL(t *testing.T) {
	downloader := NewImageDownloader(t.TempDir())

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
	downloader := NewImageDownloader(t.TempDir())

	url := "https://example.com/image.jpg"
	extension := "jpg"

	fileName1 := downloader.generateFileName(url, extension)

	if filepath.Ext(fileName1) != "."+extension {
		t.Errorf("fileName should end with .%s, got %s", extension, fileName1)
	}

	if !strings.HasPrefix(filepath.Base(fileName1), "img_") {
		t.Errorf("fileName should start with img_, got %s", fileName1)
	}

	url2 := "https://example.com/different.jpg"
	fileName2 := downloader.generateFileName(url2, extension)
	if fileName1 == fileName2 {
		t.Errorf("different URLs should generate different file names")
	}
}

// TestDownloadImage_SendsUAAndReferer 下载请求应带上 User-Agent 和 Referer。
func TestDownloadImage_SendsUAAndReferer(t *testing.T) {
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
