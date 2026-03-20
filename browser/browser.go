package browser

import (
	"net/url"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type browserConfig struct {
	binPath string
}

type Option func(*browserConfig)

func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

// maskProxyCredentials masks username and password in proxy URL for safe logging.
func maskProxyCredentials(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.User == nil {
		return proxyURL
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword("***", "***")
	} else {
		u.User = url.User("***")
	}
	return u.String()
}

func NewBrowser(headless bool, options ...Option) *headless_browser.Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	// 固定 Chrome user-data-dir 到持久化目录，保证 IndexedDB（mode=local 暂存）不会因为新建浏览器/容器重启而丢失。
	// headless_browser 内部可能会读取这些环境变量来拼接 chrome 启动参数。
	if userDataDir := os.Getenv("XHS_USER_DATA_DIR"); userDataDir != "" {
		userDataDir = filepath.Clean(userDataDir)
		if err := os.MkdirAll(userDataDir, 0o755); err != nil {
			logrus.Warnf("failed to create XHS_USER_DATA_DIR=%s: %v", userDataDir, err)
		} else {
			// 兼容多种可能的环境变量命名（以 headless_browser 实现为准）
			_ = os.Setenv("ROD_BROWSER_USER_DATA_DIR", userDataDir)
			_ = os.Setenv("ROD_USER_DATA_DIR", userDataDir)
			_ = os.Setenv("CHROME_USER_DATA_DIR", userDataDir)
		}
	}

	opts := []headless_browser.Option{
		headless_browser.WithHeadless(headless),
	}
	if cfg.binPath != "" {
		opts = append(opts, headless_browser.WithChromeBinPath(cfg.binPath))
	}

	// Read proxy from environment variable
	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		opts = append(opts, headless_browser.WithProxy(proxy))
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	// 加载 cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		opts = append(opts, headless_browser.WithCookies(string(data)))
		logrus.Debugf("loaded cookies from filesuccessfully")
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return headless_browser.New(opts...)
}
