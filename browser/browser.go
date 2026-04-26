package browser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
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

// maskProxyCredentials 对代理 URL 中的密码进行脱敏，用于安全日志记录
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

// Browser 封装了 rod.Browser，提供页面创建和清理功能
type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

// NewBrowser 创建新的浏览器实例，内置 stealth 反检测和多项反自动化配置
func NewBrowser(headless bool, options ...Option) *Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	w, h := configs.GetWindowSize()

	l := launcher.New().
		Set("--no-sandbox").
		Set("--disable-blink-features=AutomationControlled").
		Set("--disable-features=Translate").
		Set("--disable-sync").
		Set("--no-first-run").
		Set("--no-default-browser-check").
		Set("--disable-background-networking").
		Set("--disable-background-timer-throttling").
		Set("--disable-renderer-backgrounding").
		Set("--window-size", fmt.Sprintf("%d,%d", w, h)).
		Set("--start-maximized")

	// 使用新版无头模式 (--headless=new)，比旧版更难以被检测
	if headless {
		l.Set("--headless", "new")
	}

	// 设置自定义 user-agent（覆盖默认的 HeadlessChrome UA）
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	l.Set("user-agent", ua)

	if cfg.binPath != "" {
		l = l.Bin(cfg.binPath)
	}

	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
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
			logrus.Debugf("loaded %d cookies from file", len(cks))
		}
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return &Browser{
		browser:  browser,
		launcher: l,
	}
}

// NewPage 创建新页面，自动应用 stealth 反检测
func (b *Browser) NewPage() *rod.Page {
	return stealth.MustPage(b.browser)
}

// Close 关闭浏览器并清理资源
func (b *Browser) Close() {
	b.browser.MustClose()
	b.launcher.Cleanup()
}
