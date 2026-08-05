package humanize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

// pressAndRelease 按下再松开左键，中间保留一个拟人化的按住时长。
func pressAndRelease(mouse playwright.Mouse) error {
	if err := mouse.Down(); err != nil {
		return err
	}
	time.Sleep(defaultProvider.Timing()[ClickHold].Sample())
	return mouse.Up()
}

func jitterOffset(size float64) float64 {
	limit := math.Min(math.Abs(size)*0.15, 8)
	return (rand.Float64() - 0.5) * 2 * limit
}

// elemRect 读取元素的首个可视盒；无盒视为不可点。
func elemRect(elem playwright.ElementHandle) (x, y, w, h float64, err error) {
	box, err := elem.BoundingBox()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if box == nil || box.Width <= 0 || box.Height <= 0 {
		return 0, 0, 0, 0, errors.New("元素无可点击区域")
	}
	return box.X, box.Y, box.Width, box.Height, nil
}

// jitterInBox 在元素盒内对目标点加抖动，落点不固定几何中心。
func jitterInBox(pt Point, x, y, w, h float64) Point {
	return Point{
		X: pt.X + jitterOffset(w),
		Y: pt.Y + jitterOffset(h),
	}
}

func boxCenter(x, y, w, h float64) Point {
	return Point{X: x + w/2, Y: y + h/2}
}

func ensurePointInViewport(page playwright.Page, pt Point) error {
	res, err := page.Evaluate(`() => JSON.stringify([window.innerWidth, window.innerHeight])`)
	if err != nil {
		return err
	}
	s, _ := res.(string)
	var size []float64
	if json.Unmarshal([]byte(s), &size) != nil || len(size) != 2 {
		return nil
	}
	if pt.X < 0 || pt.Y < 0 || pt.X > size[0] || pt.Y > size[1] {
		return fmt.Errorf("落点 (%.0f,%.0f) 在视口 %.0fx%.0f 之外", pt.X, pt.Y, size[0], size[1])
	}
	return nil
}

// ensureClickable 校验落点在视口内且元素不是 visibility:hidden。
// 不用 elementFromPoint：结果不稳定。
func ensureClickable(page playwright.Page, elem playwright.ElementHandle, pt Point) error {
	if err := ensurePointInViewport(page, pt); err != nil {
		return err
	}
	res, err := elem.Evaluate(`() => getComputedStyle(this).visibility`)
	if err != nil {
		return nil
	}
	if s, _ := res.(string); s == "hidden" {
		return errors.New("元素当前不可命中")
	}
	return nil
}

// Click 等元素可交互后，拟人化移动到盒内随机落点并按下松开。
func Click(elem playwright.ElementHandle) error {
	page, err := ownerPage(elem)
	if err != nil {
		return err
	}
	// 等元素进入稳定且可命中状态
	if _, err := elem.WaitForSelector(":scope", playwright.ElementHandleWaitForSelectorOptions{
		State: playwright.WaitForSelectorStateVisible,
	}); err != nil {
		return err
	}

	x, y, w, h, err := elemRect(elem)
	if err != nil {
		return err
	}
	target := jitterInBox(boxCenter(x, y, w, h), x, y, w, h)
	if err := ensureClickable(page, elem, target); err != nil {
		return err
	}

	mouse := page.Mouse()
	if err := moveMouseCurved(mouse, Point{X: x, Y: y}, target); err != nil {
		return err
	}

	if ok, err := elem.IsEnabled(); err == nil && !ok {
		return errors.New("元素不可用")
	}
	return pressAndRelease(mouse)
}

// ClickNoWait 跳过「可交互」等待，用于 hover 浮层等遮挡会被误判而死等的场景。
func ClickNoWait(elem playwright.ElementHandle) error {
	page, err := ownerPage(elem)
	if err != nil {
		return err
	}
	x, y, w, h, err := elemRect(elem)
	if err != nil {
		return err
	}
	target := jitterInBox(boxCenter(x, y, w, h), x, y, w, h)
	if err := ensureClickable(page, elem, target); err != nil {
		return err
	}
	mouse := page.Mouse()
	if err := moveMouseCurved(mouse, Point{X: x, Y: y}, target); err != nil {
		return err
	}
	return pressAndRelease(mouse)
}

// MoveTo 把指针拟人化移动到指定视口坐标。
func MoveTo(page playwright.Page, pt Point) error {
	return moveMouseCurved(page.Mouse(), Point{X: pt.X, Y: pt.Y}, pt)
}

// Hover 拟人化移动到元素盒内随机落点（不点击）。
func Hover(elem playwright.ElementHandle) error {
	page, err := ownerPage(elem)
	if err != nil {
		return err
	}
	x, y, w, h, err := elemRect(elem)
	if err != nil {
		return err
	}
	target := jitterInBox(boxCenter(x, y, w, h), x, y, w, h)
	if err := ensureClickable(page, elem, target); err != nil {
		return err
	}
	return moveMouseCurved(page.Mouse(), Point{X: x, Y: y}, target)
}

// ClickAt 拟人化移动到指定视口坐标并点击。
func ClickAt(page playwright.Page, pt Point) error {
	if err := ensurePointInViewport(page, pt); err != nil {
		return err
	}
	mouse := page.Mouse()
	if err := moveMouseCurved(mouse, pt, pt); err != nil {
		return err
	}
	return pressAndRelease(mouse)
}

// Type 逐字符输入，带拟人化击键间隔。
func Type(ctx context.Context, elem playwright.ElementHandle, text string) error {
	dist := defaultProvider.Timing()[Keystroke]

	if err := elem.Focus(); err != nil {
		return err
	}
	if ok, err := elem.IsEnabled(); err == nil && !ok {
		return errors.New("元素不可用")
	}
	if ok, err := elem.IsEditable(); err == nil && !ok {
		return errors.New("元素不可编辑")
	}

	page, err := ownerPage(elem)
	if err != nil {
		return err
	}
	kb := page.Keyboard()

	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := kb.InsertText(string(r)); err != nil {
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
