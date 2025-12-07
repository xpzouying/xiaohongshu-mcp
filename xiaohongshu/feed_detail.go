package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// ========== 配置常量 ==========
const (
	defaultMaxAttempts      = 500
	stagnantLimit           = 20
	minScrollDelta          = 10
	maxClickPerRound        = 3
	stagnantCheckThreshold  = 2  // 达到目标后需要停滞几次才确认
	largeScrollTrigger      = 5  // 停滞多少次后触发大滚动
	buttonClickInterval     = 3  // 每隔多少次尝试点击一次按钮
	finalSprintPushCount    = 15
)

// 延迟时间配置（毫秒）
type delayConfig struct {
	min, max int
}

var (
	humanDelayRange    = delayConfig{300, 700}
	reactionTimeRange  = delayConfig{300, 800}
	hoverTimeRange     = delayConfig{100, 300}
	readTimeRange      = delayConfig{500, 1200}
	shortReadRange     = delayConfig{600, 1200}
	scrollWaitRange    = delayConfig{100, 200}
	postScrollRange    = delayConfig{300, 500}
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
	sleepRandom(1000, 1000)
	
	if err := checkPageAccessible(page); err != nil {
		return nil, err
	}
	
	if loadAllComments {
		if err := f.loadAllCommentsWithConfig(page, config); err != nil {
			logrus.Warnf("加载全部评论失败: %v", err)
		}
	}
	
	return f.extractFeedDetail(page, feedID)
}

// ========== 评论加载器 ==========

type commentLoader struct {
	page                *rod.Page
	config              CommentLoadConfig
	stats               *loadStats
	state               *loadState
}

type loadStats struct {
	totalClicked  int
	totalSkipped  int
	attempts      int
}

type loadState struct {
	lastCount      int
	lastScrollTop  int
	stagnantChecks int
}

func (f *FeedDetailAction) loadAllCommentsWithConfig(page *rod.Page, config CommentLoadConfig) error {
	loader := &commentLoader{
		page:   page,
		config: config,
		stats:  &loadStats{},
		state:  &loadState{},
	}
	
	return loader.load()
}

func (cl *commentLoader) load() error {
	maxAttempts := cl.calculateMaxAttempts()
	scrollInterval := getScrollInterval(cl.config.ScrollSpeed)
	
	logrus.Info("开始加载评论...")
	scrollToCommentsArea(cl.page)
	sleepRandom(humanDelayRange.min, humanDelayRange.max)
	
	for cl.stats.attempts = 0; cl.stats.attempts < maxAttempts; cl.stats.attempts++ {
		logrus.Debugf("=== 尝试 %d/%d ===", cl.stats.attempts+1, maxAttempts)
		
		if cl.checkComplete() {
			return nil
		}
		
		if cl.shouldClickButtons() {
			cl.clickButtonsWithRetry()
		}
		
		currentCount := getCommentCount(cl.page)
		cl.updateState(currentCount)
		
		if cl.shouldStopAtTarget(currentCount) {
			return nil
		}
		
		cl.performScroll()
		cl.handleStagnation()
		
		time.Sleep(scrollInterval)
	}
	
	cl.performFinalSprint()
	return nil
}

func (cl *commentLoader) calculateMaxAttempts() int {
	if cl.config.MaxCommentItems > 0 {
		return cl.config.MaxCommentItems * 3
	}
	return defaultMaxAttempts
}

func (cl *commentLoader) checkComplete() bool {
	if checkEndContainer(cl.page) {
		currentCount := getCommentCount(cl.page)
		logrus.Infof("✓ 检测到 'THE END' 元素，已滑动到底部")
		sleepRandom(humanDelayRange.min, humanDelayRange.max)
		logrus.Infof("✓ 加载完成: %d 条评论, 尝试次数: %d, 点击: %d, 跳过: %d",
			currentCount, cl.stats.attempts+1, cl.stats.totalClicked, cl.stats.totalSkipped)
		return true
	}
	return false
}

func (cl *commentLoader) shouldClickButtons() bool {
	return cl.config.ClickMoreReplies && cl.stats.attempts%buttonClickInterval == 0
}

