// Modified 2026-10-30 by Marvelooous: add bounded navigation that waits for the required UI element.
// SPDX-License-Identifier: Apache-2.0

package xiaohongshu

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

const pageReadyTimeout = 90 * time.Second

// navigateToElement waits for the UI needed by the action, rather than the
// window load event, which can be held open by unrelated images and frames.
// The caller supplies a page with a bounded context.
func navigateToElement(page *rod.Page, url, selector string) error {
	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("navigate to page: %w", err)
	}
	if _, err := page.Element(selector); err != nil {
		return fmt.Errorf("wait for page UI %q: %w", selector, err)
	}
	return nil
}

// Existing feed-detail behavior is preserved by the login/navigation fix.
const feedReadyTimeout = 8 * time.Second

const feedReadySelectors = ".interact-container, .note-scroller, " +
	".access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper"

func waitFeedPageReady(page *rod.Page) {
	page.MustWaitLoad()
	if _, err := page.Timeout(feedReadyTimeout).Element(feedReadySelectors); err != nil {
		logrus.Warnf("详情页关键容器未在 %s 内出现，继续尝试: %v", feedReadyTimeout, err)
	}
}
