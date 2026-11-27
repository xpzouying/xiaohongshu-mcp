package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// 复用浏览器。
//
// 上游每个请求都冷启动一个 Chrome（约 2–3 秒）再关掉；这里改成常驻一个浏览器，每个请求只开一个标签页。
// 会触发重建的情况：cookies 文件有变化（重新登录 / 删除）、开页失败（浏览器已死）、存活超过 maxAge、
// 累计开页数超过 maxPages（防内存缓慢增长）。设置 XHS_BROWSER_REUSE=0 可回到上游的每请求新开浏览器。

const (
	sharedBrowserMaxAge   = 30 * time.Minute
	sharedBrowserMaxPages = 200
)

type sharedBrowser struct {
	mu          sync.Mutex
	b           *headless_browser.Browser
	cookieMtime time.Time
	createdAt   time.Time
	pages       int
}

var browserPool sharedBrowser

func browserReuseEnabled() bool {
	return os.Getenv("XHS_BROWSER_REUSE") != "0"
}

func cookieFileMtime() time.Time {
	st, err := os.Stat(cookies.GetCookiesFilePath())
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

func (p *sharedBrowser) closeLocked(reason string) {
	if p.b == nil {
		return
	}
	logrus.Infof("关闭共享浏览器（%s；已开 %d 页，存活 %s）", reason, p.pages, time.Since(p.createdAt).Round(time.Second))
	func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warnf("关闭共享浏览器出错: %v", r)
			}
		}()
		p.b.Close()
	}()
	p.b = nil
}

func (p *sharedBrowser) staleReasonLocked() string {
	switch {
	case p.b == nil:
		return ""
	case !cookieFileMtime().Equal(p.cookieMtime):
		return "cookies 文件已变化"
	case time.Since(p.createdAt) > sharedBrowserMaxAge:
		return "超过最大存活时间"
	case p.pages >= sharedBrowserMaxPages:
		return "开页数达到上限"
	}
	return ""
}

func tryNewPage(b *headless_browser.Browser) (page *rod.Page, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return b.NewPage(), nil
}

// newPageLocked 在共享浏览器上开一页；浏览器已死（开页 panic）则重建一次再试。
func (p *sharedBrowser) newPageLocked() (*headless_browser.Browser, *rod.Page) {
	for attempt := 0; attempt < 2; attempt++ {
		if p.b == nil {
			p.b = newBrowser()
			p.cookieMtime = cookieFileMtime()
			p.createdAt = time.Now()
			p.pages = 0
			logrus.Info("共享浏览器已启动")
		}
		page, err := tryNewPage(p.b)
		if err == nil {
			p.pages++
			return p.b, page
		}
		logrus.Warnf("共享浏览器开页失败，重建: %v", err)
		p.closeLocked("开页失败")
	}
	panic("共享浏览器重建后仍无法开页")
}

// acquirePage 取一个可用页面。复用关闭时退化为上游行为：新开一个浏览器。
func acquirePage() (*headless_browser.Browser, *rod.Page) {
	if !browserReuseEnabled() {
		b := newBrowser()
		return b, b.NewPage()
	}
	p := &browserPool
	p.mu.Lock()
	defer p.mu.Unlock()
	if reason := p.staleReasonLocked(); reason != "" {
		p.closeLocked(reason)
	}
	return p.newPageLocked()
}

// releasePage 请求结束：关掉页面；不复用时连浏览器一起关。
func releasePage(b *headless_browser.Browser, page *rod.Page) {
	if err := page.Close(); err != nil {
		logrus.Debugf("关闭页面出错: %v", err)
	}
	if !browserReuseEnabled() {
		b.Close()
	}
}

// resetSharedBrowser 主动关闭共享浏览器（删除 cookies 后调用，避免继续用旧登录态）。
func resetSharedBrowser() {
	p := &browserPool
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked("手动重置")
}
