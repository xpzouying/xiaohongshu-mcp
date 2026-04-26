package xiaohongshu

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// ========== 延迟配置 ==========

var (
	reactionTime = delayConfig{400, 900}
	hoverTime    = delayConfig{150, 400}
	readTime     = delayConfig{800, 2000}
	scrollWait   = delayConfig{100, 200}
	landingWait  = delayConfig{2000, 4000}
	navWait      = delayConfig{1000, 2500}
	typeDelay    = delayConfig{30, 120}
)

// ========== 基础工具 ==========

// RandomDelay 随机等待 min~max 毫秒
func RandomDelay(minMs, maxMs int) {
	if maxMs <= minMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}
	d := time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond
	time.Sleep(d)
}

// RandomInt 返回 [min, max] 范围的随机整数
func RandomInt(min, max int) int {
	if max <= min {
		return min
	}
	return min + rand.Intn(max-min)
}

// ========== 拟人导航 ==========

// NavigateWithLanding 先访问落地页再导航到目标页，模拟自然浏览行为
func NavigateWithLanding(page *rod.Page, targetURL string, skipLanding bool) error {
	if !skipLanding {
		logrus.Debug("拟人导航: 先访问首页作为跳板")
		if err := page.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
			logrus.Warnf("导航到落地页失败: %v，直接跳转目标页", err)
		} else {
			_ = page.WaitDOMStable(time.Second, 0.1)
			RandomDelay(landingWait.min, landingWait.max)

			// 在落地页小幅度滚动，模拟浏览行为
			scrollDelta := RandomInt(100, 400)
			_, _ = page.Eval(fmt.Sprintf(`() => window.scrollBy(0, %d)`, scrollDelta))
			RandomDelay(navWait.min, navWait.max)
		}
	}

	logrus.Debugf("拟人导航: 跳转到目标页面")
	if err := page.Navigate(targetURL); err != nil {
		return err
	}
	_ = page.WaitDOMStable(time.Second, 0.1)
	RandomDelay(1200, 3000)
	return nil
}

// ========== 拟人点击 ==========

// ClickWithHumanBehavior 模拟人类点击：滚动到元素 → 鼠标悬停 → 随机延迟 → 点击 → 阅读等待
func ClickWithHumanBehavior(page *rod.Page, el *rod.Element) error {
	// 滚动到元素可见
	if err := el.ScrollIntoView(); err != nil {
		// 兜底：用 JS 滚动
		_, _ = el.Eval(`() => { try { this.scrollIntoView({behavior: 'smooth', block: 'center'}); } catch(e) {} }`)
	}

	RandomDelay(reactionTime.min, reactionTime.max)

	// 鼠标移动到元素位置
	if box, err := el.Shape(); err == nil && len(box.Quads) > 0 {
		x := float64(box.Quads[0][0]+box.Quads[0][4]) / 2
		y := float64(box.Quads[0][1]+box.Quads[0][5]) / 2
		page.Mouse.MustMoveTo(x, y)
		RandomDelay(hoverTime.min, hoverTime.max)
	}

	// 点击
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	// 点击后阅读等待
	RandomDelay(readTime.min, readTime.max)
	return nil
}

// ========== 拟人输入 ==========

// TypeWithHumanBehavior 逐字符输入文本，模拟人类打字节奏
func TypeWithHumanBehavior(el *rod.Element, text string) error {
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	RandomDelay(200, 500)

	for _, ch := range text {
		if err := el.Input(string(ch)); err != nil {
			return err
		}
		RandomDelay(typeDelay.min, typeDelay.max)
	}
	return nil
}
