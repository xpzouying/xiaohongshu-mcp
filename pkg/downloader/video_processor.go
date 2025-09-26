package downloader

import (
    "fmt"

    "github.com/xpzouying/xiaohongshu-mcp/configs"
)

// VideoProcessor 视频处理器
type VideoProcessor struct {
    downloader *VideoDownloader
}

// NewVideoProcessor 创建视频处理器
func NewVideoProcessor() *VideoProcessor {
    return &VideoProcessor{
        downloader: NewVideoDownloader(configs.GetVideosPath()),
    }
}

// ProcessVideo 处理单个视频，返回本地文件路径
// 支持两种输入：
// 1. URL（http/https）- 自动下载保存
// 2. 本地绝对路径 - 直接返回
func (p *VideoProcessor) ProcessVideo(video string) (string, error) {
    if IsVideoURL(video) {
        return p.downloader.DownloadVideo(video)
    }
    if video == "" {
        return "", fmt.Errorf("empty video path")
    }
    // Assume local path exists; upstream will validate on upload
    return video, nil
}

