package xiaohongshu

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

const (
	favoriteScrollRoundsWhenUnlimited = 30
	favoriteScrollIdleRounds          = 3
)

type FavoriteFeedsAction struct {
	page *rod.Page
}

func NewFavoriteFeedsAction(page *rod.Page) *FavoriteFeedsAction {
	pp := page.Timeout(60 * time.Second)
	return &FavoriteFeedsAction{page: pp}
}

// GetFavoriteFeeds 获取当前登录用户的收藏笔记列表
func (f *FavoriteFeedsAction) GetFavoriteFeeds(ctx context.Context, limit int) ([]Feed, error) {
	page := f.page.Context(ctx)

	if err := f.navigateToFavoriteFeedsTab(ctx, page); err != nil {
		return nil, err
	}

	feeds, err := f.collectFavoriteFeedsWithScroll(page, limit)
	if err != nil {
		return nil, err
	}
	if len(feeds) == 0 {
		return nil, errors.ErrNoFeeds
	}

	return feeds, nil
}

func (f *FavoriteFeedsAction) collectFavoriteFeedsWithScroll(page *rod.Page, limit int) ([]Feed, error) {
	feeds := make([]Feed, 0)
	seen := make(map[string]struct{})
	idleRounds := 0

	for round := 0; round <= maxFavoriteScrollRounds(limit); round++ {
		added, err := f.collectFavoriteFeedsFromPage(page, &feeds, seen, limit)
		if err != nil {
			return nil, err
		}
		if limit > 0 && len(feeds) >= limit {
			return feeds[:limit], nil
		}

		if added == 0 {
			idleRounds++
			if idleRounds >= favoriteScrollIdleRounds {
				break
			}
		} else {
			idleRounds = 0
		}

		scrollFavoritePage(page)
	}

	return feeds, nil
}

func maxFavoriteScrollRounds(limit int) int {
	if limit <= 0 {
		return favoriteScrollRoundsWhenUnlimited
	}

	// 每轮通常加载若干卡片，这里按 limit 给滚动留足余量。
	return int(math.Ceil(float64(limit)/6.0)) + favoriteScrollIdleRounds
}

func scrollFavoritePage(page *rod.Page) {
	page.Mouse.MustScroll(0, 900)
	time.Sleep(700 * time.Millisecond)
	page.MustWaitDOMStable()
}

func (f *FavoriteFeedsAction) navigateToFavoriteFeedsTab(ctx context.Context, page *rod.Page) error {
	navigate := NewNavigate(page)
	if err := navigate.ToProfilePage(ctx); err != nil {
		return fmt.Errorf("failed to navigate to profile page: %w", err)
	}

	page.MustWaitDOMStable()
	if !clickTabByText(page, "收藏") {
		return fmt.Errorf("failed to switch to favorite tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(800 * time.Millisecond)
	return nil
}

func clickTabByText(page *rod.Page, tabText string) bool {
	selectors := []string{"span.channel", ".reds-tab-item", ".tab-item", "div[class*=\"tab\"]"}
	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.Text()
			if err != nil || strings.TrimSpace(text) != tabText {
				continue
			}

			if err := element.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return true
			}
		}
	}

	return false
}

func (f *FavoriteFeedsAction) collectFavoriteFeedsFromPage(page *rod.Page, feeds *[]Feed, seen map[string]struct{}, limit int) (int, error) {
	anchors, err := page.Elements(`a[href*="/explore/"], a[href*="/discovery/item/"], a[href*="/user/profile/"]`)
	if err != nil {
		return 0, fmt.Errorf("failed to find favorite feed links: %w", err)
	}

	added := 0
	for _, anchor := range anchors {
		href, ok := readElementAttr(anchor, "href")
		if !ok {
			continue
		}

		feed, ok := buildFeedFromFavoriteLink(anchor, href, len(*feeds))
		if !ok {
			continue
		}

		key := feed.ID + ":" + feed.XsecToken
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		*feeds = append(*feeds, feed)
		added++

		if limit > 0 && len(*feeds) >= limit {
			break
		}
	}

	return added, nil
}

func buildFeedFromFavoriteLink(anchor *rod.Element, href string, index int) (Feed, bool) {
	link, err := url.Parse(href)
	if err != nil {
		return Feed{}, false
	}
	if !link.IsAbs() {
		base, _ := url.Parse("https://www.xiaohongshu.com")
		link = base.ResolveReference(link)
	}

	feedID := extractFeedIDFromPath(link.Path)
	xsecToken := link.Query().Get("xsec_token")
	if xsecToken == "" {
		xsecToken = link.Query().Get("xsecToken")
	}
	if xsecToken == "" {
		xsecToken, _ = readElementAttr(anchor, "data-xsec-token")
	}
	if feedID == "" || xsecToken == "" {
		return Feed{}, false
	}

	title := readFeedTitle(anchor)
	return Feed{
		ID:        feedID,
		XsecToken: xsecToken,
		ModelType: "note",
		NoteCard: NoteCard{
			Type:         "normal",
			DisplayTitle: title,
		},
		Index: index,
	}, true
}

func extractFeedIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		switch part {
		case "explore":
			if i+1 < len(parts) {
				return parts[i+1]
			}
		case "discovery":
			if i+2 < len(parts) && parts[i+1] == "item" {
				return parts[i+2]
			}
		case "profile":
			if i+2 < len(parts) {
				return parts[i+2]
			}
		}
	}

	return ""
}

func readFeedTitle(anchor *rod.Element) string {
	text, err := anchor.Text()
	if err != nil {
		return ""
	}

	title := strings.Join(strings.Fields(text), " ")
	if len(title) > 200 {
		return title[:200]
	}
	return title
}

func readElementAttr(element *rod.Element, name string) (string, bool) {
	value, err := element.Attribute(name)
	if err != nil || value == nil || strings.TrimSpace(*value) == "" {
		return "", false
	}

	return strings.TrimSpace(*value), true
}