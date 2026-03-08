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
	_, err := page.Eval(`() => {
		document.querySelectorAll('button').forEach(b => {
			if (b.textContent.includes('好的') || b.textContent.includes('我知道了')) b.click();
		});
	}`)
	if err != nil {
		logrus.Debugf("关闭弹窗时出错（可忽略）: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Scroll right panel to bottom to show comments
	_, err = page.Eval(`() => {
		const containers = document.querySelectorAll('.note-scroller, .interaction-container, [class*="scroll"], .note-content');
		containers.forEach(c => {
			if (c.scrollHeight > c.clientHeight) c.scrollTop = c.scrollHeight;
		});
	}`)
	if err != nil {
		logrus.Debugf("滚动评论区时出错: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Click "展开回复" buttons to expand sub-comments
	_, err = page.Eval(`() => {
		document.querySelectorAll('span, div, a, button, .show-more, .reply-container span').forEach(el => {
			const t = (el.textContent || '').trim();
			if ((t.includes('展开') || t.includes('查看') || t.includes('条回复') || t.match(/^\d+条回复$/)) && t.length < 30) {
				try { el.click(); } catch(e) {}
			}
		});
	}`)
	if err != nil {
		logrus.Debugf("展开回复时出错: %v", err)
	}
	time.Sleep(3 * time.Second)

	// Scroll down again after expanding
	_, _ = page.Eval(`() => {
		const containers = document.querySelectorAll('.note-scroller, .interaction-container, [class*="scroll"], .note-content');
		containers.forEach(c => {
			if (c.scrollHeight > c.clientHeight) c.scrollTop = c.scrollHeight;
		});
	}`)
	time.Sleep(1 * time.Second)

	// Take screenshot - return error if fails
	png, err := page.Screenshot(true, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		return nil, fmt.Errorf("截图失败: %w", err)
	}

	screenshots := []string{base64.StdEncoding.EncodeToString(png)}

	logrus.Infof("截图完成，共 %d 张", len(screenshots))
	return &ScreenshotResponse{FeedID: feedID, Screenshots: screenshots}, nil
}
