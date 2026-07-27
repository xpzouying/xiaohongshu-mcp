package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

const (
	screenshotTimeout  = 90 * time.Second
	pageLoadWait       = 2 * time.Second
	scrollWait         = 2 * time.Second
	expandWait         = 500 * time.Millisecond
	scrollStep         = 500
	maxScrollAttempts  = 10
)

// ScreenshotFeed captures a post page screenshot including comments section.
func (s *XiaohongshuService) ScreenshotFeed(ctx context.Context, feedID, xsecToken string) (*ScreenshotResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	url := fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_search", feedID, xsecToken)
	logrus.Infof("截图帖子: %s", url)

	page = page.Context(ctx).Timeout(screenshotTimeout)
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(pageLoadWait)

	// Dismiss popups
	dismissPopupButtons(page)

	// Scroll to bottom incrementally until page stops growing
	scrollToBottom(page)

	// Expand collapsed reply threads
	expandReplyButtons(page)
	time.Sleep(scrollWait)

	// Scroll again after expanding
	scrollToBottom(page)

	// Take screenshot
	png, err := page.Screenshot(true, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		return nil, fmt.Errorf("截图失败: %w", err)
	}

	screenshots := []string{base64.StdEncoding.EncodeToString(png)}
	logrus.Infof("截图完成，共 %d 张", len(screenshots))

	return &ScreenshotResponse{FeedID: feedID, Screenshots: screenshots}, nil
}

// dismissPopupButtons finds and clicks common popup dismiss buttons.
func dismissPopupButtons(page *rod.Page) {
	dismissKeywords := []string{"好的", "我知道了"}
	buttons, err := page.Elements("button")
	if err != nil {
		return
	}
	for _, btn := range buttons {
		text, err := btn.Text()
		if err != nil {
			continue
		}
		for _, keyword := range dismissKeywords {
			if strings.Contains(text, keyword) {
				if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
					logrus.Debugf("关闭弹窗按钮失败: %v", err)
				}
				time.Sleep(expandWait)
				break
			}
		}
	}
}

// scrollToBottom scrolls the page incrementally until no more content loads.
func scrollToBottom(page *rod.Page) {
	for i := 0; i < maxScrollAttempts; i++ {
		prevHeight, err := getPageHeight(page)
		if err != nil {
			break
		}
		if err := page.Mouse.Scroll(0, float64(scrollStep), 3); err != nil {
			logrus.Debugf("滚动失败: %v", err)
			break
		}
		time.Sleep(scrollWait)
		currHeight, err := getPageHeight(page)
		if err != nil || currHeight <= prevHeight {
			break
		}
	}
}

// getPageHeight returns the current document scroll height.
func getPageHeight(page *rod.Page) (int, error) {
	res, err := page.Eval(`() => document.documentElement.scrollHeight`)
	if err != nil {
		return 0, err
	}
	return res.Value.Int(), nil
}

// expandReplyButtons finds and clicks reply expansion buttons.
// Uses targeted selectors to avoid scanning all DOM elements.
func expandReplyButtons(page *rod.Page) {
	expandKeywords := []string{"展开", "查看", "条回复"}

	// Target elements likely to be expand buttons
	selectors := []string{".show-more", "[class*='reply'] span", "[class*='comment'] span", "[class*='expand']"}
	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}
		clickExpandElements(elements, expandKeywords)
	}

	// Fallback: check all spans (narrower than "span, div, a, button")
	spans, err := page.Elements("span")
	if err != nil {
		return
	}
	clickExpandElements(spans, expandKeywords)
}

// clickExpandElements clicks elements whose text matches expand keywords.
func clickExpandElements(elements rod.Elements, keywords []string) {
	for _, el := range elements {
		text, err := el.Text()
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if len(text) == 0 || len(text) > 30 {
			continue
		}
		for _, keyword := range keywords {
			if strings.Contains(text, keyword) {
				if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
					logrus.Debugf("点击展开按钮失败: %v", err)
				}
				time.Sleep(expandWait)
				break
			}
		}
	}
}
