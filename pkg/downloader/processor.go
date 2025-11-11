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
// 保持原始图片顺序
func (p *ImageProcessor) ProcessImages(images []string) ([]string, error) {
	// 记录每个位置的图片类型和URL索引
	type imageInfo struct {
		isURL     bool
		localPath string
		urlIndex  int // 如果是URL，记录在urlsToDownload中的索引
	}

	imageInfos := make([]imageInfo, 0, len(images))
	urlsToDownload := make([]string, 0)
	urlIndexMap := make(map[string]int) // URL -> urlsToDownload中的索引

	// 第一遍遍历：收集URL和本地路径，保持顺序
	for _, image := range images {
		if IsImageURL(image) {
			// 检查URL是否已经存在（去重）
			if idx, exists := urlIndexMap[image]; exists {
				imageInfos = append(imageInfos, imageInfo{
					isURL:    true,
					urlIndex: idx,
				})
			} else {
				// 新URL，添加到下载列表
				idx := len(urlsToDownload)
				urlsToDownload = append(urlsToDownload, image)
				urlIndexMap[image] = idx
				imageInfos = append(imageInfos, imageInfo{
					isURL:    true,
					urlIndex: idx,
				})
			}
		} else {
			// 本地路径直接记录
			imageInfos = append(imageInfos, imageInfo{
				isURL:     false,
				localPath: image,
			})
		}
	}

	// 批量下载URL图片
	downloadedPaths := make([]string, len(urlsToDownload))
	if len(urlsToDownload) > 0 {
		paths, err := p.downloader.DownloadImages(urlsToDownload)
		if err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
		copy(downloadedPaths, paths)
	}

	// 按原始顺序组装结果
	localPaths := make([]string, 0, len(images))
	for _, info := range imageInfos {
		if info.isURL {
			if info.urlIndex < len(downloadedPaths) {
				localPaths = append(localPaths, downloadedPaths[info.urlIndex])
			}
		} else {
			localPaths = append(localPaths, info.localPath)
		}
	}

	if len(localPaths) == 0 {
		return nil, fmt.Errorf("no valid images found")
	}

	return localPaths, nil
}
