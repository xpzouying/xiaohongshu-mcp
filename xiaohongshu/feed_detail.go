package xiaohongshu

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// ========== 配置常量 ==========
const (
	defaultMaxAttempts   = 500
	stagnantLimit        = 20
	minScrollDelta       = 10
	maxClickPerRound     = 3
	largeScrollTrigger   = 5
	buttonClickInterval  = 3
	finalSprintPushCount = 15

	maxSearchScrolls  = 25
	maxExpandRounds   = 5
	maxSearchDuration = 90 * time.Second
)

// ========== 数据结构 ==========

type CommentLoadConfig struct {
	ClickMoreReplies    bool
	MaxRepliesThreshold int
	MaxCommentItems     int
	ScrollSpeed         string
}

const (
	defaultMaxCommentItems     = 20
	defaultMaxRepliesThreshold = 10
	defaultScrollSpeed         = "normal"
)

func DefaultCommentLoadConfig() CommentLoadConfig {
	return CommentLoadConfig{
		ClickMoreReplies:    false,
		MaxRepliesThreshold: defaultMaxRepliesThreshold,
		MaxCommentItems:     defaultMaxCommentItems,
		ScrollSpeed:         defaultScrollSpeed,
	}
}

// normalize 把零值字段填回默认值。零值一律按「未设置」处理，不再按「无上限」。
func (c CommentLoadConfig) normalize() CommentLoadConfig {
	if c.MaxCommentItems <= 0 {
		c.MaxCommentItems = defaultMaxCommentItems
	}
	if c.MaxRepliesThreshold <= 0 {
		c.MaxRepliesThreshold = defaultMaxRepliesThreshold
	}
	if c.ScrollSpeed == "" {
		c.ScrollSpeed = defaultScrollSpeed
	}
	return c
}

type FeedDetailAction struct {
	page playwright.Page
}

func NewFeedDetailAction(page playwright.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// ========== 主要业务逻辑 ==========

func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config CommentLoadConfig) (*FeedDetailResponse, error) {
	return f.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, config)
}

