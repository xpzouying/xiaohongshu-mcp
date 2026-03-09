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

// ScreenshotFeed captures a post page screenshot including comments section.
func (s *XiaohongshuService) ScreenshotFeed(ctx context.Context, feedID, xsecToken string) (*ScreenshotResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	url := fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_search", feedID, xsecToken)
	logrus.Infof("截图帖子: %s", url)

	page = page.Context(ctx).Timeout(90 * time.Second)
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// Dismiss popups using go-rod native API
	dismissPopupButtons(page)

	// Scroll down to reveal comments using go-rod Mouse.Scroll
	if err := page.Mouse.Scroll(0, 3000, 5); err != nil {
		logrus.Debugf("滚动页面时出错（可忽略）: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Click "展开回复" buttons to expand sub-comments
	expandReplyButtons(page)
	time.Sleep(3 * time.Second)

	// Scroll down again after expanding
	if err := page.Mouse.Scroll(0, 2000, 3); err != nil {
		logrus.Debugf("再次滚动时出错: %v", err)
	}
	time.Sleep(1 * time.Second)

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
	buttons, err := page.Elements("button")
	if err != nil {
		return
	}
	for _, btn := range buttons {
		text, err := btn.Text()
		if err != nil {
			continue
		}
		if strings.Contains(text, "好的") || strings.Contains(text, "我知道了") {
			if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
				logrus.Debugf("关闭弹窗按钮失败: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// expandReplyButtons finds and clicks "展开回复" style buttons.
func expandReplyButtons(page *rod.Page) {
	elements, err := page.Elements("span, div, a, button")
	if err != nil {
		return
	}
	for _, el := range elements {
		text, err := el.Text()
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if len(text) > 30 {
			continue
		}
		if strings.Contains(text, "展开") || strings.Contains(text, "查看") || strings.Contains(text, "条回复") {
			if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
				logrus.Debugf("点击展开按钮失败: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}
