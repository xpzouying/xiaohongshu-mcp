package xiaohongshu

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

// ResolveVideoURLFromDetail 从详情结构中解析视频 URL。
func ResolveVideoURLFromDetail(note FeedDetail) string {
	if note.Video == nil {
		return ""
	}

	streamCandidates := [][]DetailVideoStreamItem{
		note.Video.Media.Stream.H264,
		note.Video.Media.Stream.H265,
		note.Video.Media.Stream.H266,
		note.Video.Media.Stream.AV1,
	}

	for _, items := range streamCandidates {
		for _, item := range items {
			if trimmed := strings.TrimSpace(item.MasterURL); trimmed != "" {
				return trimmed
			}
			for _, backup := range item.BackupURLs {
				if trimmed := strings.TrimSpace(backup); trimmed != "" {
					return trimmed
				}
			}
		}
	}

	return strings.TrimSpace(note.Video.Media.OriginVideoKey)
}

// ResolveVideoURLFromPage 从页面 video 元素兜底解析视频 URL。
func ResolveVideoURLFromPage(page *rod.Page) (string, error) {
	if page == nil {
		return "", fmt.Errorf("page is nil")
	}

	videoEl, err := page.Timeout(3 * time.Second).Element("video")
	if err != nil {
		return "", err
	}

	src, err := videoEl.Attribute("src")
	if err == nil && src != nil && strings.TrimSpace(*src) != "" {
		return strings.TrimSpace(*src), nil
	}

	currentSrc, err := videoEl.Property("currentSrc")
	if err == nil {
		if val := strings.TrimSpace(currentSrc.String()); val != "" && val != "null" {
			return val, nil
		}
	}

	return "", fmt.Errorf("video url not found on page")
}
