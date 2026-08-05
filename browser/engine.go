package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// browserEngineConfig 启动一个 Camoufox 实例所需的全部参数。
type browserEngineConfig struct {
	headless        bool
	binPath         string
	language        string
	cookies         string
	proxy           string
	fingerprintSeed int
	userDataDir     string
	width, height   int
}

// Browser 是常驻 Camoufox 实例的封装，经 playwright-go（Juggler 协议）驱动。
//
// 结构：一个持久化 BrowserContext（持有登录 cookie），Page 按需开关；
// 关 Page 不关 Context/浏览器，避免每请求冷启。指纹由 Camoufox 在 C++ 层注入，
// 无需 stealth JS 或事后 UA 覆盖。
type Browser struct {
	pw      *playwright.Playwright
	ctx     playwright.BrowserContext
	userDir string

	mu       sync.Mutex
	closed   bool
	ownsDir  bool
	profiles ProfileOptions
}

// ProfileOptions 控制指纹与窗口等运行时画像。
type ProfileOptions struct {
	Language string
	Width    int
	Height   int
}

func newBrowser(cfg browserEngineConfig) (*Browser, error) {
	if err := ensureDriverEnv(); err != nil {
		return nil, err
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright driver failed: %w", err)
	}

	env, err := camouEnv(buildCamouConfig(cfg.fingerprintSeed, cfg.language))
	if err != nil {
		_ = pw.Stop()
		return nil, err
	}

	userDir, ownsDir, err := resolveUserDataDir(cfg.userDataDir)
	if err != nil {
		_ = pw.Stop()
		return nil, err
	}

	headless := cfg.headless
	width, height := cfg.width, cfg.height
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 800
	}

	opts := playwright.BrowserTypeLaunchPersistentContextOptions{
		ExecutablePath:  playwright.String(cfg.binPath),
		Headless:        &headless,
		Env:             env,
		Viewport:        &playwright.Size{Width: width, Height: height},
		Locale:          playwright.String(cfg.language),
		AcceptDownloads: playwright.Bool(false),
	}
	if cfg.proxy != "" {
		opts.Proxy = &playwright.Proxy{Server: cfg.proxy}
	}

	ctx, err := pw.Firefox.LaunchPersistentContext(userDir, opts)
	if err != nil {
		_ = pw.Stop()
		if ownsDir {
			_ = os.RemoveAll(userDir)
		}
		return nil, fmt.Errorf("launch camoufox failed: %w", err)
	}

	// 注入登录 cookie（文件为 CDP 线格式，见 cookies.go）
	if cfg.cookies != "" {
		injected := addCookies(ctx, cookiesToOptional([]byte(cfg.cookies)))
		logrus.Debugf("injected %d cookies into context", injected)
	}

	b := &Browser{
		pw:      pw,
		ctx:     ctx,
		userDir: userDir,
		ownsDir: ownsDir,
		profiles: ProfileOptions{
			Language: cfg.language,
			Width:    width,
			Height:   height,
		},
	}
	return b, nil
}

// NewPage 打开一个新标签页。调用方负责 Close。
func (b *Browser) NewPage() (playwright.Page, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("browser is closed")
	}
	return b.ctx.NewPage()
}

// Context 暴露底层持久化上下文（取 cookie、新增 page 等）。
func (b *Browser) Context() playwright.BrowserContext {
	return b.ctx
}

// Cookies 导出当前上下文的全部 cookie（CDP 线格式字节），供登录后落盘。
func (b *Browser) Cookies() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("browser is closed")
	}
	cks, err := b.ctx.Cookies()
	if err != nil {
		return nil, err
	}
	return cookiesFromPlaywright(cks), nil
}

// Close 关闭上下文与驱动进程，并按需清理临时 profile。
func (b *Browser) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true

	if b.ctx != nil {
		if err := b.ctx.Close(); err != nil {
			logrus.Warnf("close browser context failed: %v", err)
		}
	}
	if b.pw != nil {
		if err := b.pw.Stop(); err != nil {
			logrus.Warnf("stop playwright driver failed: %v", err)
		}
	}
	if b.ownsDir && b.userDir != "" {
		if err := os.RemoveAll(b.userDir); err != nil {
			logrus.Warnf("cleanup camoufox profile failed: %v", err)
		}
	}
}

// addCookies 批量注入 cookie；整批失败时回退逐条注入，
// 避免单条坏 cookie（过期、域不匹配等）拖垮整份登录态。返回成功注入的条数。
func addCookies(ctx playwright.BrowserContext, ocs []playwright.OptionalCookie) int {
	if len(ocs) == 0 {
		return 0
	}
	if err := ctx.AddCookies(ocs); err == nil {
		return len(ocs)
	}
	logrus.Warnf("batch addCookies failed, fallback to per-cookie injection (%d cookies)", len(ocs))
	ok := 0
	for _, oc := range ocs {
		if err := ctx.AddCookies([]playwright.OptionalCookie{oc}); err != nil {
			logrus.Warnf("skip cookie %q: %v", oc.Name, err)
			continue
		}
		ok++
	}
	return ok
}

// resolveUserDataDir 决定 Camoufox profile 目录。
// 未显式指定时用临时目录（ownsDir=true），Close 时删除，与 go-rod 时代
// launcher.Cleanup 删除临时 user-data 的行为对齐。
func resolveUserDataDir(configured string) (dir string, owns bool, err error) {
	if configured != "" {
		abs, err := filepath.Abs(configured)
		if err != nil {
			return "", false, err
		}
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return "", false, err
		}
		return abs, false, nil
	}
	dir, err = os.MkdirTemp("", "camoufox-profile-*")
	if err != nil {
		return "", false, err
	}
	return dir, true, nil
}
