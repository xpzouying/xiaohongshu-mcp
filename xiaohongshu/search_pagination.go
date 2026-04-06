package xiaohongshu

import (
	"encoding/json"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// scrollAndCollectFeeds 滚动页面加载更多搜索结果，按 Feed.ID 去重
func scrollAndCollectFeeds(page *rod.Page, initialFeeds []Feed, limit int) []Feed {
	// 确保翻页有超时保护，防止单次 WaitStable/Eval 无限等待
	page = page.Timeout(60 * time.Second)

	seen := make(map[string]bool, len(initialFeeds))
	var allFeeds []Feed
	for _, f := range initialFeeds {
		if !seen[f.ID] {
			seen[f.ID] = true
			allFeeds = append(allFeeds, f)
		}
	}

	if len(allFeeds) >= limit {
		return allFeeds[:limit]
	}

	const maxStale = 3
	staleCount := 0

	for staleCount < maxStale && len(allFeeds) < limit {
		// 使用 go-rod 滚动触发懒加载
		if err := page.Mouse.Scroll(0, 800, 3); err != nil {
			logrus.Warnf("翻页滚动失败: %v", err)
			break
		}
		time.Sleep(1 * time.Second)

		// 等待页面稳定
		_ = page.WaitStable(500 * time.Millisecond)

		// 重读 __INITIAL_STATE__ 中的 feeds
		obj, err := page.Eval(readFeedsJS)
		if err != nil {
			logrus.Warnf("翻页读取 feeds 失败（可能超时）: %v", err)
			break
		}

		raw := obj.Value.String()
		if raw == "" {
			staleCount++
			continue
		}

		var feeds []Feed
		if err := json.Unmarshal([]byte(raw), &feeds); err != nil {
			logrus.Warnf("翻页解析 feeds 失败: %v", err)
			staleCount++
			continue
		}

		newCount := 0
		for _, f := range feeds {
			if !seen[f.ID] {
				seen[f.ID] = true
				allFeeds = append(allFeeds, f)
				newCount++
			}
		}

		if newCount == 0 {
			staleCount++
		} else {
			staleCount = 0
		}

		logrus.Infof("翻页: 本次新增 %d 条, 总计 %d/%d", newCount, len(allFeeds), limit)
	}

	if len(allFeeds) > limit {
		return allFeeds[:limit]
	}
	return allFeeds
}

// readFeedsJS 读取 __INITIAL_STATE__ 中 feeds 数据的 JS 脚本
const readFeedsJS = `() => {
	if (window.__INITIAL_STATE__ &&
	    window.__INITIAL_STATE__.search &&
	    window.__INITIAL_STATE__.search.feeds) {
		const feeds = window.__INITIAL_STATE__.search.feeds;
		const feedsData = feeds.value !== undefined ? feeds.value : feeds._value;
		if (feedsData) {
			return JSON.stringify(feedsData);
		}
	}
	return "";
}`
