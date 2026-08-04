package browser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type browserConfig struct {
	// fingerprintSeed 固定指纹 seed；>0 时钉死。仅对支持 --fingerprint 的补丁构建有意义。
	fingerprintSeed int
	// proxy 代理地址；非空时启用。
	proxy string
	// binPath 浏览器可执行文件；空则 ResolveBrowserBin()。
	binPath string
}

type Option func(*browserConfig)

// WithProxy 设置代理（http/https/socks5）。空字符串视为不启用。
func WithProxy(proxy string) Option {
	return func(c *browserConfig) {
		c.proxy = proxy
	}
}

// WithFingerprintSeed 设置 seed，seed<=0 视为未设。
// 仅当二进制支持 Cloak/fingerprint-chromium 类 flag 时生效；系统 Chrome 会忽略未知 flag。
func WithFingerprintSeed(seed int) Option {
	return func(c *browserConfig) {
		c.fingerprintSeed = seed
	}
}

// WithBinPath 指定浏览器二进制（通常来自 ResolveBrowserBin / 配置层）。
func WithBinPath(path string) Option {
	return func(c *browserConfig) {
		c.binPath = path
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
	return strings.Replace(proxyURL, u.User.String()+"@", cred+"@", 1)
}

func NewBrowser(headless bool, options ...Option) *Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	binPath := strings.TrimSpace(cfg.binPath)
	if binPath == "" {
		var err error
		binPath, _, err = ResolveBrowserBin()
		if err != nil {
			panic(fmt.Sprintf("browser binary unavailable: %v", err))
		}
	}

	engineCfg := browserEngineConfig{
		headless:      headless,
		chromeBinPath: binPath,
		stealthJS:     false,
		language:      "zh-CN",
	}

	// 指纹 flag 只对补丁 Chromium 有意义；系统 Chrome 上会静默忽略并制造「假启用」错觉。
	stock := IsLikelyStockChrome(binPath)
	if !stock {
		engineCfg.fingerprint = true // 空 platform = 按 OS 自动
		engineCfg.extraFlags = map[string]string{"fingerprint-brand": "Chrome"}
		if cfg.fingerprintSeed > 0 {
			engineCfg.fingerprintSeed = cfg.fingerprintSeed
			logrus.Infof("fingerprint seed pinned: %d", cfg.fingerprintSeed)
		}
	} else {
		logrus.Infof("using stock browser (fingerprint flags disabled): %s", binPath)
	}

	if cfg.proxy != "" {
		engineCfg.proxy = cfg.proxy
		logrus.Infof("Using proxy: %s", maskProxyCredentials(cfg.proxy))
	}

	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		engineCfg.cookies = string(data)
		logrus.Debugf("loaded cookies from file successfully")
	} else {
		logrus.Warnf("failed to load cookies: %v", err)
	}

	return newBrowser(engineCfg)
}
