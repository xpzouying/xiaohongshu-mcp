package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/go-rod/rod"
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
	largeScrollTrigger   = 5 // 停滞多少次后触发大滚动
	buttonClickInterval  = 3 // 每隔多少次尝试点击一次按钮
	finalSprintPushCount = 15
)

// ========== 数据结构 ==========

type CommentLoadConfig struct {
	ClickMoreReplies    bool
	MaxRepliesThreshold int
	MaxCommentItems     int
	ScrollSpeed         string
}

func DefaultCommentLoadConfig() CommentLoadConfig {
	return CommentLoadConfig{
		ClickMoreReplies:    false,
		MaxRepliesThreshold: 10,
		MaxCommentItems:     0,
		ScrollSpeed:         "normal",
	}
}

type FeedDetailAction struct {
	page *rod.Page
}

func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// ========== 主要业务逻辑 ==========

func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config CommentLoadConfig) (*FeedDetailResponse, error) {
	return f.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, config)
}

func (f *FeedDetailAction) GetFeedDetailWithConfig(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config CommentLoadConfig) (*FeedDetailResponse, error) {
	page := f.page.Context(ctx).Timeout(10 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("打开 feed 详情页: %s", url)
	logrus.Infof("配置: 点击更多=%v, 回复阈值=%d, 最大评论数=%d, 滚动速度=%s",
		config.ClickMoreReplies, config.MaxRepliesThreshold, config.MaxCommentItems, config.ScrollSpeed)

	// 使用retry-go处理页面导航和DOM稳定等待
	err := retry.Do(
		func() error {
			page.MustNavigate(url)
			page.MustWaitDOMStable()
			return nil
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

	// ctx 已取消时直接返回，避免在已取消的 page 上执行 MustEval 触发 panic
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return f.extractFeedDetail(page, feedID)
}

// ========== 评论加载器 ==========

type commentLoader struct {
	page   *rod.Page
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

func (f *FeedDetailAction) loadAllCommentsWithConfig(ctx context.Context, page *rod.Page, config CommentLoadConfig) error {
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

	// 检查是否没有评论
	if cl.checkNoComments() {
		return nil
	}

	for cl.stats.attempts = 0; cl.stats.attempts < maxAttempts; cl.stats.attempts++ {
		// 协作取消点：ctx 取消后干净退出，避免空转直到撞上 MustEval panic
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
	if checkEndContainer(cl.page) {
		currentCount := getCommentCount(cl.page)
		logrus.Infof("✓ 检测到 'THE END' 元素，已滑动到底部")
		humanize.Delay(ctx, humanize.BetweenScroll)
		logrus.Infof("✓ 加载完成: %d 条评论, 尝试次数: %d, 点击: %d, 跳过: %d",
			currentCount, cl.stats.attempts+1, cl.stats.totalClicked, cl.stats.totalSkipped)
		return true
	}
	return false
}

func (cl *commentLoader) shouldClickButtons() bool {
	return cl.config.ClickMoreReplies && cl.stats.attempts%buttonClickInterval == 0
}

func (cl *commentLoader) clickButtonsWithRetry(ctx context.Context) {
	clicked, skipped := clickShowMoreButtonsSmart(ctx, cl.page, cl.config.MaxRepliesThreshold)
	if clicked > 0 || skipped > 0 {
		cl.stats.totalClicked += clicked
		cl.stats.totalSkipped += skipped
		logrus.Infof("点击'更多': %d 个, 跳过: %d 个, 累计点击: %d, 累计跳过: %d",
			clicked, skipped, cl.stats.totalClicked, cl.stats.totalSkipped)

		humanize.Delay(ctx, humanize.Reading)

		// 重试一轮
		clicked2, skipped2 := clickShowMoreButtonsSmart(ctx, cl.page, cl.config.MaxRepliesThreshold)
		if clicked2 > 0 || skipped2 > 0 {
			cl.stats.totalClicked += clicked2
			cl.stats.totalSkipped += skipped2
			logrus.Infof("第 2 轮: 点击 %d, 跳过 %d", clicked2, skipped2)
			humanize.Delay(ctx, humanize.Reading)
		}
	}
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
	// 如果未设置最大评论数，或者还未达到目标，继续加载
	if cl.config.MaxCommentItems <= 0 {
		return false
	}

	// 如果已达到或超过目标评论数，立即停止
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
		time.Sleep(400 * time.Millisecond) // 技术 settle：等 scrollIntoView 动画落位
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

func clickShowMoreButtonsSmart(ctx context.Context, page *rod.Page, maxRepliesThreshold int) (clicked, skipped int) {
	elements, err := page.Elements(".show-more")
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

		text, err := el.Text()
		if err != nil {
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

func isElementClickable(el *rod.Element) bool {
	visible, err := el.Visible()
	if err != nil || !visible {
		return false
	}

	box, err := el.Shape()
	return err == nil && len(box.Quads) > 0
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

func clickElementWithHumanBehavior(ctx context.Context, page *rod.Page, el *rod.Element, text string) bool {
	var clickSuccess bool

	// 使用retry-go进行点击操作重试
	err := retry.Do(
		func() error {
			// 滚动到元素
			el.MustEval(`() => {
				try {
					this.scrollIntoView({behavior: 'smooth', block: 'center'});
				} catch (e) {}
			}`)

			humanize.Delay(ctx, humanize.Reading)

			if box, err := el.Shape(); err == nil && len(box.Quads) > 0 {
				x := float64(box.Quads[0][0]+box.Quads[0][4]) / 2
				y := float64(box.Quads[0][1]+box.Quads[0][5]) / 2
				page.Mouse.MustMoveTo(x, y)
				humanize.Delay(ctx, humanize.BeforeClick)
			}

			// 点击
			if err := humanize.Click(el); err != nil {
				return err // 返回错误以触发重试
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

func humanScroll(ctx context.Context, page *rod.Page, speed string, largeMode bool, pushCount int) (bool, int, int) {
	beforeTop := getScrollTop(page)
	viewportHeight := page.MustEval(`() => window.innerHeight`).Int()

	baseRatio := getScrollRatio(speed)
	if largeMode {
		baseRatio *= 2.0
	}

	scrolled := false
	actualDelta := 0
	currentScrollTop := beforeTop

	for i := 0; i < max(1, pushCount); i++ {
		scrollDelta := calculateScrollDelta(viewportHeight, baseRatio)
		page.MustEval(`(delta) => { window.scrollBy(0, delta); }`, scrollDelta)

		time.Sleep(150 * time.Millisecond) // 技术 settle：等 scrollBy 后懒加载渲染，再读 scrollTop

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

	if !scrolled && pushCount > 0 {
		page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
		time.Sleep(400 * time.Millisecond) // 技术 settle：等 scrollTo 落位
		currentScrollTop = getScrollTop(page)
		actualDelta = currentScrollTop - beforeTop + actualDelta
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
	default: // normal
		return 0.7
	}
}

func calculateScrollDelta(viewportHeight int, baseRatio float64) float64 {
	scrollDelta := float64(viewportHeight) * (baseRatio + rand.Float64()*0.2)
	if scrollDelta < 400 {
		scrollDelta = 400
	}
	return scrollDelta + float64(rand.Intn(100)-50)
}

func scrollToCommentsArea(page *rod.Page) {
	logrus.Info("滚动到评论区...")

	// 先定位到评论区
	if el, err := page.Timeout(2 * time.Second).Element(".comments-container"); err == nil {
		el.MustScrollIntoView()
	}
	// 等 scrollIntoView 动画落位
	time.Sleep(400 * time.Millisecond)

	// 触发一次小滚动，激活懒加载机制
	smartScroll(page, 100)
}

// smartScroll 智能滚动：触发滚轮事件以正确触发懒加载
func smartScroll(page *rod.Page, delta float64) {
	page.MustEval(`(delta) => {
		// 查找滚动目标元素
		let targetElement = document.querySelector('.note-scroller') 
			|| document.querySelector('.interaction-container') 
			|| document.documentElement;
		
		// 触发滚轮事件（关键！这样才能触发懒加载）
		const wheelEvent = new WheelEvent('wheel', {
			deltaY: delta,
			deltaMode: 0, // 像素模式
			bubbles: true,
			cancelable: true,
			view: window
		});
		targetElement.dispatchEvent(wheelEvent);
	}`, delta)
}

func scrollToLastComment(page *rod.Page) {
	// 获取所有主评论元素
	elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment")
	if err != nil || len(elements) == 0 {
		return
	}
	// 滚动到最后一个评论
	lastComment := elements[len(elements)-1]
	lastComment.MustScrollIntoView()
}

// ========== DOM 查询 ==========

func getScrollTop(page *rod.Page) int {
	var result int

	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			evalResult := page.MustEval(`() => {
				return window.pageYOffset || document.documentElement.scrollTop || document.body.scrollTop || 0;
			}`)

			result = evalResult.Int()
			return nil
		},
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.MaxJitter(200*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("获取滚动位置重试 #%d: %v", n, err)
		}),
	)

	if err != nil {
		logrus.Warnf("获取滚动位置失败: %v", err)
		return 0 // 失败时返回0
	}

	return result
}

func getCommentCount(page *rod.Page) int {
	var result int

	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			// 使用 Go 获取评论元素
			elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment")
			if err != nil {
				return err
			}
			result = len(elements)
			return nil
		},
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.MaxJitter(200*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("获取评论计数重试 #%d: %v", n, err)
		}),
	)

	if err != nil {
		logrus.Warnf("获取评论计数失败: %v", err)
		return 0 // 失败时返回0
	}

	return result
}

func getTotalCommentCount(page *rod.Page) int {
	var result int

	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			// 使用 Go 获取总评论数元素
			totalEl, err := page.Timeout(2 * time.Second).Element(".comments-container .total")
			if err != nil {
				return err
			}

			// 获取文本内容
			text, err := totalEl.Text()
			if err != nil {
				return err
			}

			// 使用正则提取数字
			re := regexp.MustCompile(`共(\d+)条评论`)
			matches := re.FindStringSubmatch(text)
			if len(matches) > 1 {
				count, err := strconv.Atoi(matches[1])
				if err != nil {
					return err
				}
				result = count
			} else {
				result = 0
			}

			return nil
		},
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.MaxJitter(200*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("获取总评论计数重试 #%d: %v", n, err)
		}),
	)

	if err != nil {
		logrus.Warnf("获取总评论计数失败: %v", err)
		return 0 // 失败时返回0
	}

	return result
}

func checkNoCommentsArea(page *rod.Page) bool {
	// 查找无评论区域
	noCommentsEl, err := page.Timeout(2 * time.Second).Element(".no-comments-text")
	if err != nil {
		// 未找到无评论元素，说明有评论或评论区正常
		return false
	}

	// 获取文本内容
	text, err := noCommentsEl.Text()
	if err != nil {
		return false
	}

	// 检查是否包含"这是一片荒地"等关键词
	text = strings.TrimSpace(text)
	return strings.Contains(text, "这是一片荒地")
}

func checkEndContainer(page *rod.Page) bool {
	var result bool

	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			// 使用 Go 查找结束容器
			endEl, err := page.Timeout(2 * time.Second).Element(".end-container")
			if err != nil {
				// 未找到元素，说明未到底部
				result = false
				return nil
			}

			// 获取文本内容
			text, err := endEl.Text()
			if err != nil {
				result = false
				return nil
			}

			// 转换为大写并检查
			textUpper := strings.ToUpper(strings.TrimSpace(text))
			result = strings.Contains(textUpper, "THE END") || strings.Contains(textUpper, "THEEND")
			return nil
		},
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.MaxJitter(200*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("检查结束容器重试 #%d: %v", n, err)
		}),
	)

	if err != nil {
		logrus.Warnf("检查结束容器失败: %v", err)
		return false // 失败时返回false
	}

	return result
}

// ========== 页面检查 ==========

func checkPageAccessible(page *rod.Page) error {
	// 等错误提示 UI 渲染出来再检查
	time.Sleep(500 * time.Millisecond)

	// 查找错误提示容器
	wrapperEl, err := page.Timeout(2 * time.Second).Element(".access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper")
	if err != nil {
		// 未找到错误容器，说明页面可访问
		return nil
	}

	// 获取文本内容
	text, err := wrapperEl.Text()
	if err != nil {
		// 无法获取文本，假设页面可访问
		return nil
	}

	// 检查关键词
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

	// 如果有文本但不匹配关键词，返回未知错误
	trimmedText := strings.TrimSpace(text)
	if trimmedText != "" {
		logrus.Warnf("笔记不可访问（未知原因）: %s", trimmedText)
		return fmt.Errorf("笔记不可访问: %s", trimmedText)
	}

	return nil
}

// ========== 数据提取 ==========

func (f *FeedDetailAction) extractFeedDetail(page *rod.Page, feedID string) (*FeedDetailResponse, error) {
	var result string

	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			evalResult := page.MustEval(`() => {
				if (window.__INITIAL_STATE__ &&
					window.__INITIAL_STATE__.note &&
					window.__INITIAL_STATE__.note.noteDetailMap) {
					const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
					return JSON.stringify(noteDetailMap);
				}
				return "";
			}`).String()

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

	// 视频笔记：video.mediaV2 是一个 JSON 字符串，官方字幕埋在 video.subtitles 里。
	// 解析出来挂到 Note.Video.Subtitles，让调用方可直接拿到字幕。
	enrichVideoSubtitles(&noteDetail.Note)

	return &FeedDetailResponse{
		Note:     noteDetail.Note,
		Comments: noteDetail.Comments,
	}, nil
}

// enrichVideoSubtitles 把 video.mediaV2（JSON 字符串）里的官方字幕解析到 Video.Subtitles，
// 并清空体积庞大的 RawMediaV2 原始串。解析失败不影响主流程（字幕只是增益）。
func enrichVideoSubtitles(note *FeedDetail) {
	if note == nil || note.Video == nil || note.Video.RawMediaV2 == "" {
		return
	}
	raw := note.Video.RawMediaV2
	note.Video.RawMediaV2 = "" // 不在响应里回传巨大的原始串

	var payload mediaV2Payload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		logrus.Debugf("解析 mediaV2 字幕失败（忽略）: %v", err)
		return
	}
	if len(payload.Video.Subtitles) > 0 {
		note.Video.Subtitles = payload.Video.Subtitles
		logrus.Infof("✓ 提取到官方字幕语种: %d 种", len(payload.Video.Subtitles))
	}
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}
