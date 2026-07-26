package humanize

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// pressAndRelease 按下、停留一段采样出的时长、再抬起。
//
// rod 的 Mouse.Click 是 Down 紧接着 Up，中间没有间隔；这里按 ClickHold 的
// 分布补上停留时间。
func pressAndRelease(mouse *rod.Mouse) error {
	if err := mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	time.Sleep(defaultProvider.Timing()[ClickHold].Sample())

	return mouse.Up(proto.InputMouseButtonLeft, 1)
}

// jitterOffset 给定边长，返回 ±min(边长的 15%, 8px) 内的随机偏移。
//
// 两个上限缺一不可：按比例是为了小元素上不至于偏出去；封顶 8px 是因为宽元素
// 里真正可点的常常只是中间一小块（比如一整行里只有个图标可点），偏太远会落空。
func jitterOffset(size float64) float64 {
	limit := math.Min(math.Abs(size)*0.15, 8)
	return (rand.Float64() - 0.5) * 2 * limit
}

// jitterInQuad 在元素框内围绕 pt 取一个带小幅随机偏移的落点。
// rod 对同一元素返回的可点位置是常量，这里让它在元素内有些变化。
func jitterInQuad(pt proto.Point, q proto.DOMQuad) proto.Point {
	return proto.Point{
		X: pt.X + jitterOffset(q[4]-q[0]),
		Y: pt.Y + jitterOffset(q[5]-q[1]),
	}
}

// jitterOn 取元素外框后围绕 pt 抖动；取不到外框就原样返回。
func jitterOn(elem *rod.Element, pt proto.Point) proto.Point {
	shape, err := elem.Shape()
	if err != nil || len(shape.Quads) == 0 {
		return pt
	}
	return jitterInQuad(pt, shape.Quads[0])
}

func Click(elem *rod.Element) error {
	pt, err := elem.WaitInteractable()
	if err != nil {
		return err
	}

	mouse := elem.Page().Mouse
	if err := moveMouseCurved(mouse, jitterOn(elem, *pt)); err != nil {
		return err
	}

	if err := elem.WaitEnabled(); err != nil {
		return err
	}

	return pressAndRelease(mouse)
}

// ClickNoWait 移到元素中心再点击，跳过 rod 的 WaitInteractable 遮挡重试。
//
// 专用于 hover 浮层里的选项：浮层的悬停特性会让 WaitInteractable 误判"被遮挡"而死等
// （见搜索筛选面板）；而"从触发元素移进浮层内选项"这段移动本身恰好维持 :hover、
// 让浮层保持打开。因此这里直接取元素中心、移动过去、点击，不做遮挡检查。
// 前提：调用前浮层已打开（如已 Hover 触发元素）。
func ClickNoWait(elem *rod.Element) error {
	shape, err := elem.Shape()
	if err != nil {
		return err
	}
	if len(shape.Quads) == 0 {
		return errors.New("元素无可点击区域")
	}

	q := shape.Quads[0] // 8 个值 = 4 个角点 (x,y)；对角线中点即中心
	center := proto.Point{X: (q[0] + q[4]) / 2, Y: (q[1] + q[5]) / 2}

	mouse := elem.Page().Mouse
	if err := moveMouseCurved(mouse, jitterInQuad(center, q)); err != nil {
		return err
	}
	return pressAndRelease(mouse)
}

// MoveTo 沿曲线把指针移到指定坐标，不点击。
// 用于目标不是元素的场景，比如把指针放进某个滚动容器让滚轮作用在它上面。
func MoveTo(page *rod.Page, pt proto.Point) error {
	return moveMouseCurved(page.Mouse, pt)
}

// ClickAt 沿曲线移到指定坐标再点击。
// 用于点不到元素、只能算坐标的场景（如取元素区域内的偏移点、点击空白处）；
// 有元素可点时用 Click。
//
// 落点由调用方决定，这里不抖动——调用方挑那个坐标通常有它的理由。
func ClickAt(page *rod.Page, pt proto.Point) error {
	if err := moveMouseCurved(page.Mouse, pt); err != nil {
		return err
	}
	return pressAndRelease(page.Mouse)
}

// Type 逐字符输入文本，字间带间隔。
// 元素只在开头聚焦一次，后续插入都落在该焦点上。
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