func (f *FeedDetailAction) GetFeedDetailWithConfig(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config CommentLoadConfig) (*FeedDetailResponse, error) {
	config = config.normalize()

	page := f.page
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("打开 feed 详情页: %s", RedactURL(url))
	logrus.Infof("配置: 点击更多=%v, 回复阈值=%d, 最大评论数=%d, 滚动速度=%s",
		config.ClickMoreReplies, config.MaxRepliesThreshold, config.MaxCommentItems, config.ScrollSpeed)

	// 使用 retry-go 处理页面导航和 DOM 稳定等待
	err := retry.Do(
		func() error {
			if _, err := page.Goto(url, playwright.PageGotoOptions{
				WaitUntil: playwright.WaitUntilStateDomcontentloaded,
				Timeout:   playwright.Float(60_000),
			}); err != nil {
				return err
			}
			_, err := page.WaitForFunction(`() => window.__INITIAL_STATE__ !== undefined`, nil,
				playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(30_000)})
			return err
		},
		retry.Attempts(3),
		retry.Delay(500*time.Millisecond),
		retry.MaxJitter(1000*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("页面导航重试 #%d: %v", n, err)
		}),
	)
	if err != nil {
		logrus.Errorf("页面导航失败: %v", err)
		return nil, err
	}
	humanize.Delay(ctx, humanize.AfterNavigate)

	if err := checkPageAccessible(page); err != nil {
		return nil, err
	}

	if loadAllComments {
		if err := f.loadAllCommentsWithConfig(ctx, page, config); err != nil {
			logrus.Warnf("加载全部评论失败: %v", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return f.extractFeedDetail(page, feedID)
}

// ========== 评论加载器 ==========

type commentLoader struct {
	page   playwright.Page
	config CommentLoadConfig
	stats  *loadStats
	state  *loadState
}

type loadStats struct {
	totalClicked int
	totalSkipped int
	attempts     int
}

type loadState struct {
	lastCount      int
	lastScrollTop  int
	stagnantChecks int
}

func (f *FeedDetailAction) loadAllCommentsWithConfig(ctx context.Context, page playwright.Page, config CommentLoadConfig) error {
	loader := &commentLoader{
		page:   page,
		config: config,
		stats:  &loadStats{},
		state:  &loadState{},
	}
	return loader.load(ctx)
}

func (cl *commentLoader) load(ctx context.Context) error {
	maxAttempts := cl.calculateMaxAttempts()

	logrus.Info("开始加载评论...")
	scrollToCommentsArea(cl.page)
	humanize.Delay(ctx, humanize.BetweenScroll)

	if cl.checkNoComments() {
		return nil
	}

	for cl.stats.attempts = 0; cl.stats.attempts < maxAttempts; cl.stats.attempts++ {
		if err := ctx.Err(); err != nil {
			logrus.Infof("上下文已取消，停止加载评论: %v", err)
			return err
		}

		logrus.Debugf("=== 尝试 %d/%d ===", cl.stats.attempts+1, maxAttempts)

		if cl.checkComplete(ctx) {
			return nil
		}

		if cl.shouldClickButtons() {
			cl.clickButtonsWithRetry(ctx)
		}

		currentCount := getCommentCount(cl.page)
		cl.updateState(currentCount)

		if cl.shouldStopAtTarget(currentCount) {
			return nil
		}

		cl.performScroll(ctx)
		cl.handleStagnation(ctx)

		humanize.Delay(ctx, humanize.BetweenScroll)
	}

	cl.performFinalSprint(ctx)
	return nil
}

func (cl *commentLoader) calculateMaxAttempts() int {
	if cl.config.MaxCommentItems > 0 {
		return cl.config.MaxCommentItems * 3
	}
	return defaultMaxAttempts
}

func (cl *commentLoader) checkNoComments() bool {
	if checkNoCommentsArea(cl.page) {
		logrus.Infof("✓ 检测到无评论区域（这是一片荒地），跳过加载")
		return true
	}
	return false
}

func (cl *commentLoader) checkComplete(ctx context.Context) bool {
	if !checkEndContainer(cl.page) {
		return false
	}

	// 到底之后再展开一轮：点开就先不算完，让下一轮重新判断
	if cl.config.ClickMoreReplies && cl.clickButtonsWithRetry(ctx) > 0 {
		return false
	}

	currentCount := getCommentCount(cl.page)
	logrus.Infof("✓ 检测到 'THE END' 元素，已滑动到底部")
	humanize.Delay(ctx, humanize.BetweenScroll)
	logrus.Infof("✓ 加载完成: %d 条评论, 尝试次数: %d, 点击: %d, 跳过: %d",
		currentCount, cl.stats.attempts+1, cl.stats.totalClicked, cl.stats.totalSkipped)
	return true
}

func (cl *commentLoader) shouldClickButtons() bool {
	return cl.config.ClickMoreReplies && cl.stats.attempts%buttonClickInterval == 0
}

func (cl *commentLoader) clickButtonsWithRetry(ctx context.Context) int {
	clicked, skipped := clickShowMoreButtonsSmart(ctx, cl.page, cl.config.MaxRepliesThreshold)
	if clicked == 0 && skipped == 0 {
		return 0
	}

	cl.stats.totalClicked += clicked
	cl.stats.totalSkipped += skipped
	logrus.Infof("点击'更多': %d 个, 跳过: %d 个, 累计点击: %d, 累计跳过: %d",
		clicked, skipped, cl.stats.totalClicked, cl.stats.totalSkipped)

	humanize.Delay(ctx, humanize.Reading)

	clicked2, skipped2 := clickShowMoreButtonsSmart(ctx, cl.page, cl.config.MaxRepliesThreshold)
	if clicked2 > 0 || skipped2 > 0 {
		cl.stats.totalClicked += clicked2
		cl.stats.totalSkipped += skipped2
		logrus.Infof("第 2 轮: 点击 %d, 跳过 %d", clicked2, skipped2)
		humanize.Delay(ctx, humanize.Reading)
	}

	return clicked + clicked2
}

func (cl *commentLoader) updateState(currentCount int) {
	totalCount := getTotalCommentCount(cl.page)
	logrus.Debugf("当前评论: %d, 目标: %d", currentCount, totalCount)

	if currentCount != cl.state.lastCount {
		logrus.Infof("✓ 评论增加: %d -> %d (+%d)",
			cl.state.lastCount, currentCount, currentCount-cl.state.lastCount)
		cl.state.lastCount = currentCount
		cl.state.stagnantChecks = 0
	} else {
		cl.state.stagnantChecks++
		if cl.state.stagnantChecks%5 == 0 {
			logrus.Debugf("评论停滞 %d 次", cl.state.stagnantChecks)
		}
	}
}

func (cl *commentLoader) shouldStopAtTarget(currentCount int) bool {
	if cl.config.MaxCommentItems <= 0 {
		return false
	}
	if currentCount >= cl.config.MaxCommentItems {
		logrus.Infof("✓ 已达到目标评论数: %d/%d, 停止加载",
			currentCount, cl.config.MaxCommentItems)
		return true
	}
	return false
}

func (cl *commentLoader) performScroll(ctx context.Context) {
	currentCount := getCommentCount(cl.page)
	if currentCount > 0 {
		scrollToLastComment(cl.page)
		time.Sleep(400 * time.Millisecond)
	}

	largeMode := cl.state.stagnantChecks >= largeScrollTrigger
	pushCount := 1
	if largeMode {
		pushCount = 3 + rand.Intn(3)
	}

	_, scrollDelta, currentScrollTop := humanScroll(ctx, cl.page, cl.config.ScrollSpeed, largeMode, pushCount)

	if scrollDelta < minScrollDelta || currentScrollTop == cl.state.lastScrollTop {
		cl.state.stagnantChecks++
		if cl.state.stagnantChecks%5 == 0 {
			logrus.Debugf("滚动停滞 %d 次", cl.state.stagnantChecks)
		}
	} else {
		cl.state.stagnantChecks = 0
		cl.state.lastScrollTop = currentScrollTop
	}
}

func (cl *commentLoader) handleStagnation(ctx context.Context) {
	if cl.state.stagnantChecks >= stagnantLimit {
		logrus.Infof("停滞过多，尝试大冲刺...")
		humanScroll(ctx, cl.page, cl.config.ScrollSpeed, true, 10)
		cl.state.stagnantChecks = 0

		if checkEndContainer(cl.page) {
			currentCount := getCommentCount(cl.page)
			logrus.Infof("✓ 到达底部，评论数: %d", currentCount)
		}
	}
}

func (cl *commentLoader) performFinalSprint(ctx context.Context) {
	logrus.Infof("达到最大尝试次数，最后冲刺...")
	humanScroll(ctx, cl.page, cl.config.ScrollSpeed, true, finalSprintPushCount)

	currentCount := getCommentCount(cl.page)
	hasEnd := checkEndContainer(cl.page)
	logrus.Infof("✓ 加载结束: %d 条评论, 点击: %d, 跳过: %d, 到达底部: %v",
		currentCount, cl.stats.totalClicked, cl.stats.totalSkipped, hasEnd)
}

// ========== 按钮点击 ==========

func clickShowMoreButtonsSmart(ctx context.Context, page playwright.Page, maxRepliesThreshold int) (clicked, skipped int) {
	elements, err := page.QuerySelectorAll(".show-more")
	if err != nil {
		return 0, 0
	}

	replyCountRegex := regexp.MustCompile(`展开\s*(\d+)\s*条回复`)
	maxClick := maxClickPerRound + rand.Intn(maxClickPerRound)
	clickedInRound := 0

	for _, el := range elements {
		if clickedInRound >= maxClick {
			break
		}

		if !isElementClickable(el) {
			continue
		}

		text, err := el.InnerText()
		if err != nil {
			continue
		}

		if !isSafeExpandButton(el, text) {
			continue
		}

		if shouldSkipButton(text, maxRepliesThreshold, replyCountRegex) {
			skipped++
			continue
		}

		if clickElementWithHumanBehavior(ctx, page, el, text) {
			clicked++
			clickedInRound++
		}
	}

	return clicked, skipped
}

// expandNearbyReplies 展开视口附近的「展开 N 条回复」，返回本轮点开的个数。
func expandNearbyReplies(ctx context.Context, page playwright.Page) int {
	elements, err := page.QuerySelectorAll(".show-more")
	if err != nil || len(elements) == 0 {
		return 0
	}

	maxClick := maxClickPerRound + rand.Intn(maxClickPerRound)
	clicked := 0

	for _, el := range elements {
		if clicked >= maxClick {
			break
		}

		if !isElementClickable(el) || !isNearViewport(page, el) {
			continue
		}

		text, err := el.InnerText()
		if err != nil {
			continue
		}

		if !isSafeExpandButton(el, text) {
			continue
		}

		if clickElementWithHumanBehavior(ctx, page, el, text) {
			clicked++
		}
	}

	return clicked
}

// isSafeExpandButton 判断 .show-more 是不是展开回复按钮。
func isSafeExpandButton(el playwright.ElementHandle, text string) bool {
	if !isExpandRepliesButton(text) {
		logrus.Debugf("跳过展开按钮：文案不匹配 %q", text)
		return false
	}
	if !hasReadableSize(el) {
		logrus.Debugf("跳过展开按钮：尺寸过小 %q", text)
		return false
	}
	return true
}

var expandRepliesTextRegex = regexp.MustCompile(`^展开\s*(\d+\s*条|更多)回复$`)

func isExpandRepliesButton(text string) bool {
	return expandRepliesTextRegex.MatchString(strings.TrimSpace(text))
}

// hasReadableSize 判断元素尺寸是否达到按钮的量级。
func hasReadableSize(el playwright.ElementHandle) bool {
	const minWidth, minHeight = 24, 10
	box, err := el.BoundingBox()
	if err != nil || box == nil {
		return false
	}
	return box.Width >= minWidth && box.Height >= minHeight
}

// isNearViewport 判断元素是否落在视口上下各一屏的范围内。
func isNearViewport(page playwright.Page, el playwright.ElementHandle) bool {
	box, err := el.BoundingBox()
	if err != nil || box == nil {
		return false
	}
	top := box.Y
	height, ok := evalFloat(page, `() => window.innerHeight`)
	if !ok {
		return false
	}
	return top > -height && top < 2*height
}

func isElementClickable(el playwright.ElementHandle) bool {
	visible, err := el.IsVisible()
	if err != nil || !visible {
		return false
	}
	box, err := el.BoundingBox()
	return err == nil && box != nil && box.Width > 0 && box.Height > 0
}

func shouldSkipButton(text string, threshold int, regex *regexp.Regexp) bool {
	if threshold <= 0 {
		return false
	}
	matches := regex.FindStringSubmatch(text)
	if len(matches) > 1 {
		if replyCount, err := strconv.Atoi(matches[1]); err == nil && replyCount > threshold {
			logrus.Debugf("跳过'%s'（回复数 %d > 阈值 %d）", text, replyCount, threshold)
			return true
		}
	}
	return false
}

func clickElementWithHumanBehavior(ctx context.Context, page playwright.Page, el playwright.ElementHandle, text string) bool {
	var clickSuccess bool

	err := retry.Do(
		func() error {
			if err := el.ScrollIntoViewIfNeeded(); err != nil {
				return err
			}
			humanize.Delay(ctx, humanize.Reading)
			if err := humanize.Click(el); err != nil {
				return err
			}
			humanize.Delay(ctx, humanize.Reading)
			clickSuccess = true
			return nil
		},
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.MaxJitter(200*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("点击重试 #%d: %s, 错误: %v", n, text, err)
		}),
	)

	if err != nil {
		logrus.Debugf("点击失败 '%s': %v", text, err)
		return false
	}
	if clickSuccess {
		logrus.Debugf("点击了'%s'", text)
	}
	return clickSuccess
}

// ========== 滚动相关 ==========

func humanScroll(ctx context.Context, page playwright.Page, speed string, largeMode bool, pushCount int) (bool, int, int) {
	beforeTop := getScrollTop(page)
	viewportHeight, _ := evalFloat(page, `() => window.innerHeight`)

	baseRatio := getScrollRatio(speed)
	if largeMode {
		baseRatio *= 2.0
	}

	scrolled := false
	actualDelta := 0
	currentScrollTop := beforeTop

	for i := 0; i < max(1, pushCount); i++ {
		scrollDelta := calculateScrollDelta(viewportHeight, baseRatio)
		smartScroll(page, scrollDelta)

		time.Sleep(150 * time.Millisecond)

		currentScrollTop = getScrollTop(page)
		deltaThisTime := currentScrollTop - beforeTop
		actualDelta += deltaThisTime

		if deltaThisTime > 5 {
			scrolled = true
		}

		beforeTop = currentScrollTop

		if i < pushCount-1 {
			humanize.Delay(ctx, humanize.BetweenScroll)
		}
	}

	// 兜底：常规幅度没推动，加大力度再滚一次。
	if !scrolled && pushCount > 0 {
		smartScroll(page, viewportHeight*3)
		time.Sleep(400 * time.Millisecond)
		currentScrollTop = getScrollTop(page)
		actualDelta += currentScrollTop - beforeTop
		scrolled = actualDelta > 5
	}

	if scrolled {
		logrus.Debugf("滚动: %d -> %d (Δ%d, large=%v, push=%d)",
			beforeTop-actualDelta, currentScrollTop, actualDelta, largeMode, pushCount)
	}

	return scrolled, actualDelta, currentScrollTop
}

func getScrollRatio(speed string) float64 {
	switch speed {
	case "slow":
		return 0.5
	case "fast":
		return 0.9
	default:
		return 0.7
	}
}

func calculateScrollDelta(viewportHeight float64, baseRatio float64) float64 {
	scrollDelta := viewportHeight * (baseRatio + rand.Float64()*0.2)
	if scrollDelta < 400 {
		scrollDelta = 400
	}
	return scrollDelta + float64(rand.Intn(100)-50)
}

func scrollToCommentsArea(page playwright.Page) {
	logrus.Info("滚动到评论区...")

	if el, err := queryWithTimeout(page, ".comments-container", 2*time.Second); err == nil && el != nil {
		_ = el.ScrollIntoViewIfNeeded()
	}
	time.Sleep(400 * time.Millisecond)

	smartScroll(page, 100)
}

// smartScroll 向下滚动 delta 像素，触发评论区懒加载。
// 指针先落在评论滚动容器上，滚轮才只作用于评论区。
func smartScroll(page playwright.Page, delta float64) {
	moveToCommentScroller(page)

	for remain := delta; remain > 0; {
		notch := scrollNotchSize()
		if notch > remain {
			notch = remain
		}
		if err := page.Mouse().Wheel(0, notch); err != nil {
			return
		}
		remain -= notch

		if remain > 0 {
			time.Sleep(scrollNotchInterval())
		}
	}
}

func scrollNotchSize() float64 {
	return 100 + rand.Float64()*40
}

func scrollNotchInterval() time.Duration {
	return time.Duration(20+rand.Intn(45)) * time.Millisecond
}

// commentScrollerSelectors 评论区滚动容器，按优先级排列。
var commentScrollerSelectors = []string{".note-scroller", ".comments-container"}

// moveToCommentScroller 把指针移到评论滚动容器内；找不到则退回视口中心。
func moveToCommentScroller(page playwright.Page) {
	for _, sel := range commentScrollerSelectors {
		el, err := queryWithTimeout(page, sel, 2*time.Second)
		if err != nil || el == nil {
			continue
		}
		box, err := el.BoundingBox()
		if err != nil || box == nil {
			continue
		}
		left, top, right, bottom := box.X, box.Y, box.X+box.Width, box.Y+box.Height

		// 落点在容器中心附近随机偏移，不固定在几何中心
		cx, cy := (left+right)/2, (top+bottom)/2
		_ = humanize.MoveTo(page, humanize.Point{
			X: cx + (rand.Float64()-0.5)*(right-left)*0.3,
			Y: cy + (rand.Float64()-0.5)*(bottom-top)*0.3,
		})
		return
	}
	vw, _ := evalFloat(page, `() => window.innerWidth`)
	vh, _ := evalFloat(page, `() => window.innerHeight`)
	_ = humanize.MoveTo(page, humanize.Point{X: vw / 2, Y: vh / 2})
}

func scrollToLastComment(page playwright.Page) {
	elements, err := page.QuerySelectorAll(".parent-comment")
	if err != nil || len(elements) == 0 {
		return
	}
	_ = elements[len(elements)-1].ScrollIntoViewIfNeeded()
}

// ========== DOM 查询 ==========

// queryWithTimeout 带超时的单元素查询；未命中返回 (nil, nil)，查询出错才返回 err。
func queryWithTimeout(page playwright.Page, selector string, timeout time.Duration) (playwright.ElementHandle, error) {
	el, err := page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
	if err != nil {
		if stderrors.Is(err, playwright.ErrTimeout) {
			return nil, nil
		}
		return nil, err
	}
	return el, nil
}

func getScrollTop(page playwright.Page) int {
	selsJSON, _ := json.Marshal(commentScrollerSelectors)
	expr := fmt.Sprintf(`() => {
		const sels = %s;
		// 详情页评论在容器内滚动，window 的 scrollTop 恒为 0；读实际滚动的容器。
		for (const sel of sels) {
			const el = document.querySelector(sel);
			if (el && el.scrollHeight > el.clientHeight) {
				return el.scrollTop;
			}
		}
		return window.pageYOffset || document.documentElement.scrollTop || document.body.scrollTop || 0;
	}`, selsJSON)
	v, ok := evalFloat(page, expr)
	if !ok {
		return 0
	}
	return int(v)
}

func getCommentCount(page playwright.Page) int {
	elements, err := page.QuerySelectorAll(".parent-comment")
	if err != nil {
		return 0
	}
	return len(elements)
}

// getTotalCommentCount 取笔记的评论总数，读 __INITIAL_STATE__ 的 interactInfo.commentCount。
func getTotalCommentCount(page playwright.Page) int {
	s := evalString(page, `() => {
		const m = window.__INITIAL_STATE__?.note?.noteDetailMap;
		if (!m) return "";
		for (const v of Object.values(m)) {
			const c = v?.note?.interactInfo?.commentCount;
			if (c !== undefined && c !== null) return String(c);
		}
		return "";
	}`)
	count, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return count
}

func checkNoCommentsArea(page playwright.Page) bool {
	el, err := queryWithTimeout(page, ".no-comments-text", 2*time.Second)
	if err != nil || el == nil {
		return false
	}
	text, err := el.InnerText()
	if err != nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(text), "这是一片荒地")
}

