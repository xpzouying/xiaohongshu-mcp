package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// CommentLoadConfig 评论加载配置
type CommentLoadConfig struct {
	// 是否点击"更多回复"按钮
	ClickMoreReplies bool
	// 回复数量阈值，超过这个数量的"更多"按钮将被跳过（0表示不跳过任何）
	MaxRepliesThreshold int
	// 最大加载评论数（comment-item数量），0表示加载所有
	MaxCommentItems int
	// 滚动速度等级: slow(慢速), normal(正常), fast(快速)
	ScrollSpeed string
}

// DefaultCommentLoadConfig 默认配置
func DefaultCommentLoadConfig() CommentLoadConfig {
	return CommentLoadConfig{
		ClickMoreReplies:    false, // 默认不点击"更多回复"
		MaxRepliesThreshold: 10,    // 默认超过10条回复就跳过
		MaxCommentItems:     0,     // 默认加载所有评论
		ScrollSpeed:         "normal",
	}
}

// FeedDetailAction 表示 Feed 详情页动作
type FeedDetailAction struct {
	page *rod.Page
}

// NewFeedDetailAction 创建 Feed 详情页动作
func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// GetFeedDetail 获取 Feed 详情页数据
func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config CommentLoadConfig) (*FeedDetailResponse, error) {
	return f.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, config)
}

// GetFeedDetailWithConfig 获取 Feed 详情页数据（带配置）
func (f *FeedDetailAction) GetFeedDetailWithConfig(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config CommentLoadConfig) (*FeedDetailResponse, error) {
	page := f.page.Context(ctx).Timeout(10 * time.Minute)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)
	logrus.Infof("配置: 点击更多=%v, 回复阈值=%d, 最大评论数=%d, 滚动速度=%s",
		config.ClickMoreReplies, config.MaxRepliesThreshold, config.MaxCommentItems, config.ScrollSpeed)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	// 检测页面是否不可访问
	if err := checkPageAccessible(page); err != nil {
		return nil, err
	}

	// 加载全部评论
	if loadAllComments {
		if err := f.loadAllCommentsWithConfig(page, config); err != nil {
			logrus.Warnf("加载全部评论失败: %v", err)
		}
	}

	// 提取笔记详情数据
	return f.extractFeedDetail(page, feedID)
}

// checkPageAccessible 检查页面是否可访问
func checkPageAccessible(page *rod.Page) error {
	unavailableResult := page.MustEval(`() => {
		const wrapper = document.querySelector('.access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper');
		if (!wrapper) return null;

		const text = wrapper.textContent || '';
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
				return kw.trim();
			}
		}
		return null;
	}`)

	rawJSON, err := unavailableResult.MarshalJSON()
	if err != nil {
		logrus.Errorf("无法解析页面状态检查的结果: %v", err)
		return fmt.Errorf("无法解析页面状态检查的结果: %w", err)
	}

	if string(rawJSON) != "null" {
		var reason string
		if err := json.Unmarshal(rawJSON, &reason); err == nil {
			logrus.Warnf("笔记不可访问: %s", reason)
			return fmt.Errorf("笔记不可访问: %s", reason)
		}
		rawReason := string(rawJSON)
		logrus.Warnf("笔记不可访问，且无法解析原因: %s", rawReason)
		return fmt.Errorf("笔记不可访问，无法解析原因: %s", rawReason)
	}

	return nil
}

