package humanize

import (
	"math"
	"math/rand"

	"github.com/go-rod/rod/lib/proto"
)

// Point is an alias for proto.Point for local use.
type Point = proto.Point

// SegmentCurve generates a curved path between two points using a randomly
// chosen curve family, making reverse-engineering harder.
type SegmentCurve int

const (
	CurveBezier SegmentCurve = iota
	CurveQuadBezier
	CurveSigmoid
)

// randomCurve picks a curve family at random.
func randomCurve() SegmentCurve {
	switch rand.Intn(3) {
	case 0:
		return CurveBezier
	case 1:
		return CurveQuadBezier
	default:
		return CurveSigmoid
	}
}

// generateSegment produces intermediate points for one segment of a path.
// Control points are kept between start/end so the curve does not loop back.
func generateSegment(start, end Point, steps int, roughness float64) []Point {
	curve := randomCurve()
	points := make([]Point, 0, steps)

	offset := Point{X: end.X - start.X, Y: end.Y - start.Y}
	dist := math.Hypot(offset.X, offset.Y)
	if dist < 1 {
		dist = 1
	}

	// Perpendicular unit vector for control-point displacement.
	perp := Point{X: -offset.Y / dist, Y: offset.X / dist}

	// Keep perpendicular displacement modest so the path does not make big loops.
	maxPerp := dist * roughness

	// Place control points along the forward direction but add slight curvature.
	cp1 := Point{
		X: start.X + offset.X*(0.25+rand.Float64()*0.15) + perp.X*maxPerp*(rand.Float64()*2-1),
		Y: start.Y + offset.Y*(0.25+rand.Float64()*0.15) + perp.Y*maxPerp*(rand.Float64()*2-1),
	}
	cp2 := Point{
		X: start.X + offset.X*(0.60+rand.Float64()*0.15) + perp.X*maxPerp*(rand.Float64()*2-1),
		Y: start.Y + offset.Y*(0.60+rand.Float64()*0.15) + perp.Y*maxPerp*(rand.Float64()*2-1),
	}

	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		var p Point
		switch curve {
		case CurveBezier:
			p = cubicBezier(t, start, cp1, cp2, end)
		case CurveQuadBezier:
			p = quadraticBezier(t, start, midPoint(start, end), end)
		case CurveSigmoid:
			p = sigmoidBlend(t, start, end, cp1, cp2)
		}
		points = append(points, p)
	}
	return points
}

// GeneratePath creates a non-uniform, multi-segment mouse path from start to end.
// It splits the overall movement into 2-4 segments, each with its own curve
// shape, so the path cannot be described by a single mathematical formula.
func GeneratePath(start, end Point, minSteps, maxSteps int, overshootRatio float64) []Point {
	segments := 2 + rand.Intn(3) // 2 to 4 segments
	totalSteps := minSteps + rand.Intn(maxSteps-minSteps+1)

	// Add slight overshoot to some moves. Keep it subtle so it looks like a
	// small correction rather than a large loop.
	var target Point
	if rand.Float64() < 0.20 && overshootRatio > 0 {
		offset := Point{X: end.X - start.X, Y: end.Y - start.Y}
		dist := math.Hypot(offset.X, offset.Y)
		if dist > 50 {
			dir := Point{X: offset.X / dist, Y: offset.Y / dist}
			overshoot := dist * overshootRatio * (0.3 + rand.Float64()*0.5)
			target = Point{
				X: end.X + dir.X*overshoot,
				Y: end.Y + dir.Y*overshoot,
			}
		} else {
			target = end
		}
	} else {
		target = end
	}

	waypoints := make([]Point, 0, segments+1)
	waypoints = append(waypoints, start)

	for i := 1; i < segments; i++ {
		ratio := float64(i) / float64(segments)
		base := Point{
			X: start.X + (target.X-start.X)*ratio,
			Y: start.Y + (target.Y-start.Y)*ratio,
		}
		// Random waypoint scatter proportional to remaining distance.
		// Keep scatter small so waypoints do not pull the path far off course.
		remain := Point{X: target.X - base.X, Y: target.Y - base.Y}
		remainDist := math.Hypot(remain.X, remain.Y)
		scatter := remainDist * 0.06 * (rand.Float64()*2 - 1)
		if remainDist > 0 {
			perp := Point{X: -remain.Y / remainDist, Y: remain.X / remainDist}
			base.X += perp.X * scatter
			base.Y += perp.Y * scatter
		}
		waypoints = append(waypoints, base)
	}
	waypoints = append(waypoints, target)

	stepsPerSegment := totalSteps / segments
	if stepsPerSegment < 3 {
		stepsPerSegment = 3
	}

	path := []Point{start}
	for i := 0; i < len(waypoints)-1; i++ {
		s := stepsPerSegment + rand.Intn(stepsPerSegment/3+1) - stepsPerSegment/6
		if s < 3 {
			s = 3
		}
		seg := generateSegment(waypoints[i], waypoints[i+1], s, 0.05+rand.Float64()*0.10)
		path = append(path, seg...)
	}

	// If we overshot, add a correction segment back to the real target.
	if target != end {
		corrSteps := 6 + rand.Intn(8)
		path = append(path, generateSegment(target, end, corrSteps, 0.05)...)
	}

	return path
}

func cubicBezier(t float64, p0, p1, p2, p3 Point) Point {
	u := 1 - t
	return Point{
		X: u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X,
		Y: u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y,
	}
}

func quadraticBezier(t float64, p0, p1, p2 Point) Point {
	u := 1 - t
	return Point{
		X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X,
		Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y,
	}
}

func sigmoidBlend(t float64, start, end, cp1, cp2 Point) Point {
	// Ease-in-out sigmoid: t' = t^2 / (t^2 + (1-t)^2)
	tt := t * t
	st := (1 - t) * (1 - t)
	s := tt / (tt + st)
	// Blend two linear projections with control points for curvature.
	pA := Point{
		X: start.X + (cp1.X-start.X)*s,
		Y: start.Y + (cp1.Y-start.Y)*s,
	}
	pB := Point{
		X: cp2.X + (end.X-cp2.X)*s,
		Y: cp2.Y + (end.Y-cp2.Y)*s,
	}
	return Point{
		X: pA.X + (pB.X-pA.X)*s,
		Y: pA.Y + (pB.Y-pA.Y)*s,
	}
}

func midPoint(a, b Point) Point {
	return Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}
