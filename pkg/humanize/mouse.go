package humanize

import (
	"errors"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var debugMouse bool

func init() {
	v := os.Getenv("HUMANIZE_DEBUG")
	debugMouse = v == "1" || v == "true" || v == "yes"
}

// Mouse provides human-like mouse operations.
type Mouse struct {
	page        *rod.Page
	cfg         Config
	initialized bool
}

// NewMouse creates a new humanized mouse wrapper.
func NewMouse(page *rod.Page, cfg Config) *Mouse {
	return &Mouse{page: page, cfg: cfg}
}

// initPosition moves the cursor from the rod default (0,0) to a plausible
// starting point inside the viewport. This is done once per Mouse instance so
// subsequent movements do not look like long flights from the screen corner.
// The movement itself is humanized so the cursor does not teleport.
func (m *Mouse) initPosition() error {
	if m.initialized {
		return nil
	}
	vp, err := m.viewport()
	if err != nil {
		return err
	}
	center := Point{
		X: vp.width/2 + (rand.Float64()*2-1)*vp.width*0.15,
		Y: vp.height/2 + (rand.Float64()*2-1)*vp.height*0.15,
	}

	// Mark initialized before calling moveTo to avoid recursion.
	m.initialized = true
	if err := m.moveTo(center); err != nil {
		m.initialized = false
		return err
	}
	return nil
}

// InitPosition eagerly moves the cursor from the rod default (0,0) to a
// plausible starting point. Call this right after a page is created so the
// first real interaction does not start from the detectable (0,0) origin.
func (m *Mouse) InitPosition() error {
	return m.initPosition()
}

// Move moves the cursor to target with a realistic, non-deterministic path.
// If the target lies outside the current viewport, the page is scrolled first
// so that the destination is rendered before the cursor moves there.
func (m *Mouse) Move(target Point) error {
	// target is in page-absolute coordinates. Scroll it into view if it is
	// outside the current viewport, then convert to viewport-relative
	// coordinates before moving the cursor (rod.Mouse.MoveTo expects
	// viewport-relative coordinates).
	if err := m.scrollToVisible(target); err != nil {
		return err
	}
	vp, err := m.viewport()
	if err != nil {
		return err
	}
	return m.moveTo(Point{
		X: target.X - vp.scrollX,
		Y: target.Y - vp.scrollY,
	})
}

// moveTo performs the actual cursor movement without any extra scrolling.
func (m *Mouse) moveTo(target Point) error {
	if debugMouse {
		m.ensureDebugOverlay()
	}

	// Start from a plausible position instead of rod's default (0,0).
	if err := m.initPosition(); err != nil {
		return err
	}

	start := m.page.Mouse.Position()
	straightDist := math.Hypot(target.X-start.X, target.Y-start.Y)

	// Derive step count from distance so short moves finish quickly and long
	// moves still have enough points to look natural.
	desiredSteps := int(straightDist / m.cfg.Mouse.StepDistance)
	if desiredSteps < m.cfg.Mouse.MinSteps {
		desiredSteps = m.cfg.Mouse.MinSteps
	}
	if desiredSteps > m.cfg.Mouse.MaxSteps {
		desiredSteps = m.cfg.Mouse.MaxSteps
	}

	path := GeneratePath(start, target, desiredSteps, desiredSteps, m.cfg.Mouse.OvershootRatio)

	// Base speed with variance.
	speed := m.cfg.Mouse.MoveSpeedPxPerSec * (1 + (rand.Float64()*2-1)*m.cfg.Mouse.SpeedVariance)

	// Total distance for velocity profile normalization.
	totalDist := 0.0
	prev := start
	for _, p := range path {
		totalDist += math.Hypot(p.X-prev.X, p.Y-prev.Y)
		prev = p
	}

	// Accelerate-then-fine-tune velocity profile: slow at the start, fast in
	// the middle, and slow again near the target. The profile is a sine hump
	// scaled so its average over [0,1] is 1.0, keeping the overall move time
	// comparable to the constant-speed baseline.
	const velocityFloor = 0.3
	velocityAmp := (1.0 - velocityFloor) * math.Pi / 2

	cumulativeDist := 0.0
	last := start
	for i, p := range path {
		// Inject jitter.
		if rand.Float64() < m.cfg.Mouse.JitterProbability {
			p = jitter(p, m.cfg.Mouse.JitterRadius)
		}

		// Distance-based step duration with ease-in-out acceleration.
		dist := math.Hypot(p.X-last.X, p.Y-last.Y)
		cumulativeDist += dist

		var stepDuration time.Duration
		if totalDist > 0 {
			t := cumulativeDist / totalDist
			// Use the midpoint of the step for smoother transitions.
			tMid := t - dist/(2*totalDist)
			if tMid < 0 {
				tMid = 0
			}
			velocity := velocityFloor + velocityAmp*math.Sin(math.Pi*tMid)
			effectiveSpeed := speed * velocity
			stepDuration = time.Duration(float64(time.Second) * dist / effectiveSpeed)
		} else {
			stepDuration = time.Duration(float64(time.Second) * dist / speed)
		}
		if stepDuration < 1*time.Millisecond {
			stepDuration = 1 * time.Millisecond
		}

		// Keep the event density high enough to look like a real mouse
		// (typical browser refresh rate is 60-120Hz). If the planned step is
		// too long, subdivide it into smaller micro-steps.
		const maxStepDuration = 16 * time.Millisecond
		subSteps := 1
		if stepDuration > maxStepDuration {
			subSteps = int(math.Ceil(float64(stepDuration) / float64(maxStepDuration)))
		}

		for j := 0; j < subSteps; j++ {
			ratio := float64(j+1) / float64(subSteps)
			subP := Point{
				X: last.X + (p.X-last.X)*ratio,
				Y: last.Y + (p.Y-last.Y)*ratio,
			}

			if err := m.page.Mouse.MoveTo(subP); err != nil {
				return err
			}

			if debugMouse {
				_ = m.tracePoint(subP.X, subP.Y, i == 0 && j == 0)
			}

			time.Sleep(stepDuration / time.Duration(subSteps))
		}

		last = p
	}
	return nil
}

// Click scrolls the element into view, moves to its center with random offset, and clicks.
func (m *Mouse) Click(el *rod.Element) error {
	// Scroll the target element into view first; its on-screen position may
	// change after scrolling (fixed/sticky elements or layout shifts).
	if err := m.ScrollIntoView(el); err != nil {
		return err
	}
	// Re-calculate the target after scrolling, because fixed/sticky elements
	// move with the viewport and the old page-absolute coordinates are stale.
	target, err := elementTarget(el)
	if err != nil {
		return err
	}
	if err := m.moveTo(target); err != nil {
		return err
	}

	// Human pause before clicking.
	time.Sleep(randDuration(80*time.Millisecond, 350*time.Millisecond))

	if err := m.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	time.Sleep(randDuration(40*time.Millisecond, 120*time.Millisecond))
	if err := m.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	return nil
}

// ClickNoScroll performs a human-like click without scrolling the element into
// view first. Use it when the target is already known to be visible (e.g.
// sticky/fixed elements) to avoid the overhead or infinite loops caused by
// ScrollIntoView.
func (m *Mouse) ClickNoScroll(el *rod.Element) error {
	target, err := elementTarget(el)
	if err != nil {
		return err
	}
	if err := m.moveTo(target); err != nil {
		return err
	}

	// Human pause before clicking.
	time.Sleep(randDuration(80*time.Millisecond, 350*time.Millisecond))

	if err := m.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	time.Sleep(randDuration(40*time.Millisecond, 120*time.Millisecond))
	if err := m.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	return nil
}

// Scroll scrolls by deltaY (and optionally deltaX) in human-like increments.
func (m *Mouse) Scroll(deltaX, deltaY float64) error {
	if deltaY == 0 && deltaX == 0 {
		return nil
	}

	steps := int(math.Abs(deltaY)/80) + int(math.Abs(deltaX)/80) + 3
	steps += rand.Intn(5)
	stepX := deltaX / float64(steps)
	stepY := deltaY / float64(steps)

	for i := 0; i < steps; i++ {
		if err := m.page.Mouse.Scroll(stepX, stepY, 1); err != nil {
			return err
		}
		// Variable scroll speed: faster at start, slower near end.
		base := 30 + float64(i)*5
		time.Sleep(time.Duration(base+rand.Float64()*40) * time.Millisecond)
	}
	time.Sleep(randDuration(200*time.Millisecond, 600*time.Millisecond))
	return nil
}

// ScrollIntoView scrolls the page just enough to bring the element into the
// visible viewport using humanized wheel events. It avoids JS scrollIntoView
// which can be detected by pages observing synchronous scroll/layout changes.
// The element only needs to be visible (with a small margin); it is not forced
// to the center, so sticky/fixed elements do not cause infinite scrolling.
func (m *Mouse) ScrollIntoView(el *rod.Element) error {
	const maxAttempts = 12
	const margin = 80
	for i := 0; i < maxAttempts; i++ {
		shape, err := el.Shape()
		if err != nil {
			return err
		}
		if len(shape.Quads) == 0 {
			return errors.New("element has no content quads")
		}

		// CDP quads are viewport-relative, so no scroll offset is needed.
		q := shape.Quads[0]
		var minX, maxX, minY, maxY float64
		for j := 0; j < q.Len(); j++ {
			x := q[j*2]
			y := q[j*2+1]
			if j == 0 || x < minX {
				minX = x
			}
			if j == 0 || x > maxX {
				maxX = x
			}
			if j == 0 || y < minY {
				minY = y
			}
			if j == 0 || y > maxY {
				maxY = y
			}
		}

		vp, err := m.viewport()
		if err != nil {
			return err
		}

		var deltaX, deltaY float64
		if maxX < margin {
			deltaX = maxX - vp.width + margin
		} else if minX > vp.width-margin {
			deltaX = minX - margin
		}
		if maxY < margin {
			deltaY = maxY - vp.height + margin
		} else if minY > vp.height-margin {
			deltaY = minY - margin
		}

		if deltaX == 0 && deltaY == 0 {
			return nil
		}

		if err := m.Scroll(deltaX, deltaY); err != nil {
			return err
		}
		time.Sleep(randDuration(80*time.Millisecond, 200*time.Millisecond))
	}
	return nil
}

// Hover scrolls the element into view, moves to it, and pauses briefly.
func (m *Mouse) Hover(el *rod.Element) error {
	if err := m.ScrollIntoView(el); err != nil {
		return err
	}
	target, err := elementTarget(el)
	if err != nil {
		return err
	}
	if err := m.moveTo(target); err != nil {
		return err
	}
	time.Sleep(randDuration(150*time.Millisecond, 500*time.Millisecond))
	return nil
}

func (m *Mouse) scrollRandom() error {
	deltaY := randomSign() * (m.cfg.Mouse.ScrollMin + rand.Float64()*(m.cfg.Mouse.ScrollMax-m.cfg.Mouse.ScrollMin))
	return m.page.Mouse.Scroll(0, deltaY, 1)
}

type viewport struct {
	scrollX, scrollY float64
	width, height    float64
}

func (m *Mouse) viewport() (viewport, error) {
	obj, err := m.page.Eval(`() => ({
		scrollX: window.scrollX,
		scrollY: window.scrollY,
		innerWidth: window.innerWidth,
		innerHeight: window.innerHeight
	})`)
	if err != nil {
		return viewport{}, err
	}
	res, err := m.page.ObjectToJSON(obj)
	if err != nil {
		return viewport{}, err
	}
	return viewport{
		scrollX: res.Get("scrollX").Num(),
		scrollY: res.Get("scrollY").Num(),
		width:   res.Get("innerWidth").Num(),
		height:  res.Get("innerHeight").Num(),
	}, nil
}

// scrollToVisible scrolls the page so that target is rendered inside the
// viewport with a comfortable margin. It is a no-op if target is already visible.
func (m *Mouse) scrollToVisible(target Point) error {
	vp, err := m.viewport()
	if err != nil {
		return err
	}

	const margin = 80
	var deltaX, deltaY float64

	if target.X < vp.scrollX+margin {
		deltaX = target.X - vp.scrollX - vp.width/2
	} else if target.X > vp.scrollX+vp.width-margin {
		deltaX = target.X - vp.scrollX - vp.width/2
	}

	if target.Y < vp.scrollY+margin {
		deltaY = target.Y - vp.scrollY - vp.height/2
	} else if target.Y > vp.scrollY+vp.height-margin {
		deltaY = target.Y - vp.scrollY - vp.height/2
	}

	if deltaX == 0 && deltaY == 0 {
		return nil
	}

	// Add a small random offset so the target does not always land at the exact
	// center of the viewport.
	deltaX += (rand.Float64()*2 - 1) * 30
	deltaY += (rand.Float64()*2 - 1) * 30

	return m.Scroll(deltaX, deltaY)
}

// ensureDebugOverlay injects a canvas to visualize mouse movement.
// It is only called when HUMANIZE_DEBUG=1.
func (m *Mouse) ensureDebugOverlay() {
	_, _ = m.page.Eval(`() => {
		if (window.__humanizeCanvas) return;
		const canvas = document.createElement('canvas');
		canvas.id = '__humanize_mouse_trace';
		canvas.width = window.innerWidth;
		canvas.height = window.innerHeight;
		canvas.style.cssText = 'position:fixed;top:0;left:0;pointer-events:none;z-index:2147483647;';
		document.body.appendChild(canvas);
		window.__humanizeCanvas = canvas;
		window.__humanizeCtx = canvas.getContext('2d');
		window.addEventListener('resize', () => {
			canvas.width = window.innerWidth;
			canvas.height = window.innerHeight;
		});
	}`)
}

// tracePoint draws a dot on the debug overlay at (x, y).
func (m *Mouse) tracePoint(x, y float64, first bool) error {
	_, err := m.page.Eval(`(x, y, first) => {
		const ctx = window.__humanizeCtx;
		if (!ctx) return;
		ctx.fillStyle = first ? 'rgba(0, 255, 0, 0.8)' : 'rgba(255, 0, 0, 0.5)';
		ctx.beginPath();
		ctx.arc(x, y, first ? 5 : 3, 0, Math.PI * 2);
		ctx.fill();
	}`, x, y, first)
	return err
}

func elementTarget(el *rod.Element) (Point, error) {
	shape, err := el.Shape()
	if err != nil {
		return Point{}, err
	}
	if len(shape.Quads) == 0 {
		return Point{}, errors.New("element has no content quads")
	}
	q := shape.Quads[0]

	// Compute the bounding box of the quad to handle arbitrary vertex order.
	var minX, maxX, minY, maxY float64
	for i := 0; i < q.Len(); i++ {
		x := q[i*2]
		y := q[i*2+1]
		if i == 0 || x < minX {
			minX = x
		}
		if i == 0 || x > maxX {
			maxX = x
		}
		if i == 0 || y < minY {
			minY = y
		}
		if i == 0 || y > maxY {
			maxY = y
		}
	}
	center := Point{
		X: (minX + maxX) / 2,
		Y: (minY + maxY) / 2,
	}
	width := maxX - minX
	height := maxY - minY

	// CDP DOM.getContentQuads returns coordinates relative to the viewport, and
	// rod.Mouse.MoveTo also expects viewport-relative coordinates, so no scroll
	// offset conversion is needed.
	// Random offset within central 60% of element.
	return Point{
		X: center.X + width*(rand.Float64()*0.3-0.15),
		Y: center.Y + height*(rand.Float64()*0.3-0.15),
	}, nil
}

func jitter(p Point, radius float64) Point {
	angle := rand.Float64() * 2 * math.Pi
	d := rand.Float64() * radius
	return Point{
		X: p.X + math.Cos(angle)*d,
		Y: p.Y + math.Sin(angle)*d,
	}
}

func randomSign() float64 {
	if rand.Intn(2) == 0 {
		return -1
	}
	return 1
}
