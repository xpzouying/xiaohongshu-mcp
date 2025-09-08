package browser

import (
	"encoding/json"
	"os/exec"
	"runtime"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// Browser 浏览器封装结构体
type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

// NewPage 创建新页面
func (b *Browser) NewPage() *rod.Page {
	page := b.browser.MustPage()

	// 应用cookies
	b.applyCookies(page)

	return page
}

// Close 关闭浏览器
func (b *Browser) Close() {
	if b.browser != nil {
		b.browser.MustClose()
	}
	if b.launcher != nil {
		b.launcher.Cleanup()
	}
}

// applyCookies 应用cookies到页面
func (b *Browser) applyCookies(page *rod.Page) {
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)

	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cookieData []map[string]interface{}
		if err := json.Unmarshal(data, &cookieData); err == nil {
			// 应用每个cookie - 使用正确的Rod API
			for _, cookie := range cookieData {
				if name, ok := cookie["name"].(string); ok {
					if value, ok := cookie["value"].(string); ok {
						domain := ""
						if d, ok := cookie["domain"].(string); ok {
							domain = d
						}
						path := "/"
						if p, ok := cookie["path"].(string); ok {
							path = p
						}

						// 使用Rod的正确API设置cookie
						page.MustSetCookies(&proto.NetworkCookieParam{
							Name:   name,
							Value:  value,
							Domain: domain,
							Path:   path,
						})
					}
				}
			}
			logrus.Debugf("Applied %d cookies to page", len(cookieData))
		} else {
			logrus.Warnf("Failed to unmarshal cookies: %v", err)
		}
	} else {
		logrus.Debugf("No cookies to load: %v", err)
	}
}

// getGoogleChromeExecutablePath 获取Google Chrome可执行文件路径
func getGoogleChromeExecutablePath() string {
	var googleChromePaths []string

	switch runtime.GOOS {
	case "darwin": // macOS
		googleChromePaths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
	case "windows":
		googleChromePaths = []string{
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
		}
	case "linux":
		googleChromePaths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome-unstable",
		}
	default:
		logrus.Warnf("Unsupported OS: %s", runtime.GOOS)
		return ""
	}

	// 检查每个路径是否存在
	for _, path := range googleChromePaths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
	}

	logrus.Warn("Google Chrome executable not found")
	return ""
}

// NewBrowser 创建新的浏览器实例
func NewBrowser(headless bool) *Browser {
	// 获取Google Chrome可执行文件路径
	googleChromePath := getGoogleChromeExecutablePath()
	if googleChromePath == "" {
		logrus.Fatal("Google Chrome not found. Please install Google Chrome.")
	}

	logrus.Infof("Using Google Chrome at: %s", googleChromePath)

	// 创建launcher
	l := launcher.New().
		Bin(googleChromePath).
		Headless(headless).
		Set("disable-gpu").
		Set("no-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-extensions").
		Set("disable-background-timer-throttling").
		Set("disable-backgrounding-occluded-windows").
		Set("disable-renderer-backgrounding")

	// 启动浏览器
	debuggingURL, err := l.Launch()
	if err != nil {
		logrus.Fatalf("Failed to launch Google Chrome: %v", err)
	}

	// 连接到浏览器
	browser := rod.New().ControlURL(debuggingURL)
	if err := browser.Connect(); err != nil {
		logrus.Fatalf("Failed to connect to Google Chrome: %v", err)
	}

	logrus.Info("Browser launched successfully with Google Chrome")

	return &Browser{
		browser:  browser,
		launcher: l,
	}
}
