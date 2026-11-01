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

// waitFeedPageReady 等待 feed 详情页可用：load 事件 + 关键容器出现，不要求整页 DOM 静止。
func waitFeedPageReady(page *rod.Page) {
	page.MustWaitLoad()

	if _, err := page.Timeout(feedReadyTimeout).Element(feedReadySelectors); err != nil {
		// 没等到也继续，后续 Element 各自带轮询
		logrus.Warnf("详情页关键容器未在 %s 内出现，继续尝试: %v", feedReadyTimeout, err)
	}
}
