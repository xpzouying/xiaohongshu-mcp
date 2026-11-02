package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
)

// 浏览器并发闸门。
//
// 每次 newBrowser() 都是一整个 Chromium 进程组，而在此之前**全项目没有任何并发控制**：
// 18 个 MCP 工具和 20 条 REST 端点共用一个 service 单例，net/http 一请求一 goroutine，
// 两个请求撞上就是两套完整浏览器一起把容器推向 OOM——而 OOM 杀掉的是整个容器，
// 不是那一个请求。
//
// 并发度取 2，依据是在目标机器（2 核 / 容器上限 1200MB）上的实测。注意口径：必须读
// cgroup 的 memory.current，不能把 11 个 chrome 进程的 ps RSS 相加——后者会把进程间
// 共享的二进制映射和 zygote 的 CoW 页重复计算 11 次，实测高估近一倍（996MB vs 528MB），
// 照那个数字算会误判成"并发上限是 1"。
//
//	并发   峰值内存   单请求耗时   吞吐        余量
//	 1      528MB      9.3s       6.5 次/分   672MB
//	 2      736MB      13s        9.2 次/分   464MB   ← 取这个
//	 3      975MB      18.5s      9.7 次/分   225MB
//
// 3 的吞吐只比 2 多 5%（2 核 CPU 已经争抢），延迟却翻倍，余量还掉到 225MB——
// 评论多的详情页能轻松吃掉这点余量。
//
// 用 XHS_BROWSER_CONCURRENCY 覆盖；换机器（尤其是改了 mem_limit 或 CPU 数）要重新实测。
const defaultBrowserConcurrency = 2

// browserAcquireTimeout 排队等名额的上限。
//
// 短排队 + 明确失败，绝不长排队：单请求约 13 秒，排在第 2、3 位也就几十秒；
// 等过这个数说明后面已经堆积，与其让调用方干等到自己超时（MCP 客户端通常 60 秒左右），
// 不如立刻给个说得清的错误。
const browserAcquireTimeout = 60 * time.Second

var (
	browserSem     chan struct{}
	browserSemOnce sync.Once
)

func browserConcurrency() int {
	if v := os.Getenv("XHS_BROWSER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		logrus.Warnf("XHS_BROWSER_CONCURRENCY=%q 不是正整数，回退到 %d", v, defaultBrowserConcurrency)
	}
	return defaultBrowserConcurrency
}

func browserSlots() chan struct{} {
	browserSemOnce.Do(func() {
		n := browserConcurrency()
		browserSem = make(chan struct{}, n)
		logrus.Infof("浏览器并发上限: %d", n)
	})
	return browserSem
}

// leasedBrowser 给浏览器实例配一张名额票，Close 时归还。
//
// 内嵌 *headless_browser.Browser 是刻意的：调用方在 b 上只用到 Close 和 NewPage
// （已 grep 核对，19 个调用点无一例外），靠 Go 的方法提升，全部调用点无需改动，
// 唯一被拦截的就是 Close。
type leasedBrowser struct {
	*headless_browser.Browser
	release func()
}

// Close 关浏览器并归还名额。名额一定要还，所以即便关闭本身出问题也不能跳过。
func (b *leasedBrowser) Close() {
	defer b.release()
	b.Browser.Close()
}

// acquireBrowserSlot 取一张名额票，取不到就是明确失败。
func acquireBrowserSlot() {
	sem := browserSlots()

	select {
	case sem <- struct{}{}:
		return
	default:
	}

	// 满了，说明正在排队——记一笔，好在日志里看出压力
	logrus.Warnf("浏览器名额已满（上限 %d），排队中", cap(sem))
	start := time.Now()

	select {
	case sem <- struct{}{}:
		logrus.Infof("排队 %s 后取得浏览器名额", time.Since(start).Round(time.Millisecond))
	case <-time.After(browserAcquireTimeout):
		// 这里 panic 而不是返回 error，是为了不改动 19 个调用点的签名；
		// MCP 侧有 withPanicRecovery、REST 侧有 gin.Recovery()，都会转成正常的错误响应。
		panic(fmt.Sprintf("浏览器繁忙：等待 %s 仍未取得名额（并发上限 %d），请稍后重试",
			browserAcquireTimeout, cap(sem)))
	}
}

// newLeasedBrowser 取名额、建浏览器，并保证建失败时名额不会漏掉。
func newLeasedBrowser(build func() *headless_browser.Browser) *leasedBrowser {
	acquireBrowserSlot()

	var once sync.Once
	release := func() { once.Do(func() { <-browserSlots() }) }

	// build 可能 panic——browser.NewBrowser 在内置浏览器不可用时是主动 panic 的。
	// 名额必须还回去，否则连续几次之后所有请求都会永久卡在 acquireBrowserSlot 上。
	defer func() {
		if r := recover(); r != nil {
			release()
			panic(r)
		}
	}()

	return &leasedBrowser{Browser: build(), release: release}
}
