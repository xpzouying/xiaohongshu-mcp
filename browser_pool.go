package main

import (
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

// 启用持久 user-data-dir 时，同一目录同时只能有一个 Chrome 进程（SingletonLock）。
// 进程内单例浏览器 + 互斥锁串行化所有工具调用，避免死锁。
var (
	browserMu     sync.Mutex
	sharedBrowser *headless_browser.Browser
)

// acquireBrowser 获取浏览器实例。
// 当 user-data-dir 未设置时，返回新建的临时浏览器（调用方必须 defer Close）。
// 当 user-data-dir 已设置时，返回进程内共享浏览器（调用方必须 defer releaseBrowser）。
func acquireBrowser() *headless_browser.Browser {
	if configs.UserDataDir() == "" {
		// 未启用持久 profile：保持原有每次新建行为
		return newBrowser()
	}

	// 启用持久 profile：使用共享浏览器 + 锁
	browserMu.Lock()
	if sharedBrowser == nil {
		sharedBrowser = newBrowser()
		logrus.Info("shared browser started with user-data-dir")
	}
	return sharedBrowser
}

// releaseBrowser 释放浏览器实例。
// 当 user-data-dir 未设置时，关闭临时浏览器。
// 当 user-data-dir 已设置时，仅释放锁，保持浏览器热态。
func releaseBrowser(b *headless_browser.Browser) {
	if configs.UserDataDir() == "" {
		// 未启用持久 profile：关闭临时浏览器
		b.Close()
		return
	}

	// 启用持久 profile：仅释放锁，不关闭共享浏览器
	browserMu.Unlock()
}

// resetSharedBrowser 关闭并重置共享浏览器（删 cookie / 浏览器异常后使用）。
// 仅在启用 user-data-dir 时有效。
func resetSharedBrowser() {
	if configs.UserDataDir() == "" {
		// 未启用持久 profile：无共享浏览器，无需重置
		return
	}

	browserMu.Lock()
	defer browserMu.Unlock()
	if sharedBrowser != nil {
		sharedBrowser.Close()
		sharedBrowser = nil
		logrus.Info("shared browser reset")
	}
}
