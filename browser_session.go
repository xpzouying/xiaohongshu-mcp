package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

// browserSession 常驻单 Camoufox + 进程内串行 page lease。
//
// 设计目标（效率优先）：
//   - 不人为 sleep / 不限速：串行只表示「同一时刻最多一个 tab 作业」，不排队空等间隔
//   - 关 page 不关浏览器（持久化上下文常驻），避免每请求冷启
//   - 登录扫码可长期占用 lease（锁持有直到 release），期间其它请求阻塞等待
type browserSession struct {
	mu sync.Mutex
	b  *browser.Browser
}

var sharedBrowser = &browserSession{}

// riskStreak 连续「像风控墙」的失败次数；成功清零。仅用于坏信号熔断（立刻返回错误），不 sleep。
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

// isRiskSignal 只认明确的墙/验证码类文案，不把普通 timeout 当墙。
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
		return fmt.Errorf("risk circuit open: consecutive risk signals >= %d; stop MCP batch and re-login (set XHS_RISK_STREAK_LIMIT=0 to disable)", lim)
	}
	return nil
}

func (s *browserSession) ensureLocked() error {
	if s.b != nil {
		return nil
	}
	logrus.Info("starting shared camoufox instance")
	opts := []browser.Option{
		browser.WithFingerprintSeed(configs.FingerprintSeed()),
		browser.WithProxy(configs.Proxy()),
	}
	if bin := configs.CamoufoxBin(); bin != "" {
		opts = append(opts, browser.WithBinPath(bin))
	}
	b, err := browser.NewBrowser(configs.IsHeadless(), opts...)
	if err != nil {
		return err
	}
	s.b = b
	return nil
}

func (s *browserSession) resetLocked() {
	if s.b == nil {
		return
	}
	logrus.Info("closing shared camoufox instance")
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

func (s *browserSession) newPageLocked() (playwright.Page, error) {
	return s.b.NewPage()
}

// Do 串行租约：持锁 → 开 page → fn → 关 page → 放锁。不引入人为间隔。
func (s *browserSession) Do(fn func(playwright.Page) error) error {
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
	if err != nil && isBrowserDead(err) {
		s.resetLocked()
	}
	return err
}

// Lease 长期占用（扫码登录）：调用方必须 release；持锁期间其它 Do 阻塞。
func (s *browserSession) Lease() (playwright.Page, func(), error) {
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

// isBrowserDead 判断错误是否为浏览器/连接层灾难（下次重建浏览器）。
func isBrowserDead(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "target closed") ||
		strings.Contains(s, "browser has been closed") ||
		strings.Contains(s, "Target closed")
}

// current 返回当前常驻浏览器实例（可能为 nil），用于登录后导出 cookie 等。
// 只在确知浏览器已启动（如扫码流程持有 lease）时使用。
func (s *browserSession) current() *browser.Browser {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b
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
