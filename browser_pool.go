package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

// BrowserPool 浏览器连接池
type BrowserPool struct {
	browser     *headless_browser.Browser
	page        *rod.Page
	mu          sync.RWMutex
	initialized bool
	lastUsed    time.Time
	maxIdleTime time.Duration
}

// 全局浏览器池实例
var globalBrowserPool *BrowserPool
var poolOnce sync.Once

// GetBrowserPool 获取全局浏览器池实例
func GetBrowserPool() *BrowserPool {
	poolOnce.Do(func() {
		globalBrowserPool = &BrowserPool{
			maxIdleTime: 30 * time.Minute,
		}
	})
	return globalBrowserPool
}

// GetPage 获取或创建页面（复用浏览器）
func (p *BrowserPool) GetPage(ctx context.Context) (*rod.Page, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查是否需要重新初始化
	if !p.initialized || p.browser == nil || p.page == nil {
		logrus.Info("初始化浏览器池...")
		if err := p.initialize(); err != nil {
			return nil, fmt.Errorf("初始化失败：%w", err)
		}
	}

	// 检查是否超时未使用
	if time.Since(p.lastUsed) > p.maxIdleTime {
		logrus.Info("浏览器空闲超时，重新初始化...")
		p.close()
		if err := p.initialize(); err != nil {
			return nil, fmt.Errorf("重新初始化失败：%w", err)
		}
	}

	// 更新最后使用时间
	p.lastUsed = time.Now()

	// 确保页面有效
	if _, err := p.page.Timeout(10 * time.Second).Eval(`() => 1`); err != nil {
		logrus.Warn("页面失效，重新创建...")
		p.page.Close()
		p.page = p.browser.NewPage()
	}

	return p.page, nil
}

// initialize 初始化浏览器和页面
func (p *BrowserPool) initialize() error {
	// 创建浏览器（无头模式）
	p.browser = browser.NewBrowser(true, browser.WithBinPath(configs.GetBinPath()))

	// 创建页面
	p.page = p.browser.NewPage()

	// 导航到收藏页面（预加载）
	logrus.Info("预加载收藏页面...")
	if err := p.page.Timeout(60 * time.Second).Navigate("https://www.xiaohongshu.com/user/profile/me?tab=fav&subTab=note"); err != nil {
		return fmt.Errorf("导航失败：%w", err)
	}

	// 等待页面稳定
	time.Sleep(3 * time.Second)

	p.initialized = true
	p.lastUsed = time.Now()

	logrus.Info("✅ 浏览器池初始化完成")
	return nil
}

// close 关闭浏览器
func (p *BrowserPool) close() {
	if p.browser != nil {
		p.browser.Close()
		p.browser = nil
	}
	if p.page != nil {
		p.page.Close()
		p.page = nil
	}
	p.initialized = false
}

// Close 关闭浏览器池
func (p *BrowserPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.close()
}

// GetBrowser 获取浏览器实例（向后兼容）
func GetBrowser() *headless_browser.Browser {
	return browser.NewBrowser(true, browser.WithBinPath(configs.GetBinPath()))
}

// GetBrowserWithPage 获取浏览器和页面（向后兼容）
func GetBrowserWithPage() (*headless_browser.Browser, *rod.Page, error) {
	b := GetBrowser()
	page := b.NewPage()
	return b, page, nil
}
