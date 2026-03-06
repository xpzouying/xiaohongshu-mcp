package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// browserPool 全局浏览器单例，避免每次工具调用都创建新 Chromium 实例
var browserPool = &BrowserPool{}

// BrowserPool 管理共享的 rod.Browser 实例
type BrowserPool struct {
	mu       sync.Mutex
	browser  *rod.Browser
	launcher *launcher.Launcher
}

// GetPage 获取一个新的 stealth page（复用全局 Browser）
func (bp *BrowserPool) GetPage() (*rod.Page, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if err := bp.ensureBrowser(); err != nil {
		return nil, fmt.Errorf("获取浏览器失败: %w", err)
	}

	page, err := stealth.Page(bp.browser)
	if err != nil {
		// 浏览器可能已崩溃，重建
		logrus.Warnf("创建页面失败，尝试重建浏览器: %v", err)
		bp.closeLocked()
		if err2 := bp.ensureBrowser(); err2 != nil {
			return nil, fmt.Errorf("重建浏览器失败: %w", err2)
		}
		page, err = stealth.Page(bp.browser)
		if err != nil {
			return nil, fmt.Errorf("重建后仍无法创建页面: %w", err)
		}
	}

	return page, nil
}

// ensureBrowser 确保全局 Browser 存在（必须在持有锁时调用）
func (bp *BrowserPool) ensureBrowser() error {
	if bp.browser != nil {
		// 检查连接是否还活着
		_, err := bp.browser.Version()
		if err == nil {
			return nil
		}
		logrus.Warnf("浏览器连接已断开: %v, 准备重建", err)
		bp.closeLocked()
	}

	// 清理 SingletonLock
	rodDir := os.Getenv("ROD_DIR")
	if rodDir == "" {
		// 从启动参数中获取 rod dir
		rodDir = getRodDir()
	}
	if rodDir != "" {
		for _, lock := range []string{
			filepath.Join(rodDir, "SingletonLock"),
			filepath.Join(rodDir, "Default", "SingletonLock"),
		} {
			os.Remove(lock)
		}
	}

	// 创建 launcher，根据 headless 模式选择启动方式
	l := launcher.New()
	switch configs.GetHeadlessMode() {
	case configs.HeadlessNew:
		l = l.HeadlessNew(true)
	case configs.HeadlessOld:
		l = l.Headless(true)
	default: // HeadlessOff — 有窗口模式
		l = l.Headless(false)
	}
	l = l.Set("--no-sandbox").
		Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	binPath := configs.GetBinPath()
	if binPath != "" {
		l = l.Bin(binPath)
	}

	u, err := l.Launch()
	if err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}

	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		l.Cleanup()
		return fmt.Errorf("连接浏览器失败: %w", err)
	}

	// 加载 cookies
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)
	if data, err := cookieLoader.LoadCookies(); err == nil {
		var cks []*proto.NetworkCookie
		if err := json.Unmarshal(data, &cks); err != nil {
			logrus.Warnf("解析 cookies 失败: %v", err)
		} else {
			if err := b.SetCookies(toNetworkCookieParams(cks)); err != nil {
				logrus.Warnf("设置 cookies 失败: %v", err)
			}
		}
	} else {
		logrus.Warnf("加载 cookies 失败: %v", err)
	}

	bp.browser = b
	bp.launcher = l
	logrus.Info("全局浏览器实例已创建")
	return nil
}

// closeLocked 关闭浏览器（必须在持有锁时调用）
func (bp *BrowserPool) closeLocked() {
	if bp.browser != nil {
		_ = bp.browser.Close()
		bp.browser = nil
	}
	if bp.launcher != nil {
		bp.launcher.Cleanup()
		bp.launcher = nil
	}
}

// Close 关闭全局浏览器
func (bp *BrowserPool) Close() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.closeLocked()
}

// GetBrowser 获取底层 rod.Browser（用于 saveCookies 等操作）
func (bp *BrowserPool) GetBrowser() *rod.Browser {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.browser
}

// toNetworkCookieParams 转换 cookie 格式
func toNetworkCookieParams(cks []*proto.NetworkCookie) []*proto.NetworkCookieParam {
	params := make([]*proto.NetworkCookieParam, 0, len(cks))
	for _, c := range cks {
		p := &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: c.SameSite,
		}
		if c.Expires > 0 {
			expires := proto.TimeSinceEpoch(c.Expires)
			p.Expires = expires
		}
		params = append(params, p)
	}
	return params
}

// getRodDir 从环境中获取 rod profile 目录
func getRodDir() string {
	// 优先使用命令行传入的 rod dir（通过 -rod 参数解析）
	// fallback 到常见位置
	home, _ := os.UserHomeDir()
	profileDir := filepath.Join(home, ".mcp", "xiaohongshu", "chromium_profile")
	if _, err := os.Stat(profileDir); err == nil {
		return profileDir
	}
	return ""
}
