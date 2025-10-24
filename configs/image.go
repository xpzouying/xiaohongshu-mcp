package configs

import (
	"os"
	"path/filepath"
)

const (
	ImagesDir = "xiaohongshu_images"
)

func GetImagesPath() string {
	return filepath.Join(os.TempDir(), ImagesDir)
}
