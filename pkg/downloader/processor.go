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
// 支持三种输入格式：
// 1. URL格式 (http/https开头) - 自动下载到本地
// 2. data URL格式 (data:image/...;base64,...) - 解析并保存到本地
// 3. 本地文件路径 - 直接使用
// 保持原始图片顺序，如果下载失败直接返回错误
func (p *ImageProcessor) ProcessImages(images []string) ([]string, error) {
	localPaths := make([]string, 0, len(images))

	// 按顺序处理每张图片
	for _, image := range images {
		if IsImageURL(image) {
			// URL图片：立即下载，失败直接返回错误
			localPath, err := p.downloader.DownloadImage(image)
			if err != nil {
				return nil, fmt.Errorf("下载图片失败 %s: %w", image, err)
			}
			localPaths = append(localPaths, localPath)
		} else if IsImageDataURL(image) {
			// data URL 图片：解码并保存到本地
			localPath, err := p.downloader.SaveDataURLImage(image)
			if err != nil {
				return nil, fmt.Errorf("解析 data URL 图片失败: %w", err)
			}
			localPaths = append(localPaths, localPath)
		} else {
			// 本地路径直接使用
			localPaths = append(localPaths, image)
		}
	}

	if len(localPaths) == 0 {
		return nil, fmt.Errorf("no valid images found")
	}

	return localPaths, nil
}