// loadAllCommentsWithConfig 加载所有评论（带配置）
func (f *FeedDetailAction) loadAllCommentsWithConfig(page *rod.Page, config CommentLoadConfig) error {
	maxAttempts := 500
	if config.MaxCommentItems > 0 {
		// 如果设置了最大评论数，减少尝试次数
		maxAttempts = config.MaxCommentItems * 3
	}

	const (
		stagnantLimit       = 20
		noScrollChangeLimit = 15
		minScrollDelta      = 10
	)

	// 获取滚动间隔（根据速度）
	scrollInterval := getScrollInterval(config.ScrollSpeed)

	logrus.Info("开始加载评论...")

	// 先滚动到评论区
	scrollToCommentsArea(page)
	humanDelay()

	var (
		lastCount           = 0
		lastScrollTop       = 0
		stagnantChecks      = 0
		noScrollChangeCount = 0
		totalClickedButtons = 0
		skippedButtons      = 0
		attempt             = 0
	)

	for attempt = 0; attempt < maxAttempts; attempt++ {
		logrus.Debugf("=== 尝试 %d/%d ===", attempt+1, maxAttempts)

		// === 1. 检查是否到达底部 ===
		if checkEndContainer(page) {
			logrus.Infof("✓ 检测到 'THE END' 元素，已滑动到底部")
			humanDelay()

			currentCount := getCommentCount(page)
			logrus.Infof("✓ 加载完成: %d 条评论, 尝试次数: %d, 点击: %d, 跳过: %d",
				currentCount, attempt+1, totalClickedButtons, skippedButtons)
			return nil
		}

		// === 2. 获取当前评论数 ===
		currentCount := getCommentCount(page)

		// === 3. 点击"更多"按钮（人性化：每隔几次尝试才点击一次） ===
		if config.ClickMoreReplies && attempt%3 == 0 {
			clicked, skipped := clickShowMoreButtonsSmart(page, config.MaxRepliesThreshold)
			if clicked > 0 || skipped > 0 {
				totalClickedButtons += clicked
				skippedButtons += skipped
				logrus.Infof("点击'更多': %d 个, 跳过: %d 个, 累计点击: %d, 累计跳过: %d",
					clicked, skipped, totalClickedButtons, skippedButtons)

				// 点击后等待更长时间，模拟人阅读新内容（800-1500ms）
				readTime := time.Duration(800+rand.Intn(700)) * time.Millisecond
				time.Sleep(readTime)

				// 多轮检查（但减少轮数，避免太频繁）
				for round := 0; round < 1; round++ {
					// 等待一段时间再检查（模拟人继续浏览）
					time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)
					clicked2, skipped2 := clickShowMoreButtonsSmart(page, config.MaxRepliesThreshold)
					if clicked2 > 0 || skipped2 > 0 {
						totalClickedButtons += clicked2
						skippedButtons += skipped2
						logrus.Infof("第 %d 轮: 点击 %d, 跳过 %d", round+2, clicked2, skipped2)
						// 再次等待阅读时间
						readTime2 := time.Duration(600+rand.Intn(600)) * time.Millisecond
						time.Sleep(readTime2)
					} else {
						break
					}
				}
			}
		}

		// === 4. 获取评论数量 ===
		totalCount := getTotalCommentCount(page)
		logrus.Debugf("当前评论: %d, 目标: %d", currentCount, totalCount)

		// === 5. 检查评论数量变化 ===
		if currentCount != lastCount {
			logrus.Infof("✓ 评论增加: %d -> %d (+%d)", lastCount, currentCount, currentCount-lastCount)
			lastCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
			if stagnantChecks%5 == 0 {
				logrus.Debugf("评论停滞 %d 次", stagnantChecks)
			}
		}

		// === 5.1 检查是否已达到目标评论数（在评论数停滞时）===
		if config.MaxCommentItems > 0 && currentCount >= config.MaxCommentItems {
			// 达到目标且停滞2次，确认加载完成
			if stagnantChecks >= 2 {
				logrus.Infof("✓ 已达到目标评论数: %d/%d (停滞%d次), 停止加载",
					currentCount, config.MaxCommentItems, stagnantChecks)
				return nil
			}
			// 刚达到目标，继续滚动确认
			if stagnantChecks > 0 {
				logrus.Debugf("已达目标数 %d/%d，再确认 %d 次...",
					currentCount, config.MaxCommentItems, 2-stagnantChecks)
			}
		}

		// === 6. 停滞处理 ===
		if stagnantChecks >= stagnantLimit {
			logrus.Infof("评论停滞，尝试最后冲刺...")
			finalPush(page, config.ScrollSpeed)

			if checkEndContainer(page) {
				logrus.Infof("✓ 到达底部，评论数: %d", currentCount)
				return nil
			}

			logrus.Infof("未到底部，重置停滞计数")
			stagnantChecks = 0
		}

		// === 7. 执行人性化滚动 ===
		// 先滚动到最后一个评论（触发懒加载的关键！）
		if currentCount > 0 {
			scrollToLastComment(page)
			time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)
		}
		
		_, scrollDelta, currentScrollTop := humanScroll(page, config.ScrollSpeed)

		// === 8. 检查滚动变化 ===
		if scrollDelta < minScrollDelta || currentScrollTop == lastScrollTop {
			noScrollChangeCount++
			if noScrollChangeCount%5 == 0 {
				logrus.Debugf("滚动停滞 %d 次", noScrollChangeCount)
				largeScroll(page, config.ScrollSpeed)
				humanDelay()
			}
		} else {
			noScrollChangeCount = 0
			lastScrollTop = currentScrollTop
		}

		// === 9. 滚动停滞处理 ===
		if noScrollChangeCount >= noScrollChangeLimit {
			logrus.Infof("滚动停滞，最后冲刺...")
			finalPush(page, config.ScrollSpeed)

			if checkEndContainer(page) {
				logrus.Infof("✓ 到达底部，评论数: %d", currentCount)
				return nil
			}

			logrus.Infof("重置滚动计数")
			noScrollChangeCount = 0
			lastScrollTop = 0
		}

		// === 10. 等待内容加载 ===
		time.Sleep(scrollInterval)
	}

	// === 11. 最后冲刺 ===
	logrus.Infof("达到最大尝试次数，最后冲刺...")
	finalPush(page, config.ScrollSpeed)

	currentCount := getCommentCount(page)
	hasEnd := checkEndContainer(page)

	logrus.Infof("✓ 加载结束: %d 条评论, 点击: %d, 跳过: %d, 到达底部: %v",
		currentCount, totalClickedButtons, skippedButtons, hasEnd)

	return nil
}

