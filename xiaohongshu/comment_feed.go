package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// CommentFeedAction 表示 Feed 评论动作
type CommentFeedAction struct {
	page *rod.Page
}

// NewCommentFeedAction 创建 Feed 评论动作
func NewCommentFeedAction(page *rod.Page) *CommentFeedAction {
	return &CommentFeedAction{page: page}
}

// PostComment 发表评论到 Feed
func (f *CommentFeedAction) PostComment(ctx context.Context, feedID, xsecToken, content string) error {
	// 不使用 Context(ctx)，避免继承外部 context 的超时
	page := f.page.Timeout(120 * time.Second)

	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	elem, err := page.Element("div.input-box div.content-edit span")
	if err != nil {
		logrus.Warnf("Failed to find comment input box: %v", err)
		return fmt.Errorf("未找到评论输入框，该帖子可能不支持评论或网页端不可访问: %w", err)
	}

	if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
		logrus.Warnf("Failed to click comment input box: %v", err)
		return fmt.Errorf("无法点击评论输入框: %w", err)
	}

	elem2, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		logrus.Warnf("Failed to find comment input field: %v", err)
		return fmt.Errorf("未找到评论输入区域: %w", err)
	}

	if err := elem2.Input(content); err != nil {
		logrus.Warnf("Failed to input comment content: %v", err)
		return fmt.Errorf("无法输入评论内容: %w", err)
	}

	time.Sleep(1 * time.Second)

	submitButton, err := page.Element("div.bottom button.submit")
	if err != nil {
		logrus.Warnf("Failed to find submit button: %v", err)
		return fmt.Errorf("未找到提交按钮: %w", err)
	}

	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		logrus.Warnf("Failed to click submit button: %v", err)
		return fmt.Errorf("无法点击提交按钮: %w", err)
	}

	time.Sleep(1 * time.Second)

	logrus.Infof("Comment posted successfully to feed: %s", feedID)
	return nil
}

// ReplyTarget 表示一个回复目标
type ReplyTarget struct {
	CommentID string // 评论ID
	UserID    string // 用户ID
	Content   string // 回复内容
}

// BatchReplyResult 批量回复结果
type BatchReplyResult struct {
	CommentID string // 评论ID
	UserID    string // 用户ID
	Success   bool   // 是否成功
	Error     string // 错误信息（如果失败）
}

