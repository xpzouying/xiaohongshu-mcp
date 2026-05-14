package xiaohongshu

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var (
	categoryNameCountRegexp = regexp.MustCompile(`^(.+?)\s*笔记[・·]\s*(\d+)`)
	categoryCountRegexp     = regexp.MustCompile(`笔记[・·]\s*(\d+)`)
)

// GetFavoriteCategories 获取收藏下的专辑分类列表
func (f *FavoriteFeedsAction) GetFavoriteCategories(ctx context.Context) ([]FavoriteCategory, error) {
	page := f.page.Context(ctx)
	if err := f.navigateToFavoriteBoardTab(ctx, page); err != nil {
		return nil, err
	}

	return f.collectFavoriteCategoriesFromPage(page)
}

func (f *FavoriteFeedsAction) collectFavoriteCategoriesFromPage(page *rod.Page) ([]FavoriteCategory, error) {
	anchors, err := page.Elements(`a[href*="/board/"]`)
	if err != nil {
		return nil, fmt.Errorf("failed to find favorite category links: %w", err)
	}

	categories := make([]FavoriteCategory, 0, len(anchors))
	seen := make(map[string]struct{}, len(anchors))
	for _, anchor := range anchors {
		href, ok := readElementAttr(anchor, "href")
		if !ok {
			continue
		}

		category, ok := buildFavoriteCategoryFromBoardLink(anchor, href)
		if !ok {
			continue
		}
		if _, exists := seen[category.ID]; exists {
			continue
		}

		seen[category.ID] = struct{}{}
		categories = append(categories, category)
	}

	return categories, nil
}

func buildFavoriteCategoryFromBoardLink(anchor *rod.Element, href string) (FavoriteCategory, bool) {
	link, err := parseXHSURL(href)
	if err != nil {
		return FavoriteCategory{}, false
	}

	categoryID := extractBoardIDFromPath(link.Path)
	if categoryID == "" {
		return FavoriteCategory{}, false
	}

	text, err := anchor.Text()
	if err != nil {
		text = ""
	}
	name, noteCount := parseFavoriteCategoryText(text)

	return FavoriteCategory{
		ID:        categoryID,
		Name:      name,
		NoteCount: noteCount,
		URL:       link.String(),
	}, true
}

func parseXHSURL(rawURL string) (*url.URL, error) {
	link, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if link.IsAbs() {
		return link, nil
	}

	base, _ := url.Parse("https://www.xiaohongshu.com")
	return base.ResolveReference(link), nil
}

func extractBoardIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "board" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func parseFavoriteCategoryText(text string) (string, int) {
	text = strings.Join(strings.Fields(text), " ")
	name := text
	noteCount := 0

	if matches := categoryNameCountRegexp.FindStringSubmatch(text); len(matches) == 3 {
		name = strings.TrimSpace(matches[1])
		noteCount, _ = strconv.Atoi(matches[2])
	} else if matches := categoryCountRegexp.FindStringSubmatch(text); len(matches) == 2 {
		noteCount, _ = strconv.Atoi(matches[1])
		name = strings.TrimSpace(text[:strings.Index(text, matches[0])])
	}

	if name == "" {
		name = "未命名专辑"
	}
	return name, noteCount
}

// GetFavoriteFeedsByCategory 获取指定收藏专辑下的笔记
func (f *FavoriteFeedsAction) GetFavoriteFeedsByCategory(ctx context.Context, categoryID, categoryName string, limit int) ([]Feed, *FavoriteCategory, error) {
	page := f.page.Context(ctx)
	if err := f.navigateToFavoriteBoardTab(ctx, page); err != nil {
		return nil, nil, err
	}

	categories, err := f.collectFavoriteCategoriesFromPage(page)
	if err != nil {
		return nil, nil, err
	}

	target := findFavoriteCategory(categories, categoryID, categoryName)
	if target == nil {
		return nil, nil, fmt.Errorf("favorite category not found (id=%q, name=%q)", categoryID, categoryName)
	}

	page.MustNavigate(target.URL).MustWaitLoad().MustWaitDOMStable()
	time.Sleep(800 * time.Millisecond)

	feeds, err := f.collectFavoriteFeedsFromPage(page)
	if err != nil {
		return nil, nil, err
	}
	if limit > 0 && len(feeds) > limit {
		feeds = feeds[:limit]
	}

	return feeds, target, nil
}

func findFavoriteCategory(categories []FavoriteCategory, categoryID, categoryName string) *FavoriteCategory {
	normalizedName := normalizeFavoriteCategoryName(categoryName)
	for i := range categories {
		category := &categories[i]
		idMatched := categoryID != "" && category.ID == categoryID
		nameMatched := false
		if normalizedName != "" {
			normalizedCandidate := normalizeFavoriteCategoryName(category.Name)
			nameMatched = normalizedCandidate == normalizedName ||
				strings.HasPrefix(normalizedCandidate, normalizedName) ||
				strings.Contains(normalizedCandidate, normalizedName)
		}
		fallback := categoryID == "" && normalizedName == ""
		if idMatched || nameMatched || fallback {
			return category
		}
	}

	return nil
}

func normalizeFavoriteCategoryName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

func (f *FavoriteFeedsAction) navigateToFavoriteBoardTab(ctx context.Context, page *rod.Page) error {
	navigate := NewNavigate(page)
	if err := navigate.ToProfilePage(ctx); err != nil {
		return fmt.Errorf("failed to navigate to profile page: %w", err)
	}
	page.MustWaitDOMStable()

	if !clickTabByText(page, "收藏") {
		return fmt.Errorf("failed to switch to favorite tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(500 * time.Millisecond)

	if !clickTabByTextPrefix(page, "专辑") {
		return fmt.Errorf("failed to switch to board tab")
	}

	page.MustWaitDOMStable()
	time.Sleep(700 * time.Millisecond)
	return nil
}

func clickTabByTextPrefix(page *rod.Page, tabTextPrefix string) bool {
	selectors := []string{"span.channel", ".reds-tab-item", ".tab-item", "div[class*=\"tab\"]"}
	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.Text()
			if err != nil || !strings.HasPrefix(strings.TrimSpace(text), tabTextPrefix) {
				continue
			}

			if err := element.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return true
			}
		}
	}

	return false
}