// getScrollInterval 根据速度获取滚动间隔
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

// humanDelay 人性化延迟
func humanDelay() {
	delay := time.Duration(300+rand.Intn(400)) * time.Millisecond
	time.Sleep(delay)
}

// clickShowMoreButtonsSmart 智能点击"更多"按钮（根据回复数量判断，人性化操作）
func clickShowMoreButtonsSmart(page *rod.Page, maxRepliesThreshold int) (clicked, skipped int) {
	elements, err := page.Elements(".show-more")
	if err != nil {
		return 0, 0
	}

	// 正则表达式：匹配"展开 X 条回复"
	replyCountRegex := regexp.MustCompile(`展开\s*(\d+)\s*条回复`)

	// 限制每次最多点击的按钮数量（模拟人不会一次性点击太多）
	maxClickPerRound := 3 + rand.Intn(3) // 每次3-5个
	clickedInRound := 0

	for _, el := range elements {
		// 限制单次点击数量
		if clickedInRound >= maxClickPerRound {
			break
		}

		// 检查元素是否可见
		visible, err := el.Visible()
		if err != nil || !visible {
			continue
		}

		// 检查是否在 DOM 中
		box, err := el.Shape()
		if err != nil || len(box.Quads) == 0 {
			continue
		}

		// 获取按钮文本
		text, err := el.Text()
		if err != nil {
			continue
		}

		// 判断是否需要跳过
		shouldSkip := false
		if maxRepliesThreshold > 0 {
			matches := replyCountRegex.FindStringSubmatch(text)
			if len(matches) > 1 {
				replyCount, err := strconv.Atoi(matches[1])
				if err == nil && replyCount > maxRepliesThreshold {
					shouldSkip = true
					logrus.Debugf("跳过'%s'（回复数 %d > 阈值 %d）", text, replyCount, maxRepliesThreshold)
				}
			}
		}

		if shouldSkip {
			skipped++
			continue
		}

		// === 人性化点击流程 ===
		// 1. 先滚动到元素附近（模拟人看到按钮）
		el.MustEval(`() => {
			try {
				this.scrollIntoView({behavior: 'smooth', block: 'center'});
			} catch (e) {}
		}`)

		// 2. 等待滚动完成 + 模拟人看到按钮后的反应时间（300-800ms）
		reactionTime := time.Duration(300+rand.Intn(500)) * time.Millisecond
		time.Sleep(reactionTime)

		// 3. 模拟鼠标移动到按钮上（悬停效果）
		box, _ = el.Shape()
		if len(box.Quads) > 0 {
			// 计算按钮中心点
			x := float64(box.Quads[0][0]+box.Quads[0][4]) / 2
			y := float64(box.Quads[0][1]+box.Quads[0][5]) / 2
			page.Mouse.MustMoveTo(x, y)
			// 悬停时间（模拟人确认要点击）
			time.Sleep(time.Duration(100+rand.Intn(200)) * time.Millisecond)
		}

		// 4. 点击元素
		if err := el.Click(proto.InputMouseButtonLeft, 1); err == nil {
			clicked++
			clickedInRound++
			logrus.Debugf("点击了'%s'", text)

			// 5. 点击后的延迟（模拟人阅读新内容的时间，500-1200ms）
			readTime := time.Duration(500+rand.Intn(700)) * time.Millisecond
			time.Sleep(readTime)
		}
	}

	return clicked, skipped
}

