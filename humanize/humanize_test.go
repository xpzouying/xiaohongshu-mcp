package humanize

import (
	"context"
	"math"
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

// TestDefaultProvider_Timing 校验默认时延表：动作齐全、参数自洽。
func TestDefaultProvider_Timing(t *testing.T) {
	tp := DefaultProvider{}.Timing()

	for _, action := range []Action{AfterClick, AfterType, AfterNavigate, BetweenScroll, BeforeSubmit, BeforeClick, Reading, Keystroke, ClickHold} {
		dist, ok := tp[action]
		assert.True(t, ok, "缺少动作 %s 的时延分布", action)
		assert.Greater(t, dist.Max, dist.Min, "%s: Max 应大于 Min", action)

		// 中位样本应落在 [Min, Max] 内（参数合理性）
		mid := dist.sample(0)
		assert.GreaterOrEqual(t, mid, dist.Min, "%s: 中位样本不应小于 Min", action)
		assert.LessOrEqual(t, mid, dist.Max, "%s: 中位样本不应大于 Max", action)
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

// TestJitterOffset_Bounds 偏移幅度受两个上限约束：按边长比例、且有绝对上限。
func TestJitterOffset_Bounds(t *testing.T) {
	cases := []struct {
		size  float64
		limit float64
		desc  string
	}{
		{size: 20, limit: 3, desc: "小元素按比例取"},
		{size: 600, limit: 8, desc: "宽元素封顶"},
		{size: -40, limit: 6, desc: "负边长按绝对值处理"},
		{size: 0, limit: 0, desc: "零边长不偏移"},
	}

	for _, c := range cases {
		for i := 0; i < 200; i++ { // 采样够多次，覆盖到边界附近
			got := jitterOffset(c.size)
			assert.LessOrEqual(t, math.Abs(got), c.limit, "%s: 偏移 %v 超出上限 %v", c.desc, got, c.limit)
		}
	}
}

// TestJitterInQuad_StaysInside 偏移后的落点必须仍在元素框内。
func TestJitterInQuad_StaysInside(t *testing.T) {
	// 100x40 的框，左上角 (10,20)；四角顺序：左上、右上、右下、左下
	q := proto.DOMQuad{10, 20, 110, 20, 110, 60, 10, 60}
	center := proto.Point{X: 60, Y: 40}

	for i := 0; i < 500; i++ {
		p := jitterInQuad(center, q)
		assert.GreaterOrEqual(t, p.X, 10.0)
		assert.LessOrEqual(t, p.X, 110.0)
		assert.GreaterOrEqual(t, p.Y, 20.0)
		assert.LessOrEqual(t, p.Y, 60.0)
	}
}

// TestDelay_UnknownActionFallback 未知动作应回退而非 panic 或零等待。
func TestDelay_UnknownActionFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.NotPanics(t, func() {
		Delay(ctx, Action("nonexistent"))
	})
}
