package browser

import (
	"encoding/json"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// Browser 浏览器实例
type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

// Config 浏览器配置
type Config struct {
	Headless    bool
	BinPath     string
	Proxy       string // 代理地址，如 http://127.0.0.1:7890
	UserDataDir string // 用户数据目录，多用户隔离必须
}

// Option 配置选项
type Option func(*Config)

// WithBinPath 设置浏览器路径
func WithBinPath(binPath string) Option {
	return func(c *Config) {
		c.BinPath = binPath
	}
}

// WithProxy 设置代理
func WithProxy(proxy string) Option {
	return func(c *Config) {
		c.Proxy = proxy
	}
}

// WithUserDataDir 设置用户数据目录
func WithUserDataDir(dir string) Option {
	return func(c *Config) {
		c.UserDataDir = dir
	}
}

// NewBrowser 创建浏览器实例
func NewBrowser(headless bool, options ...Option) *Browser {
	cfg := &Config{Headless: headless}
	for _, opt := range options {
		opt(cfg)
	}

	// 创建 launcher
	l := launcher.New().
		Headless(cfg.Headless).
		Set("no-sandbox").
		Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	// 设置浏览器路径
	if cfg.BinPath != "" {
		l = l.Bin(cfg.BinPath)
	}

	// 设置代理
	if cfg.Proxy != "" {
		l = l.Set("proxy-server", cfg.Proxy)
		logrus.Debugf("browser proxy: %s", cfg.Proxy)
	}

	// 设置用户数据目录（多用户隔离关键）
	if cfg.UserDataDir != "" {
		l = l.UserDataDir(cfg.UserDataDir)
		logrus.Debugf("browser user-data-dir: %s", cfg.UserDataDir)
	}

	url := l.MustLaunch()

	browser := rod.New().
		ControlURL(url).
		MustConnect()

	// 加载 cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cks []*proto.NetworkCookie
		if err := json.Unmarshal(data, &cks); err == nil {
			browser.MustSetCookies(cks...)
			logrus.Debugf("loaded cookies from file successfully")
		} else {
			logrus.Warnf("failed to unmarshal cookies: %v", err)
		}
	} else if os.IsNotExist(err) {
		logrus.Debugf("cookies file not found, skip loading")
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return &Browser{
		browser:  browser,
		launcher: l,
	}
}

// Close 关闭浏览器
func (b *Browser) Close() {
	b.browser.MustClose()
	b.launcher.Cleanup()
}

// NewPage 创建新页面（带 stealth 模式）
func (b *Browser) NewPage() *rod.Page {
	return stealth.MustPage(b.browser)
}
