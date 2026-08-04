package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

// browserSession 常驻单浏览器 + 进程内串行 page lease。
//
// 设计目标（效率优先）：
//   - 不人为 sleep / 不限速：串行只表示「同一时刻最多一个 tab 作业」，不排队空等间隔
//   - 关 page 不关 browser，避免每请求冷启 Chromium
//   - 登录扫码可长期占用 lease（锁持有直到 release），期间其它请求会阻塞等待——符合单账号单通道
//
// 不做：user-data-dir 持久 profile。headless_browser.Close 会 launcher.Cleanup 删除
// UserDataDir，挂持久目录会在停服时被抹掉；待上游支持 KeepUserDataDir 再接。
type browserSession struct {
	mu sync.Mutex
	b  *headless_browser.Browser
}

var sharedBrowser = &browserSession{}

// riskStreak 连续「像风控墙」的失败次数；成功清零。
// 仅用于坏信号熔断（立刻返回错误），不 sleep。
var riskStreak atomic.Int32

func riskStreakLimit() int {
	// 默认 3；XHS_RISK_STREAK_LIMIT=0 关闭熔断
	v := strings.TrimSpace(os.Getenv("XHS_RISK_STREAK_LIMIT"))
	if v == "" {
		return 3
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 3
	}
	return n
}

// isRiskSignal 只认明确的墙/验证码类文案，不把普通 timeout 当墙（timeout 更像反爬或卡顿，由调用方换主 Chrome）。
func isRiskSignal(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	keys := []string{
		"captcha", "验证码", "滑块", "access denied", "访问异常",
		"环境异常", "网络繁忙", "账号异常", "安全验证",
	}
	for _, k := range keys {
		if strings.Contains(s, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func observeOpResult(err error) {
	if err == nil {
		riskStreak.Store(0)
		return
	}
	if !isRiskSignal(err) {
		return
	}
	n := riskStreak.Add(1)
	logrus.Warnf("risk signal observed (streak=%d): %v", n, err)
}

func checkRiskCircuit() error {
	lim := riskStreakLimit()
	if lim == 0 {
		return nil
	}
	if int(riskStreak.Load()) >= lim {
		return fmt.Errorf("risk circuit open: consecutive risk signals >= %d; stop MCP batch and use main Chrome or re-login (set XHS_RISK_STREAK_LIMIT=0 to disable)", lim)
	}
	return nil
}

func (s *browserSession) ensureLocked() error {
	if s.b != nil {
		return nil
	}
	logrus.Info("starting shared browser instance")
	opts := []browser.Option{
		browser.WithFingerprintSeed(configs.FingerprintSeed()),
		browser.WithProxy(configs.Proxy()),
	}
	if bin := configs.ChromeBin(); bin != "" {
		opts = append(opts, browser.WithBinPath(bin))
	}
	s.b = browser.NewBrowser(configs.IsHeadless(), opts...)
	return nil
}

func (s *browserSession) resetLocked() {
	if s.b == nil {
		return
	}
	logrus.Info("closing shared browser instance")
	// headless_browser.Close 会 Cleanup 临时 user-data；常驻会话下只在失效/停服时调用。
	func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warnf("browser close panic: %v", r)
			}
		}()
		s.b.Close()
	}()
	s.b = nil
}

func (s *browserSession) newPageLocked() (page *rod.Page, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("new page panic: %v", r)
			page = nil
		}
	}()
	page = s.b.NewPage()
	return page, nil
}

// Do 串行租约：持锁 → 开 page → fn → 关 page → 放锁。不引入人为间隔。
func (s *browserSession) Do(fn func(*rod.Page) error) error {
	if err := checkRiskCircuit(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLocked(); err != nil {
		return err
	}

	page, err := s.newPageLocked()
	if err != nil {
		logrus.Warnf("new page failed, recreating browser: %v", err)
		s.resetLocked()
		if err = s.ensureLocked(); err != nil {
			return err
		}
		page, err = s.newPageLocked()
		if err != nil {
			return err
		}
	}
	defer func() { _ = page.Close() }()

	err = fn(page)
	observeOpResult(err)

	// 页级灾难：下次重建浏览器
	if err != nil && (strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "target closed") ||
		strings.Contains(err.Error(), "browser has been closed")) {
		s.resetLocked()
	}
	return err
}

// Lease 长期占用（扫码登录）：调用方必须 release；持锁期间其它 Do 阻塞。
func (s *browserSession) Lease() (*rod.Page, func(), error) {
	if err := checkRiskCircuit(); err != nil {
		return nil, nil, err
	}

	s.mu.Lock()
	if err := s.ensureLocked(); err != nil {
		s.mu.Unlock()
		return nil, nil, err
	}

	page, err := s.newPageLocked()
	if err != nil {
		logrus.Warnf("lease new page failed, recreating browser: %v", err)
		s.resetLocked()
		if err = s.ensureLocked(); err != nil {
			s.mu.Unlock()
			return nil, nil, err
		}
		page, err = s.newPageLocked()
		if err != nil {
			s.mu.Unlock()
			return nil, nil, err
		}
	}

	var once sync.Once
	release := func() {
		once.Do(func() {
			if page != nil {
				_ = page.Close()
			}
			s.mu.Unlock()
		})
	}
	return page, release, nil
}

// Invalidate 丢弃常驻浏览器（删 cookie / 需换登录态后调用）。
func (s *browserSession) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
	riskStreak.Store(0)
}

// Close 进程退出时关闭常驻浏览器。
func (s *browserSession) Close() {
	s.Invalidate()
}
