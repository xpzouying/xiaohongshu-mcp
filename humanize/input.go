package humanize

import (
	"context"
	"errors"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func Click(elem *rod.Element) error {
	pt, err := elem.WaitInteractable()
	if err != nil {
		return err
	}

	mouse := elem.Page().Mouse
	if err := moveMouseCurved(mouse, *pt); err != nil {
		return err
	}

	if err := elem.WaitEnabled(); err != nil {
		return err
	}

	return mouse.Click(proto.InputMouseButtonLeft, 1)
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
	if err := moveMouseCurved(mouse, center); err != nil {
		return err
	}
	return mouse.Click(proto.InputMouseButtonLeft, 1)
}

// Type 逐字符输入文本，字间带间隔。
func Type(ctx context.Context, elem *rod.Element, text string) error {
	dist := defaultProvider.Timing()[Keystroke]

	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := elem.Input(string(r)); err != nil {
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
