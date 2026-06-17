package browser

import (
	"net/url"
	"os"

	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/humanize"
	hrod "github.com/xpzouying/xiaohongshu-mcp/pkg/humanize/rod"
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

func NewBrowser(headless bool, options ...Option) *hrod.Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
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
		logrus.Debugf("loaded cookies from file successfully")
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	hb := headless_browser.New(opts...)

	// 禁用浏览器地理位置权限弹窗，避免发布页请求定位时弹出系统级权限框。
	if err := disableGeolocationPermission(hb); err != nil {
		logrus.Warnf("failed to disable geolocation permission: %v", err)
	}

	return hrod.NewBrowser(hb, humanize.DefaultConfig())
}

// disableGeolocationPermission 通过 CDP 将 geolocation 权限设为 denied，
// 使页面调用 navigator.geolocation 时直接失败，不会弹出浏览器级权限框。
func disableGeolocationPermission(hb *headless_browser.Browser) error {
	// BrowserSetPermission 作用于 browser context，需要借助一个临时 page 获取 rod.Browser。
	tempPage := hb.NewPage()
	defer tempPage.Close()

	return proto.BrowserSetPermission{
		Permission: &proto.BrowserPermissionDescriptor{Name: "geolocation"},
		Setting:    proto.BrowserPermissionSettingDenied,
	}.Call(tempPage.Browser())
}
