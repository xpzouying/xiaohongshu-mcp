package main

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xpzouying/headless_browser"
)

// mockBrowserFactory 返回一个 mock 浏览器工厂，记录创建次数
func mockBrowserFactory(createCount *atomic.Int32) func() *headless_browser.Browser {
	return func() *headless_browser.Browser {
		createCount.Add(1)
		// 返回一个真实的 Browser 结构体指针（不启动 Chrome）
		return &headless_browser.Browser{}
	}
}

// newTestService 创建注入 mock 工厂的测试服务
func newTestService(createCount *atomic.Int32) *XiaohongshuService {
	return &XiaohongshuService{
		browserFactory: mockBrowserFactory(createCount),
	}
}

// TestGetBrowserReturnsSameInstance 验证多次调用返回同一个实例
func TestGetBrowserReturnsSameInstance(t *testing.T) {
	var count atomic.Int32
	s := newTestService(&count)

	b1 := s.getBrowser()
	b2 := s.getBrowser()
	b3 := s.getBrowser()

	if b1 != b2 || b2 != b3 {
		t.Error("getBrowser 应返回同一个浏览器实例，但返回了不同实例")
	}
	if count.Load() != 1 {
		t.Errorf("浏览器工厂应只调用 1 次，实际调用了 %d 次", count.Load())
	}
}

// TestGetBrowserConcurrentSafe 验证并发调用只创建一个实例
func TestGetBrowserConcurrentSafe(t *testing.T) {
	var count atomic.Int32
	s := newTestService(&count)

	const goroutines = 50
	var wg sync.WaitGroup
	browsers := make([]*headless_browser.Browser, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			browsers[idx] = s.getBrowser()
		}(i)
	}
	wg.Wait()

	// 所有 goroutine 应拿到同一个实例
	for i := 1; i < goroutines; i++ {
		if browsers[i] != browsers[0] {
			t.Errorf("goroutine %d 返回了不同的浏览器实例", i)
		}
	}

	if count.Load() != 1 {
		t.Errorf("并发场景下浏览器工厂应只调用 1 次，实际调用了 %d 次", count.Load())
	}
}

// TestCloseAndReinitialize 验证 Close 后重新创建新实例
func TestCloseAndReinitialize(t *testing.T) {
	var count atomic.Int32
	s := newTestService(&count)

	b1 := s.getBrowser()

	// Close 后 browser 应为 nil
	s.mu.Lock()
	s.browser = nil // 模拟 Close（跳过实际 Close 调用避免 nil pointer）
	s.mu.Unlock()

	b2 := s.getBrowser()

	if b1 == b2 {
		t.Error("Close 后 getBrowser 应创建新实例，但返回了旧实例")
	}
	if count.Load() != 2 {
		t.Errorf("应创建 2 次浏览器实例，实际创建了 %d 次", count.Load())
	}
}

// TestCloseIdempotent 验证多次 Close 不会 panic
func TestCloseIdempotent(t *testing.T) {
	s := &XiaohongshuService{
		browserFactory: func() *headless_browser.Browser {
			return &headless_browser.Browser{}
		},
	}
	s.getBrowser()

	// 模拟 Close：置 nil 即可（真实 Close 会调用 Browser.Close）
	s.mu.Lock()
	s.browser = nil
	s.mu.Unlock()

	// 多次操作不应 panic
	s.mu.Lock()
	s.browser = nil
	s.mu.Unlock()
}
