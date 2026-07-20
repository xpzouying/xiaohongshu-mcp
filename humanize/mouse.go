package humanize

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func moveMouseCurved(mouse *rod.Mouse, target proto.Point) error {
	start := mouse.Position()
	dx, dy := target.X-start.X, target.Y-start.Y
	dist := math.Hypot(dx, dy)

	// 距离极近：直接到位
	if dist < 6 {
		return mouse.MoveTo(target)
	}

	steps := int(math.Round(dist / 10))
	if steps < 10 {
		steps = 10
	}
	if steps > 40 {
		steps = 40
	}

	c1, c2 := curveControlPoints(start, target, dist)
	perStep := time.Duration(5+rand.Intn(5)) * time.Millisecond

	i := 0
	return mouse.MoveAlong(func() (proto.Point, bool) {
		i++
		if i >= steps {
			return target, true // 最后一步精确落到目标
		}
		p := cubicBezier(start, c1, c2, target, easeInOut(float64(i)/float64(steps)))
		time.Sleep(perStep)
		return p, false
	})
}

func curveControlPoints(a, b proto.Point, dist float64) (proto.Point, proto.Point) {
	nx, ny := -(b.Y-a.Y)/dist, (b.X-a.X)/dist  // 单位垂直向量
	off := dist * (0.05 + rand.Float64()*0.10) // 弧度偏移：距离的 5%~15%
	if rand.Intn(2) == 0 {
		off = -off
	}
	c1 := proto.Point{X: a.X + (b.X-a.X)/3 + nx*off, Y: a.Y + (b.Y-a.Y)/3 + ny*off}
	c2 := proto.Point{X: a.X + (b.X-a.X)*2/3 + nx*off*0.5, Y: a.Y + (b.Y-a.Y)*2/3 + ny*off*0.5}
	return c1, c2
}

func cubicBezier(p0, p1, p2, p3 proto.Point, t float64) proto.Point {
	u := 1 - t
	w0, w1, w2, w3 := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return proto.Point{
		X: w0*p0.X + w1*p1.X + w2*p2.X + w3*p3.X,
		Y: w0*p0.Y + w1*p1.Y + w2*p2.Y + w3*p3.Y,
	}
}

func easeInOut(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return 1 - math.Pow(-2*t+2, 2)/2
}
