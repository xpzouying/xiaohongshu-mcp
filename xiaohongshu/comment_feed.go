package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
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
	humanize.Delay(ctx, humanize.AfterNavigate)

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	elem, err := page.Element("div.input-box div.content-edit span")
	if err != nil {
		logrus.Warnf("Failed to find comment input box: %v", err)
		return fmt.Errorf("未找到评论输入框，该帖子可能不支持评论或网页端不可访问: %w", err)
	}

	if err := humanize.Click(elem); err != nil {
		logrus.Warnf("Failed to click comment input box: %v", err)
		return fmt.Errorf("无法点击评论输入框: %w", err)
	}
	humanize.Delay(ctx, humanize.AfterClick)

	elem2, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		logrus.Warnf("Failed to find comment input field: %v", err)
		return fmt.Errorf("未找到评论输入区域: %w", err)
	}

	if err := humanize.Type(ctx, elem2, content); err != nil {
		logrus.Warnf("Failed to input comment content: %v", err)
		return fmt.Errorf("无法输入评论内容: %w", err)
	}

	humanize.Delay(ctx, humanize.AfterType)

	submitButton, err := page.Element("div.bottom button.submit")
	if err != nil {
		logrus.Warnf("Failed to find submit button: %v", err)
		return fmt.Errorf("未找到提交按钮: %w", err)
	}

	if err := humanize.Click(submitButton); err != nil {
		logrus.Warnf("Failed to click submit button: %v", err)
		return fmt.Errorf("无法点击提交按钮: %w", err)
	}

	humanize.Delay(ctx, humanize.AfterClick)

	// 就地校验：提交后评论应在评论区渲染出现；未出现则判定失败，避免假成功。
	if !waitCommentRendered(page, content, 4*time.Second) {
		logrus.Warnf("评论提交后未在评论区渲染，判定未成功: feed=%s", feedID)
		return fmt.Errorf("评论未确认成功：提交后未在评论区出现（可能账号被限制或发送失败），feed: %s", feedID)
	}

	logrus.Infof("Comment posted and verified to feed: %s", feedID)
	return nil
}

// commentRendered 就地读当前页评论区 DOM，判断指定文本的评论是否已渲染出现。
// 不重新导航、不滚动——只读已加载的 .comments-container 的可见文本。
func commentRendered(page *rod.Page, content string) bool {
	res, err := page.Eval(`(txt) => {
		const c = document.querySelector('.comments-container');
		return c ? c.innerText.includes(txt) : false;
	}`, content)
	if err != nil {
		return false
	}
	return res.Value.Bool()
}

func waitCommentRendered(page *rod.Page, content string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if commentRendered(page, content) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
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
	humanize.Delay(ctx, humanize.AfterNavigate)

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	// 使用 Go 实现的查找逻辑
	commentEl, err := findCommentElement(ctx, page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	// 滚动到评论位置
	logrus.Info("滚动到评论位置...")
	commentEl.MustScrollIntoView()
	humanize.Delay(ctx, humanize.BetweenScroll)

	logrus.Info("准备点击回复按钮")

	// 查找并点击回复按钮
	replyBtn, err := commentEl.Element(".right .interactions .reply")
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}

	if err := humanize.Click(replyBtn); err != nil {
		return fmt.Errorf("点击回复按钮失败: %w", err)
	}

	humanize.Delay(ctx, humanize.AfterClick)

	// 查找回复输入框
	inputEl, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}

	// 输入内容
	if err := humanize.Type(ctx, inputEl, content); err != nil {
		return fmt.Errorf("输入回复内容失败: %w", err)
	}

	humanize.Delay(ctx, humanize.AfterType)

	// 查找并点击提交按钮
	submitBtn, err := page.Element("div.bottom button.submit")
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}

	if err := humanize.Click(submitBtn); err != nil {
		return fmt.Errorf("点击提交按钮失败: %w", err)
	}

	humanize.Delay(ctx, humanize.AfterClick)

	// 就地校验：回复应在评论区渲染出现，否则判定未成功。
	if !waitCommentRendered(page, content, 4*time.Second) {
		logrus.Warnf("回复提交后未在评论区渲染，判定未成功: feed=%s", feedID)
		return fmt.Errorf("回复未确认成功：提交后未在评论区出现（可能账号被限制或发送失败）")
	}

	logrus.Infof("回复评论成功并已确认")
	return nil
}

// findCommentElement 滚动查找指定评论：优先按 commentID 命中，否则按 userID 匹配。
// 每轮先在已渲染的评论里查一次，再判断是否到底，最后滚动加载更多。
func findCommentElement(ctx context.Context, page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	const maxAttempts = 100

	scrollToCommentsArea(page)
	humanize.Delay(ctx, humanize.BetweenScroll)

	lastCommentCount := 0
	stagnantChecks := 0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 1. 先查已渲染的评论——目标可能一开始就在页面上
		if el := lookupComment(page, commentID, userID); el != nil {
			logrus.Infof("✓ 找到目标评论（尝试 %d 次）", attempt+1)
			return el, nil
		}

		// 2. 到底则不再加载
		if checkEndContainer(page) {
			logrus.Info("已到达评论底部，未找到目标评论")
			break
		}

		// 3. 评论数停滞说明加载不动了
		currentCount := getCommentCount(page)
		if currentCount != lastCommentCount {
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
			if stagnantChecks >= 10 {
				logrus.Info("评论数量停滞，停止查找")
				break
			}
		}

		// 4. 滚到最后一条评论再继续下滚，触发懒加载
		if currentCount > 0 {
			if elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment, .comment-item, .comment"); err == nil && len(elements) > 0 {
				if err := elements[len(elements)-1].ScrollIntoView(); err != nil {
					logrus.Debugf("滚动到最后一条评论失败: %v", err)
				}
			}
			humanize.Delay(ctx, humanize.BetweenScroll)
		}

		humanScroll(ctx, page, "normal", false, 1)
		humanize.Delay(ctx, humanize.BetweenScroll)
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s)", commentID, userID)
}

// lookupComment 在当前已渲染的评论里查找目标，找不到返回 nil。
func lookupComment(page *rod.Page, commentID, userID string) *rod.Element {
	if commentID != "" {
		if el, err := page.Timeout(2 * time.Second).Element(fmt.Sprintf("#comment-%s", commentID)); err == nil && el != nil {
			return el
		}
	}

	if userID == "" {
		return nil
	}

	elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment, .comment-item, .comment")
	if err != nil {
		return nil
	}
	for _, el := range elements {
		if userEl, err := el.Timeout(500 * time.Millisecond).Element(fmt.Sprintf(`[data-user-id="%s"]`, userID)); err == nil && userEl != nil {
			return el
		}
	}
	return nil
}
