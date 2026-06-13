package downloader

import (
	"fmt"
	"strings"

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
// 2. 本地文件路径 - 直接使用
// 3. Base64 图片数据 - 保存到本地临时文件
// 保持原始图片顺序，如果下载失败直接返回错误
func (p *ImageProcessor) ProcessImages(images []ImageSource) ([]string, error) {
	localPaths := make([]string, 0, len(images))

	// 按顺序处理每张图片
	for _, image := range images {
		switch strings.ToLower(image.Type) {
		case "url":
			// URL图片：立即下载，失败直接返回错误
			localPath, err := p.downloader.DownloadImage(image.URL)
			if err != nil {
				return nil, fmt.Errorf("下载图片失败 %s: %w", image.URL, err)
			}
			localPaths = append(localPaths, localPath)

		case "base64":
			localPath, err := p.downloader.SaveBase64Image(image.Data, image.MIMEType)
			if err != nil {
				return nil, fmt.Errorf("保存Base64图片失败: %w", err)
			}
			localPaths = append(localPaths, localPath)

		case "path", "":
			if image.Path == "" {
				return nil, fmt.Errorf("image path is empty")
			}
			// 本地路径直接使用
			localPaths = append(localPaths, image.Path)

		default:
			return nil, fmt.Errorf("unsupported image type: %s", image.Type)
		}
	}

	if len(localPaths) == 0 {
		return nil, fmt.Errorf("no valid images found")
	}

	return localPaths, nil
}
