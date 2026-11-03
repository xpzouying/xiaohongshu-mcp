package xiaohongshu

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// feedReadyTimeout 等待详情页关键容器的上限，超时不算失败。
const feedReadyTimeout = 8 * time.Second

// feedReadySelectors 详情页已出结果的判据：正常笔记容器，或 checkPageAccessible 认的错误容器。
const feedReadySelectors = ".interact-container, .note-scroller, " +
	".access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper"

// feedLoadTimeout 等待 load 事件的上限；重帖（评论数 100+）load 事件可能因挂起资源一直
// 不触发（实测 30s 都不触发，但 DOM 照常渲染），所以只给很短的宽限，真正的就绪判据
// 是下方关键容器轮询。超时不致命。
const feedLoadTimeout = 5 * time.Second

// waitFeedPageReady 等待 feed 详情页可用：load 事件 + 关键容器出现，不要求整页 DOM 静止。
// 注意：load 等待必须非致命——MustWaitLoad 在重帖上会 panic，导致评论根本发不出去。
func waitFeedPageReady(page *rod.Page) {
	if err := page.Timeout(feedLoadTimeout).WaitLoad(); err != nil {
		logrus.Warnf("页面 load 事件未在 %s 内触发，继续尝试: %v", feedLoadTimeout, err)
	}

	if _, err := page.Timeout(feedReadyTimeout).Element(feedReadySelectors); err != nil {
		// 没等到也继续，后续 Element 各自带轮询
		logrus.Warnf("详情页关键容器未在 %s 内出现，继续尝试: %v", feedReadyTimeout, err)
	}
}