func checkEndContainer(page playwright.Page) bool {
	el, err := queryWithTimeout(page, ".end-container", 2*time.Second)
	if err != nil || el == nil {
		return false
	}
	text, err := el.InnerText()
	if err != nil {
		return false
	}
	textUpper := strings.ToUpper(strings.TrimSpace(text))
	return strings.Contains(textUpper, "THE END") || strings.Contains(textUpper, "THEEND")
}

// ========== 页面检查 ==========

func checkPageAccessible(page playwright.Page) error {
	time.Sleep(500 * time.Millisecond)

	wrapperEl, err := queryWithTimeout(page, ".access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper", 2*time.Second)
	if err != nil || wrapperEl == nil {
		return nil
	}

	text, err := wrapperEl.InnerText()
	if err != nil {
		return nil
	}

	keywords := []string{
		"当前笔记暂时无法浏览",
		"该内容因违规已被删除",
		"该笔记已被删除",
		"内容不存在",
		"笔记不存在",
		"已失效",
		"私密笔记",
		"仅作者可见",
		"因用户设置，你无法查看",
		"因违规无法查看",
	}

	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			logrus.Warnf("笔记不可访问: %s", kw)
			return fmt.Errorf("笔记不可访问: %s", kw)
		}
	}

	trimmedText := strings.TrimSpace(text)
	if trimmedText != "" {
		logrus.Warnf("笔记不可访问（未知原因）: %s", trimmedText)
		return fmt.Errorf("笔记不可访问: %s", trimmedText)
	}

	return nil
}