// humanScroll 人性化滚动
func humanScroll(page *rod.Page, speed string) (bool, int, int) {
	beforeTop := getScrollTop(page)
	viewportHeight := page.MustEval(`() => window.innerHeight`).Int()

	// 根据速度调整滚动距离
	var scrollRatio float64
	switch speed {
	case "slow":
		scrollRatio = 0.5 + rand.Float64()*0.2 // 50%-70%
	case "fast":
		scrollRatio = 0.9 + rand.Float64()*0.2 // 90%-110%
	default: // normal
		scrollRatio = 0.7 + rand.Float64()*0.2 // 70%-90%
	}

	scrollDelta := float64(viewportHeight) * scrollRatio
	if scrollDelta < 400 {
		scrollDelta = 400
	}

	// 添加随机波动
	scrollDelta += float64(rand.Intn(100) - 50)

	// 使用JS的 scrollBy 方法进行滚动
	page.MustEval(`(delta) => { window.scrollBy(0, delta); }`, scrollDelta)

	// 等待滚动完成
	time.Sleep(time.Duration(100+rand.Intn(100)) * time.Millisecond)

	afterTop := getScrollTop(page)
	actualDelta := afterTop - beforeTop
	scrolled := actualDelta > 5

	if scrolled {
		logrus.Debugf("滚动: %d -> %d (Δ%d)", beforeTop, afterTop, actualDelta)
	}

	return scrolled, actualDelta, afterTop
}

// scrollToCommentsArea 滚动到评论区
func scrollToCommentsArea(page *rod.Page) {
	logrus.Info("滚动到评论区...")
	page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (container) {
			container.scrollIntoView({behavior: 'smooth', block: 'start'});
		}
	}`)
}

// scrollToLastComment 滚动到最后一个评论（触发懒加载的关键）
func scrollToLastComment(page *rod.Page) {
	page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (!container) return;
		
		// 查找最后一个主评论
		const comments = container.querySelectorAll('.parent-comment');
		if (comments.length > 0) {
			const lastComment = comments[comments.length - 1];
			// 滚动到最后一个评论，让它出现在视口中间偏下位置
			lastComment.scrollIntoView({behavior: 'smooth', block: 'center'});
		}
	}`)
}

// finalPush 最后冲刺：大幅滚动到底部
func finalPush(page *rod.Page, speed string) {
	logrus.Info("执行最后冲刺...")

	for i := 0; i < 15; i++ {
		if checkEndContainer(page) {
			return
		}

		beforeTop := getScrollTop(page)
		largeScroll(page, speed)

		// 人性化延迟
		time.Sleep(time.Duration(200+rand.Intn(200)) * time.Millisecond)

		afterTop := getScrollTop(page)
		if afterTop == beforeTop {
			page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
			time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)
		}
	}
}

// largeScroll 大幅度滚动
func largeScroll(page *rod.Page, speed string) {
	var scrollDelta float64
	switch speed {
	case "slow":
		scrollDelta = 1000 + float64(rand.Intn(500))
	case "fast":
		scrollDelta = 3000 + float64(rand.Intn(1000))
	default: // normal
		scrollDelta = 2000 + float64(rand.Intn(500))
	}

	page.MustEval(`(delta) => { window.scrollBy(0, delta); }`, scrollDelta)
	time.Sleep(time.Duration(100+rand.Intn(50)) * time.Millisecond)
}

// getScrollTop 获取当前滚动位置
func getScrollTop(page *rod.Page) int {
	result := page.MustEval(`() => {
		return window.pageYOffset || document.documentElement.scrollTop || document.body.scrollTop || 0;
	}`)
	return result.Int()
}

// getCommentCount 获取当前评论数量
func getCommentCount(page *rod.Page) int {
	result := page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (!container) return 0;
		return container.querySelectorAll('.parent-comment').length;
	}`)
	return result.Int()
}

// getTotalCommentCount 获取总评论数
func getTotalCommentCount(page *rod.Page) int {
	result := page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (!container) return 0;
		
		const totalEl = container.querySelector('.total');
		if (!totalEl) return 0;
		
		const text = (totalEl.textContent || '').replace(/\s+/g, '');
		const match = text.match(/共(\d+)条评论/);
		return match ? parseInt(match[1], 10) : 0;
	}`)
	return result.Int()
}

// checkEndContainer 检查是否出现 "THE END" 元素
func checkEndContainer(page *rod.Page) bool {
	result := page.MustEval(`() => {
		const endContainer = document.querySelector('.end-container');
		if (!endContainer) return false;
		
		const text = (endContainer.textContent || '').trim().toUpperCase();
		return text.includes('THE END') || text.includes('THEEND');
	}`)
	return result.Bool()
}

// extractFeedDetail 提取 Feed 详情数据
func (f *FeedDetailAction) extractFeedDetail(page *rod.Page, feedID string) (*FeedDetailResponse, error) {
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.note &&
		    window.__INITIAL_STATE__.note.noteDetailMap) {
			const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
			return JSON.stringify(noteDetailMap);
		}
		return "";
	}`).String()

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
