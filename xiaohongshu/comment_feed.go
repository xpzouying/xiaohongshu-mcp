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
	page := f.page.Timeout(60 * time.Second)

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

// ReplyToComment 回复指定评论
func (f *CommentFeedAction) ReplyToComment(ctx context.Context, feedID, xsecToken, commentID, userID, content string) error {
	// 增加超时时间，因为需要滚动查找评论
	// 注意：不使用 Context(ctx)，避免继承外部 context 的超时
	page := f.page.Timeout(5 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行回复: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 等待评论容器加载（增加到5秒，应对慢速网络）
	time.Sleep(5 * time.Second)

	// 使用 Go 实现的查找逻辑
	commentEl, err := findCommentElement(page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	// 滚动到评论位置
	logrus.Info("滚动到评论位置...")
	commentEl.MustScrollIntoView()
	time.Sleep(1 * time.Second)

	logrus.Info("准备点击回复按钮")

	// 查找并点击回复按钮
	replyBtn, err := commentEl.Element(".right .interactions .reply")
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}

	if err := replyBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击回复按钮失败: %w", err)
	}

	time.Sleep(1 * time.Second)

	// 查找回复输入框
	inputEl, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}

	// 输入内容
	if err := inputEl.Input(content); err != nil {
		return fmt.Errorf("输入回复内容失败: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 查找并点击提交按钮
	submitBtn, err := page.Element("div.bottom button.submit")
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}

	if err := submitBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击提交按钮失败: %w", err)
	}

	time.Sleep(2 * time.Second)
	logrus.Infof("回复评论成功")
	return nil
}

// findCommentElement 查找指定评论元素
// 修复：评论区未加载完时不应检查是否到达底部，防止过早退出循环
func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	const (
		maxScrollAttempts   = 100  // 最大滚动尝试次数
		scrollInterval      = 800 * time.Millisecond
		commentWaitTimeout  = 30   // 预等待评论区加载的最大秒数
		stagnantThreshold   = 10   // 连续无增量次数阈值
	)

	// 先滚动到评论区
	scrollToCommentsArea(page)
	time.Sleep(1 * time.Second)

	var (
		lastCommentCount  = 0
		stagnantChecks    = 0
		hasScrolled       = false // 标记是否已经开始滚动
		currentCount      int
	)

	// === 阶段1: 等待评论区加载完成 ===
	// 修复问题：旧代码在评论区未加载时就检查底部容器，导致误判退出
	logrus.Info("等待评论区加载...")
	for i := 0; i < commentWaitTimeout; i++ {
		currentCount = getCommentCount(page)
		if currentCount > 0 {
			logrus.Infof("✓ 评论区已加载，当前有 %d 条评论", currentCount)
			break
		}
		// P3修复：检测到无评论区域时提前退出，不等待30秒
		if checkEndContainer(page) {
			logrus.Info("预检测到评论底部区域（无评论），跳过等待")
			break
		}
		if i == commentWaitTimeout-1 {
			logrus.Warnf("等待 %d 秒后评论区仍为空，将继续尝试查找", commentWaitTimeout)
		}
		time.Sleep(1 * time.Second)
	}

	// === 阶段2: 滚动查找评论 ===
	for attempt := 0; attempt < maxScrollAttempts; attempt++ {
		currentCount = getCommentCount(page)

		// P1修复：底部检测必须基于评论增长信号，而不只是滚动历史
		// 原因：hasScrolled 在第一次 scrollBy 后立即为 true，此时懒加载评论可能还未返回
		//       如果底部容器是预渲染的，循环会在评论实际出现前就 break
		if hasScrolled && currentCount > 0 && currentCount > lastCommentCount {
			if checkEndContainer(page) {
				logrus.Info("已到达评论底部，未找到目标评论")
				break
			}
		}

		// 评论数增量检测
		if currentCount != lastCommentCount {
			logrus.Infof("评论数变化: %d -> %d", lastCommentCount, currentCount)
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
			if stagnantChecks%5 == 0 {
				logrus.Infof("评论数停滞检测: %d 次", stagnantChecks)
			}
		}

		// 停滞超过阈值，退出循环
		if stagnantChecks >= stagnantThreshold {
			logrus.Warnf("评论数停滞 %d 次，可能已加载完所有评论，停止查找", stagnantThreshold)
			break
		}

		// 滚动到最后一个可见评论，触发懒加载
		if currentCount > 0 {
			if elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment, .comment-item, .comment"); err == nil && len(elements) > 0 {
				if err := elements[len(elements)-1].ScrollIntoView(); err != nil {
					logrus.Warnf("滚动到最后一个评论失败: %v", err)
				}
			}
			time.Sleep(300 * time.Millisecond)
			hasScrolled = true
		}

		// 继续向下滚动页面
		if _, err := page.Eval(`() => { window.scrollBy(0, window.innerHeight * 0.8); return true; }`); err != nil {
			logrus.Warnf("页面滚动失败: %v", err)
		}
		hasScrolled = true
		time.Sleep(500 * time.Millisecond)

		// 查找评论元素：优先通过 commentID 精确查找
		if commentID != "" {
			selector := fmt.Sprintf("#comment-%s", commentID)
			if el, err := page.Timeout(2 * time.Second).Element(selector); err == nil && el != nil {
				logrus.Infof("✓ 通过 commentID 找到评论: %s", commentID)
				return el, nil
			}
		}

		// 通过 userID 查找（需匹配评论元素）
		if userID != "" {
			if elements, err := page.Timeout(2 * time.Second).Elements(".comment-item, .comment, .parent-comment"); err == nil && len(elements) > 0 {
				for i, el := range elements {
					if userEl, err := el.Timeout(500 * time.Millisecond).Element(fmt.Sprintf(`[data-user-id="%s"]`, userID)); err == nil && userEl != nil {
						logrus.Infof("✓ 通过 userID 在第 %d 个元素中找到评论: %s", i+1, userID)
						return el, nil
					}
				}
			}
		}

		time.Sleep(scrollInterval)
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s)", commentID, userID)
}
