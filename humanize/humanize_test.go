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

	assert.Equal(t, time.Second, d.sample(0))

	assert.Less(t, d.sample(-1), d.sample(0))
	assert.Less(t, d.sample(0), d.sample(1))
}

func TestLogNormal_clamp(t *testing.T) {
	d := LogNormal{Mu: 0, Sigma: 1, Min: 500 * time.Millisecond, Max: 2 * time.Second}

	assert.Equal(t, d.Min, d.sample(-100))
	assert.Equal(t, d.Max, d.sample(100))
}

func TestLogNormal_noMax(t *testing.T) {
	d := LogNormal{Mu: 0, Sigma: 1, Min: 0, Max: 0}
	assert.Greater(t, d.sample(3), 15*time.Second)
}

func TestDefaultProvider_Timing(t *testing.T) {
	tp := DefaultProvider{}.Timing()

	for _, action := range []Action{AfterClick, AfterType, AfterNavigate, BetweenScroll, BeforeSubmit, BeforeClick, Reading, Keystroke, ClickHold, AfterInteract, PointerSettle} {
		dist, ok := tp[action]
		assert.True(t, ok, "缺少动作 %s 的时延分布", action)
		assert.Greater(t, dist.Max, dist.Min, "%s: Max 应大于 Min", action)

		mid := dist.sample(0)
		assert.GreaterOrEqual(t, mid, dist.Min, "%s: 中位样本不应小于 Min", action)
		assert.LessOrEqual(t, mid, dist.Max, "%s: 中位样本不应大于 Max", action)
	}
}

func TestDelay_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	Delay(ctx, AfterNavigate)
	assert.Less(t, time.Since(start), 100*time.Millisecond, "已取消的 ctx 应让 Delay 立即返回")
}

func TestCubicBezier_Endpoints(t *testing.T) {
	p0 := proto.Point{X: 0, Y: 0}
	p1 := proto.Point{X: 10, Y: 50}
	p2 := proto.Point{X: 90, Y: 50}
	p3 := proto.Point{X: 100, Y: 0}

	assert.Equal(t, p0, cubicBezier(p0, p1, p2, p3, 0))
	assert.Equal(t, p3, cubicBezier(p0, p1, p2, p3, 1))

	mid := cubicBezier(p0, p1, p2, p3, 0.5)
	assert.Greater(t, mid.X, 0.0)
	assert.Less(t, mid.X, 100.0)
}

func TestEaseInOut(t *testing.T) {
	assert.Equal(t, 0.0, easeInOut(0))
	assert.Equal(t, 1.0, easeInOut(1))
	assert.InDelta(t, 0.5, easeInOut(0.5), 1e-9)
	assert.Less(t, easeInOut(0.25), easeInOut(0.75))
}

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
		for i := 0; i < 200; i++ {
			got := jitterOffset(c.size)
			assert.LessOrEqual(t, math.Abs(got), c.limit, "%s: 偏移 %v 超出上限 %v", c.desc, got, c.limit)
		}
	}
}

func TestJitterInQuad_StaysInside(t *testing.T) {
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

func TestDelay_UnknownActionFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.NotPanics(t, func() {
		Delay(ctx, Action("nonexistent"))
	})
}

// TestPointerSettle_HasSufficientFloor 钉住 PointerSettle 的下限。
func TestPointerSettle_HasSufficientFloor(t *testing.T) {
	dist := DefaultProvider{}.Timing()[PointerSettle]

	assert.GreaterOrEqual(t, dist.Min, 200*time.Millisecond,
		"PointerSettle 的下限不应低于 200ms")

	// Min 是 clamp 下限，等价于最坏情况的采样
	assert.GreaterOrEqual(t, dist.sample(-5), 200*time.Millisecond,
		"极端偏小的采样也应被 clamp 到 200ms 以上")
}
