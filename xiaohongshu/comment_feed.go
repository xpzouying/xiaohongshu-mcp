package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
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
	page := f.page.Timeout(120 * time.Second)

	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)

	// 拟人导航：先访问首页再跳转到目标页
	if err := NavigateWithLanding(page, url, false); err != nil {
		return fmt.Errorf("导航到详情页失败: %w", err)
	}

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 先滚动到评论区位置，模拟阅读行为
	scrollToCommentsArea(page)
	RandomDelay(1000, 2500)

	// 查找并点击评论输入框
	elem, err := page.Element("div.input-box div.content-edit span")
	if err != nil {
		logrus.Warnf("找不到评论输入框: %v", err)
		return fmt.Errorf("未找到评论输入框，该帖子可能不支持评论或网页端不可访问: %w", err)
	}

	// 拟人点击：悬停 → 延迟 → 点击
	if err := ClickWithHumanBehavior(page, elem); err != nil {
		return fmt.Errorf("无法点击评论输入框: %w", err)
	}
	RandomDelay(300, 800)

	elem2, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		logrus.Warnf("找不到评论输入区域: %v", err)
		return fmt.Errorf("未找到评论输入区域: %w", err)
	}

	// 逐字符输入评论内容（模拟真人打字）
	if err := TypeWithHumanBehavior(elem2, content); err != nil {
		return fmt.Errorf("输入评论内容失败: %w", err)
	}

	// 输入完成后停顿一下，模拟思考
	RandomDelay(800, 2000)

	submitButton, err := page.Element("div.bottom button.submit")
	if err != nil {
		logrus.Warnf("找不到提交按钮: %v", err)
		return fmt.Errorf("未找到提交按钮: %w", err)
	}

	// 拟人点击提交按钮
	if err := ClickWithHumanBehavior(page, submitButton); err != nil {
		return fmt.Errorf("无法点击提交按钮: %w", err)
	}

	// 等待提交完成
	RandomDelay(1500, 3000)

	logrus.Infof("评论发表成功: feed=%s", feedID)
	return nil
}

// ReplyToComment 回复指定评论
func (f *CommentFeedAction) ReplyToComment(ctx context.Context, feedID, xsecToken, commentID, userID, content string) error {
	page := f.page.Timeout(5 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行回复: %s", url)

	// 拟人导航：先访问首页再跳转到目标页
	if err := NavigateWithLanding(page, url, false); err != nil {
		return fmt.Errorf("导航到详情页失败: %w", err)
	}

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 等待评论区加载
	RandomDelay(2000, 3500)

	// 查找目标评论
	commentEl, err := findCommentElement(page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	// 滚动到评论位置
	logrus.Info("滚动到评论位置...")
	commentEl.MustScrollIntoView()
	RandomDelay(800, 1500)

	logrus.Info("准备点击回复按钮")

	// 查找并点击回复按钮
	replyBtn, err := commentEl.Element(".right .interactions .reply")
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}

	if err := ClickWithHumanBehavior(page, replyBtn); err != nil {
		return fmt.Errorf("点击回复按钮失败: %w", err)
	}
	RandomDelay(500, 1200)

	// 查找回复输入框
	inputEl, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}

	// 逐字符输入回复内容
	if err := TypeWithHumanBehavior(inputEl, content); err != nil {
		return fmt.Errorf("输入回复内容失败: %w", err)
	}

	RandomDelay(800, 1500)

	// 查找并点击提交按钮
	submitBtn, err := page.Element("div.bottom button.submit")
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}

	if err := ClickWithHumanBehavior(page, submitBtn); err != nil {
		return fmt.Errorf("点击提交按钮失败: %w", err)
	}

	RandomDelay(1500, 3000)
	logrus.Infof("回复评论成功: feed=%s, comment=%s", feedID, commentID)

	return nil
}

// findCommentElement 查找指定评论元素（参考 feed_detail.go 的滚动逻辑）
func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	const maxAttempts = 100
	const scrollInterval = 800 * time.Millisecond

	// 先滚动到评论区
	scrollToCommentsArea(page)
	RandomDelay(800, 1500)

	var lastCommentCount = 0
	stagnantChecks := 0

	logrus.Infof("开始循环查找，最大尝试次数: %d", maxAttempts)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		logrus.Infof("=== 查找尝试 %d/%d ===", attempt+1, maxAttempts)

		// 检查是否到达底部
		if checkEndContainer(page) {
			logrus.Info("已到达评论底部，未找到目标评论")
			break
		}

		// 获取当前评论数量
		currentCount := getCommentCount(page)
		logrus.Infof("当前评论数: %d", currentCount)

		if currentCount != lastCommentCount {
			logrus.Infof("评论数增加: %d -> %d", lastCommentCount, currentCount)
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
			if stagnantChecks%5 == 0 {
				logrus.Infof("评论数停滞 %d 次", stagnantChecks)
			}
		}

		// 停滞检测
		if stagnantChecks >= 10 {
			logrus.Info("评论数量停滞超过10次，可能已加载完所有评论")
			break
		}

		// 先滚动到最后一个评论（触发懒加载）
		if currentCount > 0 {
			logrus.Infof("滚动到最后一个评论（共 %d 条）", currentCount)

			elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment, .comment-item, .comment")
			if err == nil && len(elements) > 0 {
				lastComment := elements[len(elements)-1]
				err := lastComment.ScrollIntoView()
				if err != nil {
					logrus.Warnf("滚动到最后一个评论失败: %v", err)
				}
			} else {
				logrus.Warnf("未找到评论元素: %v", err)
			}
			RandomDelay(200, 500)
		}

		// 继续向下滚动
		logrus.Infof("继续向下滚动...")
		_, err := page.Eval(`() => { window.scrollBy(0, window.innerHeight * 0.8); return true; }`)
		if err != nil {
			logrus.Warnf("滚动失败: %v", err)
		}
		RandomDelay(300, 700)

		// 滚动后立即查找（边滚动边查找）
		if commentID != "" {
			selector := fmt.Sprintf("#comment-%s", commentID)
			logrus.Infof("尝试通过 commentID 查找: %s", selector)

			el, err := page.Timeout(2 * time.Second).Element(selector)
			if err == nil && el != nil {
				logrus.Infof("通过 commentID 找到评论: %s (尝试 %d 次)", commentID, attempt+1)
				return el, nil
			}
			logrus.Infof("未找到 commentID (2秒超时)")
		}

		// 通过 userID 查找
		if userID != "" {
			logrus.Infof("尝试通过 userID 查找: %s", userID)

			elements, err := page.Timeout(2 * time.Second).Elements(".comment-item, .comment, .parent-comment")
			if err == nil && len(elements) > 0 {
				logrus.Infof("找到 %d 个评论元素", len(elements))
				for i, el := range elements {
					userEl, err := el.Timeout(500 * time.Millisecond).Element(fmt.Sprintf(`[data-user-id="%s"]`, userID))
					if err == nil && userEl != nil {
						logrus.Infof("通过 userID 在第 %d 个元素中找到评论: %s (尝试 %d 次)", i+1, userID, attempt+1)
						return el, nil
					}
				}
				logrus.Infof("在 %d 个元素中未找到匹配的 userID", len(elements))
			} else {
				logrus.Infof("获取评论元素失败或超时: %v", err)
			}
		}

		logrus.Infof("本次尝试未找到目标评论，继续下一轮...")

		// 等待内容加载
		time.Sleep(scrollInterval)
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s), 尝试次数: %d", commentID, userID, maxAttempts)
}
