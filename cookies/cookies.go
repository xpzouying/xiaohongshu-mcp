package cookies

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

// sessionFile 是 v2 的文件结构。v1 是一个裸 cookie 数组，没有外层对象。
// cookies 用 RawMessage 原样透传，不解析不重组，避免往返时字段走样。
type sessionFile struct {
	Version int             `json:"version"`
	Seed    int             `json:"seed,omitempty"`
	SavedAt string          `json:"saved_at,omitempty"`
	Cookies json.RawMessage `json:"cookies"`
}

// localCookiesPath 当前目录下的默认文件名。
const localCookiesPath = "cookies.json"

type Cookier interface {
	LoadCookies() ([]byte, error)
	SaveCookies(data []byte) error
	// LoadSeed 读取会话绑定的 seed；老格式、文件损坏或未设时返回 0。
	LoadSeed() int
	// SaveSeed 写入 seed，保留文件中已有的 cookies。
	SaveSeed(seed int) error
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

// LoadCookies 从文件中加载 cookies 数组的原始字节。
// v2 从外层对象里取出 cookies 字段；v1 文件本身就是数组，原样返回。
func (c *localCookie) LoadCookies() ([]byte, error) {

	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cookies from tmp file")
	}

	var f sessionFile
	if err := json.Unmarshal(data, &f); err == nil && len(f.Cookies) > 0 {
		return f.Cookies, nil
	}

	return data, nil
}

// LoadSeed 读取会话绑定的 seed。老格式（裸数组）没有这个值，返回 0。
func (c *localCookie) LoadSeed() int {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return 0
	}

	var f sessionFile
	if err := json.Unmarshal(data, &f); err != nil {
		return 0
	}
	return f.Seed
}

// SaveCookies 保存 cookies 到文件中，保留文件里已有的 seed。
func (c *localCookie) SaveCookies(data []byte) error {
	return c.write(data, c.LoadSeed())
}

// SaveSeed 写入 seed，保留文件里已有的 cookies。
func (c *localCookie) SaveSeed(seed int) error {
	cks, err := c.LoadCookies()
	if err != nil {
		cks = nil // 文件还不存在：先把 seed 落下来，cookies 之后再补
	}
	return c.write(cks, seed)
}

// write 以 v2 格式落盘。cookies 用 RawMessage 原样嵌入，不经过结构体往返。
func (c *localCookie) write(cks []byte, seed int) error {
	if len(cks) == 0 {
		cks = []byte("[]")
	}

	data, err := json.MarshalIndent(sessionFile{
		Version: 2,
		Seed:    seed,
		SavedAt: time.Now().Format(time.RFC3339),
		Cookies: json.RawMessage(cks),
	}, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal session file failed")
	}

	if dir := filepath.Dir(c.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "create cookies dir failed")
		}
	}

	// cookies 含登录态，0600 仅属主可读
	return os.WriteFile(c.path, data, 0600)
}

// GetCookiesFilePath 获取 cookies 文件路径。
// 为了向后兼容，如果旧路径 /tmp/cookies.json 存在，则继续使用；
// 否则使用当前目录下的 cookies.json
func GetCookiesFilePath() string {
	// 显式指定优先，无条件——环境里的残留文件不能盖掉用户明说的配置
	if path := os.Getenv("COOKIES_PATH"); path != "" {
		return path
	}

	// 本地目录
	if _, err := os.Stat(localCookiesPath); err == nil {
		return localCookiesPath
	}

	// 旧路径 /tmp/cookies.json，仅为老用户兜底
	oldPath := filepath.Join(os.TempDir(), "cookies.json")
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath
	}

	return localCookiesPath
}