// BatchReplyToComments 批量回复多个评论
func (f *CommentFeedAction) BatchReplyToComments(ctx context.Context, feedID, xsecToken string, targets []ReplyTarget) []BatchReplyResult {
	results := make([]BatchReplyResult, 0, len(targets))

	logrus.Infof("开始批量回复 %d 个评论", len(targets))

	// 检查 context 是否已经超时
	select {
	case <-ctx.Done():
		logrus.Warn("Context 已取消，批量回复终止")
		for _, target := range targets {
			results = append(results, BatchReplyResult{
				CommentID: target.CommentID,
				UserID:    target.UserID,
				Success:   false,
				Error:     "操作被取消或超时",
			})
		}
		return results
	default:
	}

	// 只需要打开一次页面
	page := f.page.Timeout(15 * time.Minute)

	// 检查当前页面 URL 是否正确
	currentURL := page.MustInfo().URL
	targetURL := makeFeedDetailURL(feedID, xsecToken)

	// 只有当 URL 不匹配时才导航
	if currentURL != targetURL {
		logrus.Infof("打开 feed 详情页: %s", targetURL)

		// 使用带超时的 Navigate
		logrus.Info("开始导航...")
		navPage := page.Timeout(60 * time.Second)
		err := navPage.Navigate(targetURL)
		if err != nil {
			logrus.Errorf("导航失败: %v", err)
			for _, target := range targets {
				results = append(results, BatchReplyResult{
					CommentID: target.CommentID,
					UserID:    target.UserID,
					Success:   false,
					Error:     fmt.Sprintf("导航失败: %v", err),
				})
			}
			return results
		}
		logrus.Info("导航完成")

		// 不等待完全稳定，而是等待关键元素出现（更快）
		logrus.Info("等待评论区加载...")

		// 尝试等待评论区或输入框出现（最多 5 秒）
		waitStart := time.Now()
		commentAreaLoaded := false
		for time.Since(waitStart) < 5*time.Second {
			// 检查评论区是否已加载
			_, err := page.Timeout(500 * time.Millisecond).Element(".comment-container, .comments-container, .comment-list, .input-box")
			if err == nil {
				commentAreaLoaded = true
				logrus.Info("✓ 评论区已加载")
				break
			}
			time.Sleep(300 * time.Millisecond)
		}

		if !commentAreaLoaded {
			logrus.Warn("评论区加载超时，继续执行")
		}

		time.Sleep(1 * time.Second)

		// 检测页面是否可访问
		if err := checkPageAccessible(page); err != nil {
			// 如果页面不可访问，所有回复都失败
			for _, target := range targets {
				results = append(results, BatchReplyResult{
					CommentID: target.CommentID,
					UserID:    target.UserID,
					Success:   false,
					Error:     fmt.Sprintf("页面不可访问: %v", err),
				})
			}
			return results
		}
		time.Sleep(2 * time.Second)
	} else {
		logrus.Infof("页面已在正确位置，复用当前页面")
	}

	// 逐个回复
	startTime := time.Now()
	for i, target := range targets {
		// 检查 context 是否已经超时
		select {
		case <-ctx.Done():
			logrus.Warnf("Context 已取消，停止批量回复 (已完成 %d/%d)", i, len(targets))
			// 剩余的标记为失败
			for j := i; j < len(targets); j++ {
				results = append(results, BatchReplyResult{
					CommentID: targets[j].CommentID,
					UserID:    targets[j].UserID,
					Success:   false,
					Error:     "操作被取消或超时",
				})
			}
			return results
		default:
		}

		elapsed := time.Since(startTime)
		logrus.Infof("=== 处理第 %d/%d 个回复 (已耗时 %.1f 秒) ===", i+1, len(targets), elapsed.Seconds())
		logrus.Infof("目标: CommentID=%s, UserID=%s", target.CommentID, target.UserID)

		replyStart := time.Now()
		err := f.replyToCommentOnPage(page, target.CommentID, target.UserID, target.Content)
		replyDuration := time.Since(replyStart)

		result := BatchReplyResult{
			CommentID: target.CommentID,
			UserID:    target.UserID,
			Success:   err == nil,
		}

		if err != nil {
			result.Error = err.Error()
			logrus.Warnf("回复失败 (耗时 %.1f 秒): %v", replyDuration.Seconds(), err)
		} else {
			logrus.Infof("✓ 回复成功 (耗时 %.1f 秒)", replyDuration.Seconds())
		}

		results = append(results, result)

		// 每次回复后等待一下，避免操作过快
		if i < len(targets)-1 {
			time.Sleep(1500 * time.Millisecond) // 从 2 秒减少到 1.5 秒
		}
	}

	totalDuration := time.Since(startTime)
	logrus.Infof("批量回复总耗时: %.1f 秒", totalDuration.Seconds())

	logrus.Infof("批量回复完成: 成功 %d/%d", countSuccessful(results), len(results))
	return results
}

// replyToCommentOnPage 在已打开的页面上回复评论（内部方法）
func (f *CommentFeedAction) replyToCommentOnPage(page *rod.Page, commentID, userID, content string) error {
	// 查找评论元素
	commentEl, err := findCommentElement(page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	// 滚动到评论位置
	logrus.Info("滚动到评论位置...")
	commentEl.MustScrollIntoView()
	time.Sleep(300 * time.Millisecond)

	// 查找并点击回复按钮
	replyBtn, err := commentEl.Element(".right .interactions .reply")
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}

	if err := replyBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击回复按钮失败: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	// 查找回复输入框（带重试）
	var inputEl *rod.Element
	for retry := 0; retry < 3; retry++ {
		inputEl, err = page.Timeout(1 * time.Second).Element("div.input-box div.content-edit p.content-input")
		if err == nil {
			break
		}
		if retry < 2 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}

	// 清空输入框（使用 JS 清空内容）
	inputEl.MustEval(`() => { this.textContent = ''; }`)
	time.Sleep(50 * time.Millisecond)

	// 输入内容
	if err := inputEl.Input(content); err != nil {
		return fmt.Errorf("输入回复内容失败: %w", err)
	}

	time.Sleep(300 * time.Millisecond)

	// 查找并点击提交按钮
	submitBtn, err := page.Element("div.bottom button.submit")
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}

	if err := submitBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击提交按钮失败: %w", err)
	}

	time.Sleep(800 * time.Millisecond)
	return nil
}

