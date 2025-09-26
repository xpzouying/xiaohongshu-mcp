package configs

import (
    "os"
    "path/filepath"
)

const (
    VideosDir = "xiaohongshu_videos"
)

func GetVideosPath() string {
    return filepath.Join(os.TempDir(), VideosDir)
}

