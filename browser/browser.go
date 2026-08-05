package browser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type browserConfig struct {
	// fingerprintSeed 固定指纹 seed；>0 时钉死，同一账号每次启动得到同一画像。
	fingerprintSeed int
	// proxy 代理地址；非空时启用。
	proxy string
	// binPath Camoufox 可执行文件；空则 ResolveCamoufoxBin()。
	binPath string
	// userDataDir Camoufox profile 目录；空则用临时目录（Close 时清理）。
	userDataDir string
	// width/height 视口尺寸；<=0 时用默认 1280x800。
	width, height int
}

type Option func(*browserConfig)

// WithProxy 设置代理（http/https/socks5）。空字符串视为不启用。
func WithProxy(proxy string) Option {
	return func(c *browserConfig) {
		c.proxy = proxy
	}
}

// WithFingerprintSeed 设置指纹 seed，seed<=0 视为未设（随机）。
func WithFingerprintSeed(seed int) Option {
	return func(c *browserConfig) {
		c.fingerprintSeed = seed
	}
}

// WithBinPath 指定 Camoufox 二进制（通常来自 ResolveCamoufoxBin / 配置层）。
func WithBinPath(path string) Option {
	return func(c *browserConfig) {
		c.binPath = path
	}
}

// WithUserDataDir 指定持久化 profile 目录；缺省用临时目录，Close 时删除。
func WithUserDataDir(dir string) Option {
	return func(c *browserConfig) {
		c.userDataDir = dir
	}
}

// WithViewport 设置视口尺寸。
func WithViewport(width, height int) Option {
	return func(c *browserConfig) {
		c.width, c.height = width, height
	}
}

// maskProxyCredentials 日志里遮蔽代理的用户名密码。
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

// NewBrowser 启动一个常驻 Camoufox 实例（持久化上下文 + Juggler 驱动）。
// 失败返回错误，由调用方决定是否重建，不 panic。
func NewBrowser(headless bool, options ...Option) (*Browser, error) {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	binPath := strings.TrimSpace(cfg.binPath)
	if binPath == "" {
		var err error
		binPath, _, err = ResolveCamoufoxBin()
		if err != nil {
			return nil, fmt.Errorf("camoufox binary unavailable: %w", err)
		}
	}

	engineCfg := browserEngineConfig{
		headless:        headless,
		binPath:         binPath,
		language:        "zh-CN",
		fingerprintSeed: cfg.fingerprintSeed,
		userDataDir:     cfg.userDataDir,
		width:           cfg.width,
		height:          cfg.height,
	}
	if cfg.fingerprintSeed > 0 {
		logrus.Infof("fingerprint seed pinned: %d", cfg.fingerprintSeed)
	}

	if cfg.proxy != "" {
		engineCfg.proxy = cfg.proxy
		logrus.Infof("using proxy: %s", maskProxyCredentials(cfg.proxy))
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
