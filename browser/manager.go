package browser

import (
	"sync"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

var (
	globalManager *Manager
	once          sync.Once
)

// Manager 浏览器生命周期管理器
// 维护一个持久浏览器实例，每次操作通过 NewPage 创建新标签页
type Manager struct {
	mu       sync.Mutex
	browser  *Browser
	headless bool
	opts     []Option
}

// InitManager 初始化全局浏览器管理器（服务启动时调用）
func InitManager(headless bool, opts ...Option) *Manager {
	once.Do(func() {
		globalManager = &Manager{
			headless: headless,
			opts:     opts,
		}
		b := NewBrowser(headless, opts...)
		globalManager.browser = b
		logrus.Info("浏览器管理器已初始化")
	})
	return globalManager
}

// GetManager 获取全局浏览器管理器
func GetManager() *Manager {
	return globalManager
}

// NewPage 创建新标签页，浏览器不可用时自动重启
func (m *Manager) NewPage() *rod.Page {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser == nil || !m.checkAlive() {
		logrus.Warn("浏览器实例不可用，正在重新启动...")
		m.browser = NewBrowser(m.headless, m.opts...)
		logrus.Info("浏览器已重新启动")
	}

	return m.browser.NewPage()
}

// Close 关闭浏览器
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser != nil {
		m.browser.Close()
		m.browser = nil
		logrus.Info("浏览器已关闭")
	}
}

// checkAlive 检查浏览器是否存活（通过获取页面列表判断）
func (m *Manager) checkAlive() bool {
	if m.browser == nil || m.browser.browser == nil {
		return false
	}

	_, err := m.browser.browser.Pages()
	return err == nil
}
