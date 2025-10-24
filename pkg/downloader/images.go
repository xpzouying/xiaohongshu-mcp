package downloader

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"github.com/pkg/errors"
)

// ImageDownloader 图片下载器
type ImageDownloader struct {
	savePath   string
	httpClient *http.Client
}

// NewImageDownloader 创建图片下载器
func NewImageDownloader(savePath string) *ImageDownloader {
	// 确保保存目录存在
	if err := os.MkdirAll(savePath, 0755); err != nil {
		panic(fmt.Sprintf("failed to create save path: %v", err))
	}

	return &ImageDownloader{
		savePath: savePath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadImage 下载图片
// 返回本地文件路径
func (d *ImageDownloader) DownloadImage(imageURL string) (string, error) {
	// 验证URL格式
	if !d.isValidImageURL(imageURL) {
		return "", errors.New("invalid image URL format")
	}

	// 下载图片数据
	resp, err := d.httpClient.Get(imageURL)
	if err != nil {
		return "", errors.Wrap(err, "failed to download image")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// 读取图片数据
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read image data")
	}

	// 检测图片格式
	kind, err := filetype.Match(imageData)
	if err != nil {
		return "", errors.Wrap(err, "failed to detect file type")
	}

	if !filetype.IsImage(imageData) {
		return "", errors.New("downloaded file is not a valid image")
	}

	// 生成唯一文件名
	fileName := d.generateFileName(imageURL, kind.Extension)
	filePath := filepath.Join(d.savePath, fileName)

	// 如果文件已存在，直接返回路径
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	}

	// 保存到文件
	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return "", errors.Wrap(err, "failed to save image")
	}

	return filePath, nil
}

// DownloadImages 批量下载图片
func (d *ImageDownloader) DownloadImages(imageURLs []string) ([]string, error) {
	var localPaths []string
	var errs []error

	for _, imageURL := range imageURLs {
		localPath, err := d.DownloadImage(imageURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to download %s: %w", imageURL, err))
			continue
		}
		localPaths = append(localPaths, localPath)
	}

	if len(errs) > 0 {
		return localPaths, fmt.Errorf("download errors occurred: %v", errs)
	}

	return localPaths, nil
}

// isValidImageURL 检查是否为有效的图片URL
func (d *ImageDownloader) isValidImageURL(rawURL string) bool {
	// 检查是否以http/https开头
	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") &&
		!strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return false
	}

	// 检查URL格式
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return parsedURL.Scheme != "" && parsedURL.Host != ""
}

// generateFileName 生成唯一的文件名
func (d *ImageDownloader) generateFileName(imageURL, extension string) string {
	// 使用URL的SHA256哈希作为文件名，确保唯一性
	hash := sha256.Sum256([]byte(imageURL))
	hashStr := fmt.Sprintf("%x", hash)

	// 取前16位哈希值作为文件名
	shortHash := hashStr[:16]

	// 添加时间戳确保更好的唯一性
	timestamp := time.Now().Unix()

	return fmt.Sprintf("img_%s_%d.%s", shortHash, timestamp, extension)
}

// IsImageURL 判断字符串是否为图片URL
func IsImageURL(path string) bool {
	return strings.HasPrefix(strings.ToLower(path), "http://") ||
		strings.HasPrefix(strings.ToLower(path), "https://")
}
