package downloader

import (
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