func (cl *commentLoader) clickButtonsWithRetry() {
	clicked, skipped := clickShowMoreButtonsSmart(cl.page, cl.config.MaxRepliesThreshold)
	if clicked > 0 || skipped > 0 {
		cl.stats.totalClicked += clicked
		cl.stats.totalSkipped += skipped
		logrus.Infof("点击'更多': %d 个, 跳过: %d 个, 累计点击: %d, 累计跳过: %d",
			clicked, skipped, cl.stats.totalClicked, cl.stats.totalSkipped)
		
		sleepRandom(readTimeRange.min, readTimeRange.max)
		
		// 重试一轮
		clicked2, skipped2 := clickShowMoreButtonsSmart(cl.page, cl.config.MaxRepliesThreshold)
		if clicked2 > 0 || skipped2 > 0 {
			cl.stats.totalClicked += clicked2
			cl.stats.totalSkipped += skipped2
			logrus.Infof("第 2 轮: 点击 %d, 跳过 %d", clicked2, skipped2)
			sleepRandom(shortReadRange.min, shortReadRange.max)
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
	if cl.config.MaxCommentItems <= 0 || currentCount < cl.config.MaxCommentItems {
		return false
	}
	
	if cl.state.stagnantChecks >= stagnantCheckThreshold {
		logrus.Infof("✓ 已达到目标评论数: %d/%d (停滞%d次), 停止加载",
			currentCount, cl.config.MaxCommentItems, cl.state.stagnantChecks)
		return true
	}
	
	if cl.state.stagnantChecks > 0 {
		logrus.Debugf("已达目标数 %d/%d，再确认 %d 次...",
			currentCount, cl.config.MaxCommentItems, stagnantCheckThreshold-cl.state.stagnantChecks)
	}
	
	return false
}

func (cl *commentLoader) performScroll() {
	currentCount := getCommentCount(cl.page)
	if currentCount > 0 {
		scrollToLastComment(cl.page)
		sleepRandom(postScrollRange.min, postScrollRange.max)
	}
	
	largeMode := cl.state.stagnantChecks >= largeScrollTrigger
	pushCount := 1
	if largeMode {
		pushCount = 3 + rand.Intn(3)
	}
	
	_, scrollDelta, currentScrollTop := humanScroll(cl.page, cl.config.ScrollSpeed, largeMode, pushCount)
	
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

func (cl *commentLoader) handleStagnation() {
	if cl.state.stagnantChecks >= stagnantLimit {
		logrus.Infof("停滞过多，尝试大冲刺...")
		humanScroll(cl.page, cl.config.ScrollSpeed, true, 10)
		cl.state.stagnantChecks = 0
		
		if checkEndContainer(cl.page) {
			currentCount := getCommentCount(cl.page)
			logrus.Infof("✓ 到达底部，评论数: %d", currentCount)
		}
	}
}

func (cl *commentLoader) performFinalSprint() {
	logrus.Infof("达到最大尝试次数，最后冲刺...")
	humanScroll(cl.page, cl.config.ScrollSpeed, true, finalSprintPushCount)
	
	currentCount := getCommentCount(cl.page)
	hasEnd := checkEndContainer(cl.page)
	logrus.Infof("✓ 加载结束: %d 条评论, 点击: %d, 跳过: %d, 到达底部: %v",
		currentCount, cl.stats.totalClicked, cl.stats.totalSkipped, hasEnd)
}

// ========== 工具函数 ==========

func sleepRandom(minMs, maxMs int) {
	if maxMs <= minMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}
	delay := time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond
	time.Sleep(delay)
}

func getScrollInterval(speed string) time.Duration {
	switch speed {
	case "slow":
		return time.Duration(1200+rand.Intn(300)) * time.Millisecond
	case "fast":
		return time.Duration(300+rand.Intn(100)) * time.Millisecond
	default: // normal
		return time.Duration(600+rand.Intn(200)) * time.Millisecond
	}
}

// ========== 按钮点击 ==========

func clickShowMoreButtonsSmart(page *rod.Page, maxRepliesThreshold int) (clicked, skipped int) {
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
		
		if clickElementWithHumanBehavior(page, el, text) {
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

func clickElementWithHumanBehavior(page *rod.Page, el *rod.Element, text string) bool {
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
			
			sleepRandom(reactionTimeRange.min, reactionTimeRange.max)
			
			// 鼠标悬停
			if box, err := el.Shape(); err == nil && len(box.Quads) > 0 {
				x := float64(box.Quads[0][0]+box.Quads[0][4]) / 2
				y := float64(box.Quads[0][1]+box.Quads[0][5]) / 2
				page.Mouse.MustMoveTo(x, y)
				sleepRandom(hoverTimeRange.min, hoverTimeRange.max)
			}
			
			// 点击
			if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
				return err // 返回错误以触发重试
			}
			
			// 模拟人类阅读时间
			sleepRandom(readTimeRange.min, readTimeRange.max)
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

func humanScroll(page *rod.Page, speed string, largeMode bool, pushCount int) (bool, int, int) {
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
		
		sleepRandom(scrollWaitRange.min, scrollWaitRange.max)
		
		currentScrollTop = getScrollTop(page)
		deltaThisTime := currentScrollTop - beforeTop
		actualDelta += deltaThisTime
		
		if deltaThisTime > 5 {
			scrolled = true
		}
		
		beforeTop = currentScrollTop
		
		if i < pushCount-1 {
			sleepRandom(humanDelayRange.min, humanDelayRange.max)
		}
	}
	
	if !scrolled && pushCount > 0 {
		page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
		sleepRandom(postScrollRange.min, postScrollRange.max)
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
	page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (container) {
			container.scrollIntoView({behavior: 'smooth', block: 'start'});
		}
	}`)
}

func scrollToLastComment(page *rod.Page) {
	page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (!container) return;
		const comments = container.querySelectorAll('.parent-comment');
		if (comments.length > 0) {
			const lastComment = comments[comments.length - 1];
			lastComment.scrollIntoView({behavior: 'smooth', block: 'center'});
		}
	}`)
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
			evalResult := page.MustEval(`() => {
				const container = document.querySelector('.comments-container');
				if (!container) return 0;
				return container.querySelectorAll('.parent-comment').length;
			}`)
			
			result = evalResult.Int()
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
			evalResult := page.MustEval(`() => {
				const container = document.querySelector('.comments-container');
				if (!container) return 0;
				const totalEl = container.querySelector('.total');
				if (!totalEl) return 0;
				const text = (totalEl.textContent || '').replace(/\s+/g, '');
				const match = text.match(/共(\d+)条评论/);
				return match ? parseInt(match[1], 10) : 0;
			}`)
			
			result = evalResult.Int()
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

func checkEndContainer(page *rod.Page) bool {
	var result bool
	
	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			evalResult := page.MustEval(`() => {
				const endContainer = document.querySelector('.end-container');
				if (!endContainer) return false;
				const text = (endContainer.textContent || '').trim().toUpperCase();
				return text.includes('THE END') || text.includes('THEEND');
			}`)
			
			result = evalResult.Bool()
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
	time.Sleep(500 * time.Millisecond)
	
	// 使用retry-go来处理可能的DOM查询失败
	err := retry.Do(
		func() error {
			result := page.MustEval(`() => {
				const wrapper = document.querySelector('.access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper');
				if (!wrapper) return null;
				
				const text = wrapper.textContent || wrapper.innerText || '';
				const keywords = [
					'当前笔记暂时无法浏览',
					'该内容因违规已被删除',
					'该笔记已被删除',
					'内容不存在',
					'笔记不存在',
					'已失效',
					'私密笔记',
					'仅作者可见',
					'因用户设置，你无法查看',
					'因违规无法查看'
				];
				
				for (const kw of keywords) {
					if (text.includes(kw)) {
						return kw;
					}
				}
				
				if (text.trim()) {
					return '未知错误: ' + text.trim();
				}
				return null;
			}`)
			
			rawJSON, marshalErr := result.MarshalJSON()
			if marshalErr != nil {
				return fmt.Errorf("无法序列化页面状态检查结果: %w", marshalErr)
			}
			
			if string(rawJSON) != "null" {
				var reason string
				if unmarshalErr := json.Unmarshal(rawJSON, &reason); unmarshalErr == nil {
					logrus.Warnf("笔记不可访问: %s", reason)
					return fmt.Errorf("笔记不可访问: %s", reason)
				}
				
				rawReason := string(rawJSON)
				logrus.Warnf("笔记不可访问，且无法解析原因: %s", rawReason)
				return fmt.Errorf("笔记不可访问，无法解析原因: %s", rawReason)
			}
			
			return nil
		},
		retry.Attempts(3),
		retry.Delay(200*time.Millisecond),
		retry.MaxJitter(300*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			logrus.Debugf("页面可访问性检查重试 #%d: %v", n, err)
		}),
	)
	
	// If the error is nil, it means no access issue was found
	if err == nil {
		return nil // Page is accessible
	}
	
	// Return the original error from the retry operation
	return err
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
	
	return &FeedDetailResponse{
		Note:     noteDetail.Note,
		Comments: noteDetail.Comments,
	}, nil
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}