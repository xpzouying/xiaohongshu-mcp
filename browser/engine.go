package browser

import (
	"encoding/json"
	"math/rand"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
)

// Browser is the local browser wrapper used by the MCP service.
//
// This intentionally lives in the repository instead of relying on a wrapper
// that unconditionally disables Chromium's process sandbox.
type Browser struct {
	browser   *rod.Browser
	launcher  *launcher.Launcher
	stealthJS bool

	// uaOverride keeps the browser version and Client Hints consistent when a
	// fingerprint-enabled Chromium is used.
	uaOverride *proto.NetworkSetUserAgentOverride
}

type browserEngineConfig struct {
	headless        bool
	userAgent       string
	cookies         string
	chromeBinPath   string
	proxy           string
	fingerprint     bool
	fingerprintPlat string
	fingerprintSeed int
	stealthJS       bool
	language        string
	extraFlags      map[string]string
	trace           bool
}

// newBrowserLauncher constructs the browser launcher with Chromium's sandbox
// and process isolation explicitly enabled. go-rod adds --no-sandbox and a
// few isolation-reducing flags automatically in containers/automation mode;
// deleting them here keeps those unsafe defaults from leaking back in.
func newBrowserLauncher(cfg browserEngineConfig) *launcher.Launcher {
	l := launcher.New().
		NoSandbox(false).
		Set("disable-features", "TranslateUI").
		Delete("disable-site-isolation-trials").
		Delete("disable-ipc-flooding-protection").
		Headless(cfg.headless)

	if cfg.userAgent != "" {
		l = l.Set("user-agent", cfg.userAgent)
	}

	if cfg.fingerprint {
		platform := cfg.fingerprintPlat
		if platform == "" {
			platform = autoFingerprintPlatform()
		}
		seed := cfg.fingerprintSeed
		if seed == 0 {
			seed = rand.Intn(89999) + 10000
		}
		l = l.Set("fingerprint", strconv.Itoa(seed)).
			Set("fingerprint-platform", platform)
		logrus.Infof("fingerprint enabled: platform=%s seed=%d", platform, seed)
	}

	for k, v := range cfg.extraFlags {
		l = l.Set(flags.Flag(k), v)
	}

	if cfg.chromeBinPath != "" {
		l = l.Bin(cfg.chromeBinPath)
	}

	if cfg.proxy != "" {
		l = l.Proxy(cfg.proxy)
	}

	return l
}

// newBrowser starts a browser using only the executable selected by the
// repository's resolver. It never invokes go-rod's browser downloader.
func newBrowser(cfg browserEngineConfig) *Browser {
	l := newBrowserLauncher(cfg)
	url := l.MustLaunch()

	browser := rod.New().
		ControlURL(url).
		Trace(cfg.trace).
		MustConnect()

	if cfg.cookies != "" {
		var cookies []*proto.NetworkCookie
		if err := json.Unmarshal([]byte(cfg.cookies), &cookies); err != nil {
			logrus.Warnf("failed to unmarshal cookies: %v", err)
		} else {
			browser.MustSetCookies(cookies...)
		}
	}

	b := &Browser{
		browser:   browser,
		launcher:  l,
		stealthJS: cfg.stealthJS,
	}

	if cfg.fingerprint {
		if ov, err := buildUAOverride(browser, cfg.language); err != nil {
			logrus.Warnf("build UA override failed, skip: %v", err)
		} else {
			b.uaOverride = ov
		}
	}

	return b
}

// autoFingerprintPlatform returns the platform presented by the fingerprint
// patch. Linux servers use a common Windows profile; macOS stays macOS.
func autoFingerprintPlatform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "windows"
}

// buildUAOverride reads the real browser version instead of hard-coding it.
func buildUAOverride(browser *rod.Browser, language string) (*proto.NetworkSetUserAgentOverride, error) {
	ver, err := proto.BrowserGetVersion{}.Call(browser)
	if err != nil {
		return nil, err
	}

	fullVersion := strings.TrimPrefix(ver.Product, "Chrome/")
	major := fullVersion
	if i := strings.IndexByte(fullVersion, '.'); i > 0 {
		major = fullVersion[:i]
	}

	ov := &proto.NetworkSetUserAgentOverride{
		UserAgent: ver.UserAgent,
		UserAgentMetadata: &proto.EmulationUserAgentMetadata{
			Brands: []*proto.EmulationUserAgentBrandVersion{
				{Brand: "Not:A-Brand", Version: "99"},
				{Brand: "Google Chrome", Version: major},
				{Brand: "Chromium", Version: major},
			},
			FullVersionList: []*proto.EmulationUserAgentBrandVersion{
				{Brand: "Not:A-Brand", Version: "99.0.0.0"},
				{Brand: "Google Chrome", Version: fullVersion},
				{Brand: "Chromium", Version: fullVersion},
			},
			FullVersion: fullVersion,
		},
	}

	if language != "" {
		ov.AcceptLanguage = language + "," + primaryLang(language)
	}

	logrus.Infof("UA coherence: version=%s", ver.Product)
	return ov, nil
}

func primaryLang(language string) string {
	if i := strings.IndexByte(language, '-'); i > 0 {
		return language[:i]
	}
	return language
}

// Close closes the browser and removes its temporary profile.
func (b *Browser) Close() {
	b.browser.MustClose()
	b.launcher.Cleanup()
}

// NewPage creates a page and applies the configured page-level adjustments.
func (b *Browser) NewPage() *rod.Page {
	var page *rod.Page
	if b.stealthJS {
		page = stealth.MustPage(b.browser)
	} else {
		page = b.browser.MustPage()
	}

	if b.uaOverride != nil {
		if err := b.uaOverride.Call(page); err != nil {
			logrus.Warnf("apply UA override failed: %v", err)
		}
	}
	return page
}
