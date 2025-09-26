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

// VideoDownloader 视频下载器
type VideoDownloader struct {
    savePath   string
    httpClient *http.Client
}

// NewVideoDownloader 创建视频下载器
func NewVideoDownloader(savePath string) *VideoDownloader {
    if err := os.MkdirAll(savePath, 0755); err != nil {
        panic(fmt.Sprintf("failed to create save path: %v", err))
    }
    return &VideoDownloader{
        savePath: savePath,
        httpClient: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

// DownloadVideo 下载视频并返回本地路径
func (d *VideoDownloader) DownloadVideo(videoURL string) (string, error) {
    if !d.isValidVideoURL(videoURL) {
        return "", errors.New("invalid video URL format")
    }

    resp, err := d.httpClient.Get(videoURL)
    if err != nil {
        return "", errors.Wrap(err, "failed to download video")
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
    }

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", errors.Wrap(err, "failed to read video data")
    }

    kind, err := filetype.Match(data)
    if err != nil {
        return "", errors.Wrap(err, "failed to detect file type")
    }

    if !filetype.IsVideo(data) {
        return "", errors.New("downloaded file is not a valid video")
    }

    fileName := d.generateFileName(videoURL, kind.Extension)
    filePath := filepath.Join(d.savePath, fileName)

    if _, err := os.Stat(filePath); err == nil {
        return filePath, nil
    }

    if err := os.WriteFile(filePath, data, 0644); err != nil {
        return "", errors.Wrap(err, "failed to save video")
    }

    return filePath, nil
}

func (d *VideoDownloader) isValidVideoURL(rawURL string) bool {
    if !strings.HasPrefix(strings.ToLower(rawURL), "http://") &&
        !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
        return false
    }
    parsedURL, err := url.Parse(rawURL)
    if err != nil {
        return false
    }
    return parsedURL.Scheme != "" && parsedURL.Host != ""
}

func (d *VideoDownloader) generateFileName(videoURL, extension string) string {
    hash := sha256.Sum256([]byte(videoURL))
    hashStr := fmt.Sprintf("%x", hash)
    shortHash := hashStr[:16]
    timestamp := time.Now().Unix()
    return fmt.Sprintf("vid_%s_%d.%s", shortHash, timestamp, extension)
}

// IsVideoURL 判断是否为视频URL
func IsVideoURL(path string) bool {
    return strings.HasPrefix(strings.ToLower(path), "http://") ||
        strings.HasPrefix(strings.ToLower(path), "https://")
}

