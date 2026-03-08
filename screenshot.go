package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

type ScreenshotRequest struct {
	FeedID    string `json:"feed_id" binding:"required"`
	XsecToken string `json:"xsec_token" binding:"required"`
}

type ScreenshotResponse struct {
	FeedID      string   `json:"feed_id"`
	Screenshots []string `json:"screenshots"`
}

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

	// Dismiss popups
	page.MustEval(`() => {
		document.querySelectorAll('button').forEach(b => {
			if (b.textContent.includes('好的') || b.textContent.includes('我知道了')) b.click();
		});
	}`)
	time.Sleep(1 * time.Second)

	// Step 1: Scroll right panel to bottom to show comments
	page.MustEval(`() => {
		const containers = document.querySelectorAll('.note-scroller, .interaction-container, [class*="scroll"], .note-content');
		containers.forEach(c => {
			if (c.scrollHeight > c.clientHeight) c.scrollTop = c.scrollHeight;
		});
	}`)
	time.Sleep(2 * time.Second)

	// Step 2: Click ALL "展开回复" / "查看回复" / reply count buttons
	page.MustEval(`() => {
		// Click elements that contain reply-related text
		document.querySelectorAll('span, div, a, button, .show-more, .reply-container span').forEach(el => {
			const t = (el.textContent || '').trim();
			if ((t.includes('展开') || t.includes('查看') || t.includes('条回复') || t.match(/^\d+条回复$/)) && t.length < 30) {
				try { el.click(); console.log('Clicked:', t); } catch(e) {}
			}
		});
	}`)
	time.Sleep(3 * time.Second)

	// Step 3: Scroll down again after expanding
	page.MustEval(`() => {
		const containers = document.querySelectorAll('.note-scroller, .interaction-container, [class*="scroll"], .note-content');
		containers.forEach(c => {
			if (c.scrollHeight > c.clientHeight) c.scrollTop = c.scrollHeight;
		});
	}`)
	time.Sleep(1 * time.Second)

	screenshots := []string{}

	// Take full page screenshot
	png, err := page.Screenshot(true, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err == nil {
		screenshots = append(screenshots, base64.StdEncoding.EncodeToString(png))
	}

	logrus.Infof("截图完成，共 %d 张", len(screenshots))
	return &ScreenshotResponse{FeedID: feedID, Screenshots: screenshots}, nil
}
