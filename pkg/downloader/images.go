package downloader

import (
	"crypto/sha256"
	"encoding/base64"
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

const maxDataURLImageBytes = 10 * 1024 * 1024

var supportedDataImageMIMEExt = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

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

	// 创建请求并设置请求头
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	// 设置 User-Agent，模拟浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 设置 Referer，使用图片 URL 的域名
	parsedURL, _ := url.Parse(imageURL)
	if parsedURL != nil {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host))
	}

	// 下载图片数据
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to download image from %s", imageURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d for URL: %s", resp.StatusCode, imageURL)
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

// SaveDataURLImage 解析并保存 data:image/...;base64,... 格式图片
func (d *ImageDownloader) SaveDataURLImage(dataURL string) (string, error) {
	mimeType, encodedData, err := ParseImageDataURL(dataURL)
	if err != nil {
		return "", err
	}

	if base64.StdEncoding.DecodedLen(len(encodedData)) > maxDataURLImageBytes {
		return "", fmt.Errorf("data URL 图片超过大小限制(最大 %dMB)", maxDataURLImageBytes/1024/1024)
	}

	imageData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", errors.Wrap(err, "解析 data URL base64 失败")
	}

	if len(imageData) == 0 {
		return "", errors.New("data URL 图片内容为空")
	}

	if len(imageData) > maxDataURLImageBytes {
		return "", fmt.Errorf("data URL 图片超过大小限制(最大 %dMB)", maxDataURLImageBytes/1024/1024)
	}

	if !filetype.IsImage(imageData) {
		return "", errors.New("data URL 不是有效图片")
	}

	ext, ok := supportedDataImageMIMEExt[mimeType]
	if !ok {
		return "", fmt.Errorf("不支持的 data URL 图片类型: %s", mimeType)
	}

	fileName := d.generateContentFileName(imageData, ext)
	filePath := filepath.Join(d.savePath, fileName)

	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	}

	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return "", errors.Wrap(err, "保存 data URL 图片失败")
	}

	return filePath, nil
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

// ParseImageDataURL 解析 data:image/...;base64,...，返回 MIME 类型和 base64 数据
func ParseImageDataURL(dataURL string) (string, string, error) {
	raw := strings.TrimSpace(dataURL)
	rawLower := strings.ToLower(raw)
	if !strings.HasPrefix(rawLower, "data:image/") {
		return "", "", errors.New("不是 data:image 开头的内容")
	}

	commaIdx := strings.Index(raw, ",")
	if commaIdx <= 0 || commaIdx >= len(raw)-1 {
		return "", "", errors.New("data URL 格式错误，缺少有效数据段")
	}

	meta := raw[:commaIdx]
	metaLower := strings.ToLower(meta)
	if !strings.Contains(metaLower, ";base64") {
		return "", "", errors.New("data URL 必须使用 base64 编码")
	}

	parts := strings.Split(meta, ";")
	if len(parts) == 0 {
		return "", "", errors.New("data URL 元数据为空")
	}

	mimeType := strings.ToLower(strings.TrimPrefix(parts[0], "data:"))
	if _, ok := supportedDataImageMIMEExt[mimeType]; !ok {
		return "", "", fmt.Errorf("不支持的 data URL 图片类型: %s", mimeType)
	}

	encodedData := strings.TrimSpace(raw[commaIdx+1:])
	if encodedData == "" {
		return "", "", errors.New("data URL 图片数据为空")
	}

	return mimeType, encodedData, nil
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

func (d *ImageDownloader) generateContentFileName(content []byte, extension string) string {
	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)
	shortHash := hashStr[:16]
	timestamp := time.Now().Unix()

	return fmt.Sprintf("img_%s_%d.%s", shortHash, timestamp, extension)
}

// IsImageURL 判断字符串是否为图片URL
func IsImageURL(path string) bool {
	return strings.HasPrefix(strings.ToLower(path), "http://") ||
		strings.HasPrefix(strings.ToLower(path), "https://")
}

// IsImageDataURL 判断字符串是否为 data:image/...;base64,... 格式
func IsImageDataURL(path string) bool {
	raw := strings.TrimSpace(strings.ToLower(path))
	return strings.HasPrefix(raw, "data:image/") && strings.Contains(raw, ";base64,")
}