// countSuccessful 统计成功的数量
func countSuccessful(results []BatchReplyResult) int {
	count := 0
	for _, r := range results {
		if r.Success {
			count++
		}
	}
	return count
}

// ReplyToComment 回复指定评论（单个回复）
func (f *CommentFeedAction) ReplyToComment(ctx context.Context, feedID, xsecToken, commentID, userID, content string) error {
	// 使用批量回复方法处理单个回复
	targets := []ReplyTarget{
		{
			CommentID: commentID,
			UserID:    userID,
			Content:   content,
		},
	}

	results := f.BatchReplyToComments(ctx, feedID, xsecToken, targets)

	if len(results) > 0 && !results[0].Success {
		return fmt.Errorf(results[0].Error)
	}

	return nil
}

// findCommentElement 查找指定评论元素（优化版：智能缓存 + 快速查找）
func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	// 先滚动到评论区
	scrollToCommentsArea(page)
	time.Sleep(1 * time.Second)

	// === 策略 1: 快速查找（不滚动，直接在当前可见区域查找）===
	if commentID != "" {
		selector := fmt.Sprintf("#comment-%s", commentID)
		el, err := page.Timeout(1 * time.Second).Element(selector)
		if err == nil && el != nil {
			logrus.Infof("✓ 在当前可见区域找到评论: %s", commentID)
			return el, nil
		}
	}

	// === 策略 2: 智能滚动查找（减少尝试次数，增加每次滚动距离）===
	const maxAttempts = 50 // 从 150 减少到 50
	const scrollInterval = 600 * time.Millisecond
	const scrollDistance = 1.2 // 每次滚动 1.2 个屏幕高度

	var lastCommentCount = 0
	stagnantChecks := 0

	logrus.Infof("开始智能滚动查找，最大尝试次数: %d", maxAttempts)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt%10 == 0 {
			logrus.Infof("=== 查找进度 %d/%d ===", attempt+1, maxAttempts)
		}

		// === 1. 快速查找（commentID）===
		if commentID != "" {
			selector := fmt.Sprintf("#comment-%s", commentID)
			el, err := page.Timeout(1 * time.Second).Element(selector)
			if err == nil && el != nil {
				logrus.Infof("✓ 找到评论: %s (尝试 %d 次)", commentID, attempt+1)
				return el, nil
			}
		}

		// === 2. 检查是否到底 ===
		if checkEndContainer(page) {
			logrus.Info("已到达评论底部")
			break
		}

		// === 3. 获取评论数量（停滞检测）===
		currentCount := getCommentCount(page)
		if currentCount != lastCommentCount {
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
		}

		if stagnantChecks >= 5 { // 从 10 减少到 5
			logrus.Info("评论数量停滞，可能已加载完")
			break
		}

		// === 4. 大步滚动（减少滚动次数）===
		_, err := page.Eval(fmt.Sprintf(`() => { window.scrollBy(0, window.innerHeight * %f); return true; }`, scrollDistance))
		if err != nil {
			logrus.Warnf("滚动失败: %v", err)
		}
		time.Sleep(scrollInterval)

		// === 5. 滚动后快速查找（userID）===
		if userID != "" && attempt%3 == 0 { // 每 3 次尝试才查找一次，减少查找频率
			elements, err := page.Timeout(1500 * time.Millisecond).Elements(".comment-item, .comment, .parent-comment")
			if err == nil && len(elements) > 0 {
				// 只检查最后 20 个元素（新加载的）
				startIdx := len(elements) - 20
				if startIdx < 0 {
					startIdx = 0
				}
				for i := startIdx; i < len(elements); i++ {
					el := elements[i]
					userEl, err := el.Timeout(300 * time.Millisecond).Element(fmt.Sprintf(`[data-user-id="%s"]`, userID))
					if err == nil && userEl != nil {
						logrus.Infof("✓ 找到评论 (userID: %s, 尝试 %d 次)", userID, attempt+1)
						return el, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s), 尝试次数: %d", commentID, userID, maxAttempts)
}
