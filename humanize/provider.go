// Package humanize 封装交互行为的统一原语（延迟、输入、点击）。
//
// 约定：业务代码统一调 humanize.*，不直接调 time.Sleep / rod 的 Mouse.MoveTo /
// elem.Input 等裸原语。
//
// 行为参数走 Provider 接口：DefaultProvider 提供一套静态默认，可用 SetProvider 替换。
package humanize

import (
	"math"
	"math/rand"
	"time"
)

// Action 标识一类停顿场景。
type Action string

const (
	AfterClick    Action = "after_click"    // 点击后
	AfterType     Action = "after_type"     // 输入后
	AfterNavigate Action = "after_navigate" // 页面跳转/加载后
	BetweenScroll Action = "between_scroll" // 连续滚动之间
	BeforeSubmit  Action = "before_submit"  // 提交前
	BeforeClick   Action = "before_click"   // 移到元素上、点击前
	Reading       Action = "reading"        // 浏览内容时的驻留
	Keystroke     Action = "keystroke"      // 逐字符输入的字间间隔
)

// LogNormal 描述一个右偏时延分布：ln(时延/秒) ~ Normal(Mu, Sigma)，采样后 clamp 到 [Min, Max]。
type LogNormal struct {
	Mu, Sigma float64       // 对 ln(秒) 的正态参数；median = exp(Mu)
	Min, Max  time.Duration // clamp 边界（Max<=0 表示不设上限）
}

// sample 是纯函数核心：给定一个标准正态样本 norm，返回 clamp 后的时延。
// 拆出来是为了可确定性单测（喂已知 norm 值）。
func (l LogNormal) sample(norm float64) time.Duration {
	secs := math.Exp(l.Mu + l.Sigma*norm)
	// 先在"秒"量级上处理上限：secs 极大时直接 float64→Duration 会溢出成负值、
	// 破坏 clamp，所以在转换前就拦掉。
	if l.Max > 0 && secs >= l.Max.Seconds() {
		return l.Max
	}
	d := time.Duration(secs * float64(time.Second))
	if d < l.Min {
		return l.Min
	}
	return d
}

// Sample 用全局 rng 采样一个时延（Go 1.20+ 全局 rand 已自动播种且并发安全）。
func (l LogNormal) Sample() time.Duration {
	return l.sample(rand.NormFloat64())
}

type TimingProfile map[Action]LogNormal

type Provider interface {
	Timing() TimingProfile
}

type DefaultProvider struct{}

func (DefaultProvider) Timing() TimingProfile {
	return TimingProfile{
		// median ≈ exp(Mu) 秒
		AfterClick:    {Mu: -0.92, Sigma: 0.35, Min: 150 * time.Millisecond, Max: 2 * time.Second},       // ~0.4s
		AfterType:     {Mu: -0.51, Sigma: 0.40, Min: 200 * time.Millisecond, Max: 3 * time.Second},       // ~0.6s
		AfterNavigate: {Mu: 0.41, Sigma: 0.45, Min: 600 * time.Millisecond, Max: 6 * time.Second},        // ~1.5s
		BetweenScroll: {Mu: -0.22, Sigma: 0.40, Min: 250 * time.Millisecond, Max: 3 * time.Second},       // ~0.8s
		BeforeSubmit:  {Mu: 0.0, Sigma: 0.40, Min: 400 * time.Millisecond, Max: 4 * time.Second},         // ~1.0s
		BeforeClick:   {Mu: -1.61, Sigma: 0.45, Min: 80 * time.Millisecond, Max: 1 * time.Second},        // ~0.2s
		Reading:       {Mu: -0.36, Sigma: 0.40, Min: 300 * time.Millisecond, Max: 3 * time.Second},       // ~0.7s
		Keystroke:     {Mu: -2.12, Sigma: 0.50, Min: 30 * time.Millisecond, Max: 400 * time.Millisecond}, // ~120ms/字
	}
}

// defaultProvider 是包级默认 Provider。一账号一进程，进程级 Provider 即该账号的行为参数。
// 启动时可用 SetProvider 注入替换，业务代码零改动。
var defaultProvider Provider = DefaultProvider{}

// SetProvider 注入行为参数 Provider。传 nil 忽略，保证任何时候都有可用的 Provider。
func SetProvider(p Provider) {
	if p != nil {
		defaultProvider = p
	}
}
