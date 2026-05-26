package browser

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

type browserConfig struct {
	binPath      string
	cloakBrowser bool
}

type Browser struct {
	browser      *rod.Browser
	launcher     *launcher.Launcher
	useStealth   bool
	cloakBrowser bool
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

func NewBrowser(headless bool, options ...Option) *Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	binPath := resolveBrowserBinPathConfig(cfg)
	cloakBrowser := cfg.cloakBrowser || isCloakBrowserBin(binPath)

	l := launcher.New().
		Headless(headless).
		Set("--no-sandbox")

	if binPath != "" {
		l = l.Bin(binPath)
	}

	if !cloakBrowser {
		l = l.Set("user-agent", defaultUserAgent)
	} else {
		logrus.Infof("Using CloakBrowser binary: %s", binPath)
	}

	// Read proxy from environment variable.
	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
		logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
	}

	url := l.MustLaunch()
	rodBrowser := rod.New().
		ControlURL(url).
		MustConnect()

	// Load cookies.
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cks []*proto.NetworkCookie
		if err = json.Unmarshal(data, &cks); err != nil {
			logrus.Warnf("failed to unmarshal cookies: %v", err)
		} else {
			rodBrowser.MustSetCookies(cks...)
			logrus.Debugf("loaded cookies from file successfully")
		}
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return &Browser{
		browser:      rodBrowser,
		launcher:     l,
		useStealth:   !cloakBrowser,
		cloakBrowser: cloakBrowser,
	}
}

func (b *Browser) Close() {
	b.browser.MustClose()
	b.launcher.Cleanup()
}

func (b *Browser) NewPage() *rod.Page {
	if b.useStealth {
		return stealth.MustPage(b.browser)
	}
	return b.browser.MustPage()
}

func resolveBrowserBinPathConfig(cfg *browserConfig) string {
	if cfg.binPath != "" {
		return cfg.binPath
	}
	if envPath := os.Getenv("ROD_BROWSER_BIN"); envPath != "" {
		return envPath
	}
	if envPath := os.Getenv("CLOAKBROWSER_BINARY_PATH"); envPath != "" {
		cfg.cloakBrowser = true
		return envPath
	}
	return ""
}

func isCloakBrowserBin(binPath string) bool {
	normalized := strings.ToLower(filepath.ToSlash(binPath))
	return strings.Contains(normalized, "cloakbrowser") ||
		strings.Contains(normalized, ".cloakbrowser")
}