// ========== 数据提取 ==========

func (f *FeedDetailAction) extractFeedDetail(page playwright.Page, feedID string) (*FeedDetailResponse, error) {
	var result string

	err := retry.Do(
		func() error {
			evalResult := evalString(page, `() => {
				if (window.__INITIAL_STATE__ &&
					window.__INITIAL_STATE__.note &&
					window.__INITIAL_STATE__.note.noteDetailMap) {
					const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
					return JSON.stringify(noteDetailMap);
				}
				return "";
			}`)
			if evalResult != "" {
				result = evalResult
				return nil
			}
			return fmt.Errorf("无法获取初始状态数据")
		},
		retry.Attempts(3),
		retry.Delay(200*time.Millisecond),
		retry.MaxJitter(300*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("提取Feed详情重试 #%d: %v", n, err)
		}),
	)

	if err != nil {
		logrus.Errorf("提取Feed详情失败: %v", err)
		return nil, fmt.Errorf("提取Feed详情失败: %w", err)
	}

	if result == "" {
		return nil, errors.ErrNoFeedDetail
	}

	var noteDetailMap map[string]struct {
		Note     FeedDetail  `json:"note"`
		Comments CommentList `json:"comments"`
	}

	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal noteDetailMap: %w", err)
	}

	noteDetail, exists := noteDetailMap[feedID]
	if !exists {
		return nil, fmt.Errorf("feed %s not found in noteDetailMap", feedID)
	}

	return &FeedDetailResponse{
		Note:     noteDetail.Note,
		Comments: noteDetail.Comments,
	}, nil
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}
