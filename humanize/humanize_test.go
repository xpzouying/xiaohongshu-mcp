package humanize

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/assert"
)

func TestLogNormal_sample(t *testing.T) {
	d := LogNormal{Mu: 0, Sigma: 0.5, Min: 100 * time.Millisecond, Max: 10 * time.Second}

	// norm=0 → exp(0)=1s（未触边界）
	assert.Equal(t, time.Second, d.sample(0))

	// 单调递增：norm 越大时延越长
	assert.Less(t, d.sample(-1), d.sample(0))
	assert.Less(t, d.sample(0), d.sample(1))
}

func TestLogNormal_clamp(t *testing.T) {
	d := LogNormal{Mu: 0, Sigma: 1, Min: 500 * time.Millisecond, Max: 2 * time.Second}

	// exp(-100)≈0 → 下限
	assert.Equal(t, d.Min, d.sample(-100))
	// exp(100) 极大 → 上限（且不因 float64→Duration 溢出而出错）
	assert.Equal(t, d.Max, d.sample(100))
}

func TestLogNormal_noMax(t *testing.T) {
	// Max<=0 表示不设上限
	d := LogNormal{Mu: 0, Sigma: 1, Min: 0, Max: 0}
	// exp(3)≈20s，不被 clamp
	assert.Greater(t, d.sample(3), 15*time.Second)
}

// TestDefaultProvider_Timing 校验默认时延表：动作齐全、参数自洽（Min<Max、median 落在区间）。
func TestDefaultProvider_Timing(t *testing.T) {
	tp := DefaultProvider{}.Timing()

	for _, action := range []Action{AfterClick, AfterType, AfterNavigate, BetweenScroll, BeforeSubmit, BeforeClick, Reading} {
		dist, ok := tp[action]
		assert.True(t, ok, "缺少动作 %s 的时延分布", action)
		assert.Greater(t, dist.Max, dist.Min, "%s: Max 应大于 Min", action)

		// median = sample(0)，应落在 [Min, Max] 内（分布参数合理性）
		median := dist.sample(0)
		assert.GreaterOrEqual(t, median, dist.Min, "%s: median 不应小于 Min", action)
		assert.LessOrEqual(t, median, dist.Max, "%s: median 不应大于 Max", action)
	}
}

// TestDelay_RespectsContextCancel 校验 Delay 能被 ctx 取消，不傻等满时长。
func TestDelay_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	start := time.Now()
	Delay(ctx, AfterNavigate) // AfterNavigate 最短也有 600ms，取消后应几乎立即返回
	assert.Less(t, time.Since(start), 100*time.Millisecond, "已取消的 ctx 应让 Delay 立即返回")
}

// TestCubicBezier_Endpoints 贝塞尔曲线两端点应精确落在 p0/p3。
func TestCubicBezier_Endpoints(t *testing.T) {
	p0 := proto.Point{X: 0, Y: 0}
	p1 := proto.Point{X: 10, Y: 50}
	p2 := proto.Point{X: 90, Y: 50}
	p3 := proto.Point{X: 100, Y: 0}

	assert.Equal(t, p0, cubicBezier(p0, p1, p2, p3, 0)) // t=0 → 起点
	assert.Equal(t, p3, cubicBezier(p0, p1, p2, p3, 1)) // t=1 → 终点

	// 中段应落在包围盒内（不跑飞）
	mid := cubicBezier(p0, p1, p2, p3, 0.5)
	assert.Greater(t, mid.X, 0.0)
	assert.Less(t, mid.X, 100.0)
}

// TestEaseInOut 缓动函数：端点固定、中点对称、单调不减。
func TestEaseInOut(t *testing.T) {
	assert.Equal(t, 0.0, easeInOut(0))
	assert.Equal(t, 1.0, easeInOut(1))
	assert.InDelta(t, 0.5, easeInOut(0.5), 1e-9) // 中点对称
	assert.Less(t, easeInOut(0.25), easeInOut(0.75))
}

// TestDelay_UnknownActionFallback 未知动作应回退而非 panic 或零等待。
func TestDelay_UnknownActionFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.NotPanics(t, func() {
		Delay(ctx, Action("nonexistent"))
	})
}
