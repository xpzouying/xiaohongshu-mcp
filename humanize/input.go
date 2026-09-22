package humanize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func pressAndRelease(mouse *rod.Mouse) error {
	if err := mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	time.Sleep(defaultProvider.Timing()[ClickHold].Sample())

	return mouse.Up(proto.InputMouseButtonLeft, 1)
}

func jitterOffset(size float64) float64 {
	limit := math.Min(math.Abs(size)*0.15, 8)
	return (rand.Float64() - 0.5) * 2 * limit
}

func jitterInQuad(pt proto.Point, q proto.DOMQuad) proto.Point {
	return proto.Point{
		X: pt.X + jitterOffset(q[4]-q[0]),
		Y: pt.Y + jitterOffset(q[5]-q[1]),
	}
}

func jitterOn(elem *rod.Element, pt proto.Point) proto.Point {
	shape, err := elem.Shape()
	if err != nil || len(shape.Quads) == 0 {
		return pt
	}
	return jitterInQuad(pt, shape.Quads[0])
}

func ensurePointInViewport(page *rod.Page, pt proto.Point) error {
	res, err := page.Eval(`() => JSON.stringify([window.innerWidth, window.innerHeight])`)
	if err != nil {
		return err
	}

	var size []float64
	if json.Unmarshal([]byte(res.Value.Str()), &size) != nil || len(size) != 2 {
		return nil
	}

	if pt.X < 0 || pt.Y < 0 || pt.X > size[0] || pt.Y > size[1] {
		return fmt.Errorf("落点 (%.0f,%.0f) 在视口 %.0fx%.0f 之外", pt.X, pt.Y, size[0], size[1])
	}
	return nil
}

// minOpacity 不透明度低于此值的元素不作为点击目标。
const minOpacity = 0.1

// effectiveOpacityJS 算元素自身连同各级祖先的不透明度乘积；
// 链上任意一级 display:none 或 visibility:hidden 都直接算 0。
//
// 只看元素自身的 opacity 不够：祖先透明时子元素仍报 1。
const effectiveOpacityJS = `() => {
	let v = 1;
	for (let n = this; n && n.nodeType === 1; n = n.parentElement) {
		const s = getComputedStyle(n);
		if (s.display === 'none' || s.visibility === 'hidden') {
			return 0;
		}
		const o = parseFloat(s.opacity);
		if (!isNaN(o)) {
			v *= o;
		}
	}
	return v;
}`

// Visible 判断元素是否可见到足以作为点击目标。取不到样式时按可见处理，
// 宁可放过也不要挡掉正常元素。
func Visible(elem *rod.Element) bool {
	res, err := elem.Eval(effectiveOpacityJS)
	if err != nil {
		return true
	}
	return res.Value.Num() >= minOpacity
}

// 不用 document.elementFromPoint：结果不稳定
func ensureClickable(elem *rod.Element, pt proto.Point) error {
	if err := ensurePointInViewport(elem.Page(), pt); err != nil {
		return err
	}

	if !Visible(elem) {
		return errors.New("元素当前不可命中")
	}
	return nil
}

func Click(elem *rod.Element) error {
	pt, err := elem.WaitInteractable()
	if err != nil {
		return err
	}

	target := jitterOn(elem, *pt)
	if err := ensureClickable(elem, target); err != nil {
		return err
	}

	mouse := elem.Page().Mouse
	if err := moveMouseCurved(mouse, target); err != nil {
		return err
	}

	Delay(elem.Page().GetContext(), PointerSettle)

	if err := elem.WaitEnabled(); err != nil {
		return err
	}

	return pressAndRelease(mouse)
}

// ClickNoWait 跳过 WaitInteractable 的遮挡重试，用于它会误判而死等的场景。
func ClickNoWait(elem *rod.Element) error {
	shape, err := elem.Shape()
	if err != nil {
		return err
	}
	if len(shape.Quads) == 0 {
		return errors.New("元素无可点击区域")
	}

	q := shape.Quads[0]
	center := proto.Point{X: (q[0] + q[4]) / 2, Y: (q[1] + q[5]) / 2}

	target := jitterInQuad(center, q)
	if err := ensureClickable(elem, target); err != nil {
		return err
	}

	mouse := elem.Page().Mouse
	if err := moveMouseCurved(mouse, target); err != nil {
		return err
	}
	return pressAndRelease(mouse)
}

func MoveTo(page *rod.Page, pt proto.Point) error {
	return moveMouseCurved(page.Mouse, pt)
}

func Hover(elem *rod.Element) error {
	shape, err := elem.Shape()
	if err != nil {
		return err
	}
	if len(shape.Quads) == 0 {
		return errors.New("元素无可悬停区域")
	}

	q := shape.Quads[0]
	center := proto.Point{X: (q[0] + q[4]) / 2, Y: (q[1] + q[5]) / 2}

	target := jitterInQuad(center, q)
	if err := ensureClickable(elem, target); err != nil {
		return err
	}

	return moveMouseCurved(elem.Page().Mouse, target)
}

func ClickAt(page *rod.Page, pt proto.Point) error {
	if err := ensurePointInViewport(page, pt); err != nil {
		return err
	}
	if err := moveMouseCurved(page.Mouse, pt); err != nil {
		return err
	}
	return pressAndRelease(page.Mouse)
}

func Type(ctx context.Context, elem *rod.Element, text string) error {
	dist := defaultProvider.Timing()[Keystroke]

	if err := elem.Focus(); err != nil {
		return err
	}
	if err := elem.WaitEnabled(); err != nil {
		return err
	}
	if err := elem.WaitWritable(); err != nil {
		return err
	}

	page := elem.Page().Context(ctx)

	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := page.InsertText(string(r)); err != nil {
			return err
		}

		t := time.NewTimer(dist.Sample())
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		}
	}
	return nil
}
