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

// SaveCookies 保存 cookies 到文件中。
func (c *localCookie) SaveCookies(data []byte) error {
	dir := filepath.Dir(c.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "failed to create cookies dir")
		}
	}
	return os.WriteFile(c.path, data, 0644)
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
// 解析顺序（高到低）：
// 1) COOKIES_PATH 环境变量（推荐）
// 2) 旧路径 /tmp/cookies.json（兼容历史）
// 3) 当前目录 cookies.json（兼容已有部署）
// 4) 稳定默认路径 ~/.xiaohongshu-mcp/cookies.json（避免 cwd 漂移）
func GetCookiesFilePath() string {
	if p := os.Getenv("COOKIES_PATH"); p != "" {
		return p
	}

	// 旧路径：/tmp/cookies.json
	tmpDir := os.TempDir()
	oldPath := filepath.Join(tmpDir, "cookies.json")
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath
	}

	// 兼容历史：当前目录 cookies.json
	if _, err := os.Stat("cookies.json"); err == nil {
		return "cookies.json"
	}

	// 稳定默认路径：用户目录
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".xiaohongshu-mcp", "cookies.json")
	}

	// 极端兜底
	return "cookies.json"
}
