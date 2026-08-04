package humanize

import (
	"math"
	"math/rand"
	"time"
)

type Action string

const (
	AfterClick    Action = "after_click"
	AfterType     Action = "after_type"
	AfterNavigate Action = "after_navigate"
	BetweenScroll Action = "between_scroll"
	BeforeSubmit  Action = "before_submit"
	BeforeClick   Action = "before_click"
	Reading       Action = "reading"
	Keystroke     Action = "keystroke"
	ClickHold     Action = "click_hold"
	AfterInteract Action = "after_interact"
	PointerSettle Action = "pointer_settle"
)

type LogNormal struct {
	Mu, Sigma float64
	Min, Max  time.Duration
}

func (l LogNormal) sample(norm float64) time.Duration {
	secs := math.Exp(l.Mu + l.Sigma*norm)
	// 转换前先处理上限：secs 极大时 float64→Duration 会溢出成负值
	if l.Max > 0 && secs >= l.Max.Seconds() {
		return l.Max
	}
	d := time.Duration(secs * float64(time.Second))
	if d < l.Min {
		return l.Min
	}
	return d
}

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
		AfterClick:    {Mu: -0.92, Sigma: 0.35, Min: 150 * time.Millisecond, Max: 2 * time.Second},
		AfterType:     {Mu: -0.51, Sigma: 0.40, Min: 200 * time.Millisecond, Max: 3 * time.Second},
		AfterNavigate: {Mu: 0.41, Sigma: 0.45, Min: 600 * time.Millisecond, Max: 6 * time.Second},
		BetweenScroll: {Mu: -0.22, Sigma: 0.40, Min: 250 * time.Millisecond, Max: 3 * time.Second},
		BeforeSubmit:  {Mu: 0.0, Sigma: 0.40, Min: 400 * time.Millisecond, Max: 4 * time.Second},
		BeforeClick:   {Mu: -1.61, Sigma: 0.45, Min: 80 * time.Millisecond, Max: 1 * time.Second},
		Reading:       {Mu: -0.36, Sigma: 0.40, Min: 300 * time.Millisecond, Max: 3 * time.Second},
		Keystroke:     {Mu: -2.12, Sigma: 0.50, Min: 30 * time.Millisecond, Max: 400 * time.Millisecond},
		ClickHold:     {Mu: -2.47, Sigma: 0.33, Min: 45 * time.Millisecond, Max: 250 * time.Millisecond},
		AfterInteract: {Mu: 0.88, Sigma: 0.30, Min: 2 * time.Second, Max: 3 * time.Second},
		PointerSettle: {Mu: -1.20, Sigma: 0.35, Min: 200 * time.Millisecond, Max: 1200 * time.Millisecond},
	}
}

var defaultProvider Provider = DefaultProvider{}

func SetProvider(p Provider) {
	if p != nil {
		defaultProvider = p
	}
}
