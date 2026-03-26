package browser

import (
	"encoding/json"
	"net/url"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// defaultUserAgent matches the one in headless_browser package.
const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Browser wraps a go-rod browser with Chrome 132+ headless compatibility.
// Replaces headless_browser.Browser to fix "Multiple targets are not
// supported in headless mode" error in Chrome 132+.
type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

// Close closes the browser and cleans up resources.
func (b *Browser) Close() {
	b.browser.MustClose()
	b.launcher.Cleanup()
}

// NewPage creates a new page with stealth mode enabled.
func (b *Browser) NewPage() *rod.Page {
	return stealth.MustPage(b.browser)
}

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

// NewBrowser creates a browser instance with Chrome 132+ headless compatibility.
//
// Chrome 132+ removed --headless=old. The new headless mode rejects
// --no-startup-window (which go-rod's launcher.Headless(true) sets).
// We configure go-rod's launcher directly, setting only --headless
// without --no-startup-window.
func NewBrowser(headless bool, options ...Option) *Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	l := launcher.New().
		Set("no-sandbox").
		Set("user-agent", defaultUserAgent)

	// Set headless WITHOUT --no-startup-window (Chrome 132+ fix)
	if headless {
		l = l.Set("headless")
		l = l.Delete("no-startup-window")
	}

	if cfg.binPath != "" {
		l = l.Bin(cfg.binPath)
	}

	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	debugURL := l.MustLaunch()

	b := rod.New().
		ControlURL(debugURL).
		MustConnect()

	// Load cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cks []*proto.NetworkCookie
		if err := json.Unmarshal(data, &cks); err != nil {
			logrus.Warnf("failed to unmarshal cookies: %v", err)
		} else {
			b.MustSetCookies(cks...)
			logrus.Debugf("loaded cookies from file successfully")
		}
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return &Browser{
		browser:  b,
		launcher: l,
	}
}
