// Package ratelimit 提供防封号速率限制。
//
// 三层策略：
//   - < 80% 容量 → 正常执行，响应附带用量
//   - >= 80% 容量 → 执行但提醒，操作间自动加延迟
//   - >= 100% 容量 → 返回错误，除非 force=true
package ratelimit

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Config 速率限制配置
type Config struct {
	MaxPerHour    int           // 每小时最大操作数，默认 30
	CooldownBase  time.Duration // 操作间基础间隔，默认 3s
	WarnThreshold float64       // 提醒阈值（比例），默认 0.8
	SlowThreshold float64       // 慢速模式阈值（比例），默认 0.9
}

func DefaultConfig() Config {
	return Config{
		MaxPerHour:    30,
		CooldownBase:  3 * time.Second,
		WarnThreshold: 0.8,
		SlowThreshold: 0.9,
	}
}

// Info 速率限制状态
type Info struct {
	Used          int    `json:"used"`
	Limit         int    `json:"limit"`
	Remaining     int    `json:"remaining"`
	WindowSeconds int    `json:"window_seconds"`
	ResetUnix     int64  `json:"reset_unix"`
	Warning       string `json:"warning,omitempty"`
	RetryAfter    *int   `json:"retry_after,omitempty"`
}

// Limiter 速率限制器（goroutine-safe）
type Limiter struct {
	mu      sync.Mutex
	cfg     Config
	buckets map[int64]*bucket
}

type bucket struct {
	count     int
	lastTouch time.Time
}

// New 创建速率限制器
func New(cfg Config) *Limiter {
	return &Limiter{
		cfg:     cfg,
		buckets: make(map[int64]*bucket),
	}
}

func hourKey(t time.Time) int64 {
	return t.Truncate(time.Hour).Unix()
}

// Check 检查当前是否允许操作。
// 返回 (info, canProceed, error)
func (l *Limiter) Check(force bool) (Info, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	hk := hourKey(now)
	b, exists := l.buckets[hk]
	if !exists {
		b = &bucket{}
		l.buckets[hk] = b
	}
	l.cleanup(now)

	limit := l.cfg.MaxPerHour
	if limit <= 0 {
		limit = 30
	}
	used := b.count
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	windowStart := time.Unix(hk, 0)
	resetUnix := windowStart.Add(time.Hour).Unix()

	info := Info{
		Used:          used,
		Limit:         limit,
		Remaining:     remaining,
		WindowSeconds: 3600,
		ResetUnix:     resetUnix,
	}

	ratio := float64(used) / float64(limit)

	if ratio >= 1.0 {
		retryAfter := int(resetUnix - now.Unix())
		if retryAfter < 0 {
			retryAfter = 0
		}
		info.RetryAfter = &retryAfter
		info.Warning = fmt.Sprintf("已达每小时 %d 次上限（防封保护），建议 %d 秒后重试", limit, retryAfter)
		if force {
			info.Warning = fmt.Sprintf("[强制继续] %s", info.Warning)
			return info, true, nil
		}
		return info, false, nil
	}

	if ratio >= l.cfg.SlowThreshold {
		info.Warning = fmt.Sprintf("已使用 %d/%d 次额度（%.0f%%），操作将自动减速", used, limit, ratio*100)
	} else if ratio >= l.cfg.WarnThreshold {
		info.Warning = fmt.Sprintf("已使用 %d/%d 次额度（%.0f%%），请注意操作频率", used, limit, ratio*100)
	}

	return info, true, nil
}

// Record 记录一次操作
func (l *Limiter) Record() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	hk := hourKey(now)
	b, exists := l.buckets[hk]
	if !exists {
		b = &bucket{}
		l.buckets[hk] = b
	}
	b.count++
	b.lastTouch = now
}

// WaitDuration 计算操作前需要等待的时间（含随机偏移）
func (l *Limiter) WaitDuration(info Info) time.Duration {
	ratio := float64(info.Used) / float64(info.Limit)
	base := l.cfg.CooldownBase
	if base <= 0 {
		base = 3 * time.Second
	}

	if ratio >= 1.0 && info.RetryAfter != nil {
		return time.Duration(*info.RetryAfter) * time.Second
	}

	if ratio >= l.cfg.SlowThreshold && l.cfg.SlowThreshold < 1.0 {
		factor := 3.0 + rand.Float64()*3.0
		return time.Duration(float64(base) * factor)
	}

	if ratio >= l.cfg.WarnThreshold {
		factor := 1.5 + rand.Float64()*1.5
		return time.Duration(float64(base) * factor)
	}

	jitter := time.Duration(float64(base) * (0.5 + rand.Float64()))
	return base + jitter - time.Duration(float64(base)*0.5)
}

// Reset 重置（测试用）
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[int64]*bucket)
}

func (l *Limiter) cleanup(now time.Time) {
	cutoff := now.Add(-2 * time.Hour).Unix()
	for hk, b := range l.buckets {
		if hk < cutoff || b.lastTouch.Before(now.Add(-2*time.Hour)) {
			delete(l.buckets, hk)
		}
	}
}

func (l *Limiter) String() string {
	info, _, _ := l.Check(false)
	ratio := math.Round(float64(info.Used)/float64(info.Limit)*100) / 100
	resetAt := time.Unix(info.ResetUnix, 0)
	return fmt.Sprintf("ratelimit[used=%d/%d (%.0f%%), reset=%s]",
		info.Used, info.Limit, ratio*100, resetAt.Format("15:04"))
}
