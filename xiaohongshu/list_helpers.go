package xiaohongshu

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

func getMyUserID(page *rod.Page) (string, error) {
	result := page.MustEval(`() => {
		const profileLinks = Array.from(document.querySelectorAll("a[href*='user/profile']"));
		if (profileLinks.length > 0) {
			const match = profileLinks[0].href.match(/user\/profile\/([^/?#]+)/);
			if (match) return match[1];
		}

		try {
			const state = window.__INITIAL_STATE__;
			const uid = state?.user?.userInfo?.value?.userId ||
				state?.user?.userInfo?._value?.userId;
			if (uid) return uid;
		} catch (e) {}

		return "";
	}`).String()

	if result == "" {
		return "", fmt.Errorf("could not get user ID from page")
	}

	return result, nil
}

func parseListCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}

	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid cursor: %q", cursor)
	}

	return offset, nil
}

func loadEnoughNoteItems(page *rod.Page, target int) {
	if target <= 0 {
		return
	}

	lastCount := -1
	stableRounds := 0

	for i := 0; i < 8; i++ {
		count := page.MustEval(`() => document.querySelectorAll(".note-item").length`).Int()
		if count >= target {
			return
		}

		if count == lastCount {
			stableRounds++
			if stableRounds >= 2 {
				return
			}
		} else {
			stableRounds = 0
		}
		lastCount = count

		page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
		time.Sleep(1500 * time.Millisecond)
	}
}

// extractFeedID 从小红书笔记 URL 中提取 feed ID
func extractFeedID(link string) string {
	if link == "" {
		return ""
	}

	parsed, err := url.Parse(link)
	if err == nil && parsed.Path != "" {
		id := strings.Trim(path.Base(parsed.Path), "/")
		if id != "" && id != "." {
			return id
		}
	}

	cleaned := link
	if idx := strings.IndexAny(cleaned, "?#"); idx >= 0 {
		cleaned = cleaned[:idx]
	}
	cleaned = strings.TrimRight(cleaned, "/")
	if cleaned == "" {
		return ""
	}

	return path.Base(cleaned)
}
