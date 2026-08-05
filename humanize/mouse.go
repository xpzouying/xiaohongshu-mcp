package humanize

import (
	"math"
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Point 视口内的一个坐标（CSS 像素）。
type Point struct {
	X, Y float64
}

// moveMouseCurved 以三次贝塞尔曲线把指针从当前位置移动到目标点，
// 模拟人手移动轨迹而非瞬时跳转。mouse 为 nil 时无法追踪起点，直接落点。
func moveMouseCurved(mouse playwright.Mouse, from, target Point) error {
	dx, dy := target.X-from.X, target.Y-from.Y
	dist := math.Hypot(dx, dy)

	if dist < 6 {
		return mouse.Move(target.X, target.Y)
	}

	steps := int(math.Round(dist / 10))
	if steps < 10 {
		steps = 10
	}
	if steps > 40 {
		steps = 40
	}

	c1, c2 := curveControlPoints(from, target, dist)
	perStep := time.Duration(5+rand.Intn(5)) * time.Millisecond

	for i := 1; i < steps; i++ {
		p := cubicBezier(from, c1, c2, target, easeInOut(float64(i)/float64(steps)))
		if err := mouse.Move(p.X, p.Y); err != nil {
			return err
		}
		time.Sleep(perStep)
	}
	return mouse.Move(target.X, target.Y)
}

func curveControlPoints(a, b Point, dist float64) (Point, Point) {
	nx, ny := -(b.Y-a.Y)/dist, (b.X-a.X)/dist
	off := dist * (0.05 + rand.Float64()*0.10)
	if rand.Intn(2) == 0 {
		off = -off
	}
	c1 := Point{X: a.X + (b.X-a.X)/3 + nx*off, Y: a.Y + (b.Y-a.Y)/3 + ny*off}
	c2 := Point{X: a.X + (b.X-a.X)*2/3 + nx*off*0.5, Y: a.Y + (b.Y-a.Y)*2/3 + ny*off*0.5}
	return c1, c2
}

func cubicBezier(p0, p1, p2, p3 Point, t float64) Point {
	u := 1 - t
	w0, w1, w2, w3 := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return Point{
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
