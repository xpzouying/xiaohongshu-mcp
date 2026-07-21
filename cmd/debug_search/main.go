package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	appbrowser "github.com/xpzouying/xiaohongshu-mcp/browser"
)

func main() {
	binPath := os.Getenv("ROD_BROWSER_BIN")
	b := appbrowser.NewBrowser(true, appbrowser.WithBinPath(binPath))
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	pp := page.Timeout(40 * time.Second).Context(ctx)
	url := os.Getenv("DEBUG_SEARCH_URL")
	if url == "" {
		url = "https://www.xiaohongshu.com/search_result?keyword=%E4%B8%80%E4%BA%BA%E5%85%AC%E5%8F%B8&source=web_explore_feed"
	}
	pp.MustNavigate(url).MustWaitLoad()

	if keyword := os.Getenv("DEBUG_SEARCH_KEYWORD"); keyword != "" {
		inputElem := pp.MustElement("#search-input")
		inputElem.MustSelectAllText()
		inputElem.MustInput(keyword).MustType(input.Enter)
		pp.MustWait(`() => window.location.href.includes('/search_result') || (document.body && document.body.innerText.includes('登录后查看搜索结果'))`)
		pp.MustWaitLoad()
	}

	hasInitialState := false
	waitErr := rod.Try(func() {
		pp.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)
		hasInitialState = true
	})

	info := pp.MustInfo()
	title := info.Title
	currentURL := info.URL
	htmlSnippet := pp.MustElement("body").MustText()
	feedsState := pp.MustEval(`() => {
		const s = window.__INITIAL_STATE__;
		if (!s || !s.search) return JSON.stringify({hasSearch:false});
		const feeds = s.search.feeds;
		const raw = feeds ? (feeds.value !== undefined ? feeds.value : feeds._value) : undefined;
		const topLevelKeys = Object.keys(s || {});
		const searchEntries = Object.entries(s.search || {}).map(([key, value]) => ({
			key,
			type: Array.isArray(value) ? "array" : typeof value,
			hasValue: !!value,
			length: Array.isArray(value) ? value.length : undefined,
		}));
		return JSON.stringify({
			hasSearch: true,
			hasFeeds: !!feeds,
			feedsType: typeof raw,
			feedsIsArray: Array.isArray(raw),
			feedsLength: Array.isArray(raw) ? raw.length : -1,
			searchKeys: Object.keys(s.search || {}),
			topLevelKeys,
			searchEntries,
		});
	}`).String()
	pageSignals := pp.MustEval(`() => JSON.stringify({
		pathname: location.pathname,
		title: document.title,
		bodyText: document.body ? document.body.innerText.slice(0, 1200) : "",
		hasNotFoundText: document.body ? document.body.innerText.includes("你访问的页面不见了") : false,
		hasAntiSpamText: document.body ? document.body.innerText.includes("anti_spam") : false,
		linkCount: document.querySelectorAll('a').length,
		noteLinkCount: document.querySelectorAll('a[href*="/explore/"]').length,
		searchResultLinkCount: document.querySelectorAll('a[href*="/search_result"]').length,
	})`).String()
	inputsState := pp.MustEval(`() => JSON.stringify(
		Array.from(document.querySelectorAll('input'))
			.map((el, i) => ({
				i,
				placeholder: el.getAttribute('placeholder'),
				value: el.value,
				className: el.className,
				type: el.type,
			}))
	)`).String()
	searchWidgetState := pp.MustEval(`() => {
		const el = document.querySelector('input.search-input');
		if (!el) return '';
		const parent = el.parentElement;
		return parent ? parent.outerHTML.slice(0, 2000) : el.outerHTML.slice(0, 2000);
	}`).String()

	fmt.Printf("title=%s\n", title)
	fmt.Printf("url=%s\n", currentURL)
	fmt.Printf("hasInitialState=%v\n", hasInitialState)
	fmt.Printf("feedsState=%s\n", feedsState)
	fmt.Printf("inputsState=%s\n", inputsState)
	fmt.Printf("searchWidgetState=%s\n", searchWidgetState)
	fmt.Printf("pageSignals=%s\n", pageSignals)
	if waitErr != nil {
		fmt.Printf("waitErr=%v\n", waitErr)
	}
	if len(htmlSnippet) > 800 {
		htmlSnippet = htmlSnippet[:800]
	}
	fmt.Printf("body=%s\n", htmlSnippet)
}
