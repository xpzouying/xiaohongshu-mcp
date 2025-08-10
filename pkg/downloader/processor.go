package downloader

import (
	"fmt"

	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

// ImageProcessor 图片处理器
type ImageProcessor struct {
	downloader *ImageDownloader
}

// NewImageProcessor 创建图片处理器
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		downloader: NewImageDownloader(configs.GetImagesPath()),
	}
}

// ProcessImages 处理图片列表，返回本地文件路径
// 支持两种输入格式：
// 1. URL格式 (http/https开头) - 自动下载到本地
// 2. 本地文件路径 - 直接使用
func (p *ImageProcessor) ProcessImages(images []string) ([]string, error) {
	var localPaths []string
	var urlsToDownload []string

	// 分离URL和本地路径
	for _, image := range images {
		if IsImageURL(image) {
			urlsToDownload = append(urlsToDownload, image)
		} else {
			// 本地路径直接添加
			localPaths = append(localPaths, image)
		}
	}

	// 批量下载URL图片
	if len(urlsToDownload) > 0 {
		downloadedPaths, err := p.downloader.DownloadImages(urlsToDownload)
		if err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
		localPaths = append(localPaths, downloadedPaths...)
	}

	if len(localPaths) == 0 {
		return nil, fmt.Errorf("no valid images found")
	}

	return localPaths, nil
}
