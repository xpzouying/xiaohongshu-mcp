package cookies

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type Cookier interface {
	LoadCookies() ([]byte, error)
	SaveCookies(data []byte) error
	DeleteCookies() error
}

type localCookie struct {
	path string
}

func NewLoadCookie(path string) Cookier {
	if path == "" {
		panic("path is required")
	}

	return &localCookie{
		path: path,
	}
}

// LoadCookies 从文件中加载 cookies。
func (c *localCookie) LoadCookies() ([]byte, error) {

	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cookies from tmp file")
	}

	return data, nil
}

// SaveCookies 原子写入 cookies 到文件中。
// 先写临时文件再 rename，防止写入中途崩溃导致文件损坏。
func (c *localCookie) SaveCookies(data []byte) error {
	dir := filepath.Dir(c.path)
	tmp, err := os.CreateTemp(dir, "cookies.tmp.*")
	if err != nil {
		return errors.Wrap(err, "创建临时文件失败")
	}
	tmpPath := tmp.Name()

	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Wrap(err, "chmod 临时文件失败")
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Wrap(err, "写入临时文件失败")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Wrap(err, "sync 临时文件失败")
	}
	tmp.Close()

	if err := os.Rename(tmpPath, c.path); err != nil {
		os.Remove(tmpPath)
		return errors.Wrap(err, "rename 临时文件失败")
	}
	return nil
}

// DeleteCookies 删除 cookies 文件。
func (c *localCookie) DeleteCookies() error {
	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		// 文件不存在，返回 nil（认为已经删除）
		return nil
	}
	return os.Remove(c.path)
}

// GetCookiesFilePath 获取 cookies 文件路径。
// 为了向后兼容，如果旧路径 /tmp/cookies.json 存在，则继续使用；
// 否则使用当前目录下的 cookies.json
func GetCookiesFilePath() string {
	// 旧路径：/tmp/cookies.json
	tmpDir := os.TempDir()
	oldPath := filepath.Join(tmpDir, "cookies.json")

	// 检查旧路径文件是否存在
	if _, err := os.Stat(oldPath); err == nil {
		// 文件存在，使用旧路径（向后兼容）
		return oldPath
	}

	path := os.Getenv("COOKIES_PATH") // 判断环境变量
	if path == "" {
		path = "cookies.json" // fallback，本地调试时用当前目录
	}

	// 文件不存在，使用新路径（当前目录）
	return path
}
