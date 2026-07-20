package browser

import (
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type browserConfig struct {
	binPath string
	// fingerprintSeed 固定指纹 seed；>0 时钉死，同账号每次同一套指纹。0 = 每次随机。
	fingerprintSeed int
	// proxy 代理地址；非空时启用。
	proxy string
}

type Option func(*browserConfig)

func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

// WithProxy 设置代理（http/https/socks5）。空字符串视为不启用。
func WithProxy(proxy string) Option {
	return func(c *browserConfig) {
		c.proxy = proxy
	}
}

// WithFingerprintSeed 固定指纹 seed（同账号绑定一套稳定指纹）。
// seed<=0 视为未设，回退每次随机。
func WithFingerprintSeed(seed int) Option {
	return func(c *browserConfig) {
		c.fingerprintSeed = seed
	}
}

// maskProxyCredentials masks username and password in proxy URL for safe logging.
func maskProxyCredentials(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.User == nil {
		return proxyURL
	}
	cred := "***"
	if _, hasPassword := u.User.Password(); hasPassword {
		cred = "***:***"
	}
	// 直接在原串替换 userinfo，避免 url.String() 把 * 编码成 %2A（日志变乱码）。
	return strings.Replace(proxyURL, u.User.String()+"@", cred+"@", 1)
}

func NewBrowser(headless bool, options ...Option) *headless_browser.Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	opts := []headless_browser.Option{
		headless_browser.WithHeadless(headless),
		// 用内置浏览器的默认配置，不强制 UA。
		headless_browser.WithFingerprint(""), // 空 = 按运行 OS 自动：Linux→windows，mac→macos
		headless_browser.WithStealthJS(false),
		headless_browser.WithLanguage("zh-CN"), // 面向小红书
		// 品牌报 Chrome。
		// 注：hardware-concurrency 不设，交给 seed 派生。
		headless_browser.WithExtraFlags(map[string]string{"fingerprint-brand": "Chrome"}),
	}
	if cfg.binPath != "" {
		opts = append(opts, headless_browser.WithChromeBinPath(cfg.binPath))
	}

	// 代理（由调用方经 Option 传入，env 读取放在入口层）。
	if cfg.proxy != "" {
		opts = append(opts, headless_browser.WithProxy(cfg.proxy))
		logrus.Infof("Using proxy: %s", maskProxyCredentials(cfg.proxy))
	}

	// 固定指纹 seed（由调用方经 Option 传入，env 读取放在入口层）。
	if cfg.fingerprintSeed > 0 {
		opts = append(opts, headless_browser.WithFingerprintSeed(cfg.fingerprintSeed))
		logrus.Infof("fingerprint seed pinned: %d", cfg.fingerprintSeed)
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
