package xiaohongshu

import (
	"context"
	"fmt"
	"strings"
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
	page := f.page.Timeout(3 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行回复: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1200 * time.Millisecond)

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 等待评论容器加载
	time.Sleep(1800 * time.Millisecond)

	// 使用 Go 实现的查找逻辑
	commentEl, err := findCommentElement(page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	// 滚动到评论位置
	logrus.Info("滚动到评论位置...")
	commentEl.MustScrollIntoView()
	time.Sleep(500 * time.Millisecond)

	logrus.Info("准备点击回复按钮")

	// 查找并点击回复按钮（增强兼容）
	replyBtn, err := findReplyButton(commentEl)
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}

	if err := replyBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击回复按钮失败: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	// 查找回复输入框
	inputEl, err := page.Element("div.input-box div.content-edit p.content-input")
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}

	// 输入内容
	if err := inputEl.Input(content); err != nil {
		return fmt.Errorf("输入回复内容失败: %w", err)
	}

	time.Sleep(350 * time.Millisecond)

	// 查找并点击提交按钮
	submitBtn, err := page.Element("div.bottom button.submit")
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}

	if err := submitBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击提交按钮失败: %w", err)
	}

	time.Sleep(1200 * time.Millisecond)
	logrus.Infof("回复评论成功")
	return nil
}

// findCommentElement 查找指定评论元素（兼容更多 DOM）
func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	const maxAttempts = 35
	const scrollInterval = 650 * time.Millisecond

	// 先滚动到评论区
	scrollToCommentsArea(page)
	time.Sleep(1 * time.Second)

	var lastCommentCount = 0
	stagnantChecks := 0
	zeroCountChecks := 0

	logrus.Infof("开始循环查找，最大尝试次数: %d", maxAttempts)

	endSeenCount := 0
	for attempt := 0; attempt < maxAttempts; attempt++ {
		logrus.Infof("=== 查找尝试 %d/%d ===", attempt+1, maxAttempts)

		// 优先精准查找，避免被“THE END”过早短路
		if commentID != "" {
			if el := findByCommentID(page, commentID); el != nil {
				logrus.Infof("✓ 通过 commentID 找到评论: %s", commentID)
				return el, nil
			}
		}
		if userID != "" {
			if el := findByUserID(page, userID); el != nil {
				logrus.Infof("✓ 通过 userID 找到评论: %s", userID)
				return el, nil
			}
		}

		// === 1. 检查是否到达底部（仅作为信号，不立即退出）===
		if checkEndContainer(page) {
			endSeenCount++
			logrus.Infof("检测到评论底部标记（THE END）次数: %d", endSeenCount)
		}

		if checkNoCommentsArea(page) {
			return nil, fmt.Errorf("评论区显示为空（这是一片荒地），无法回复指定评论")
		}

		// === 2. 获取当前评论数量（更鲁棒）===
		currentCount := getCommentCountRobust(page)
		logrus.Infof("当前评论数(robust): %d", currentCount)

		if currentCount == 0 {
			zeroCountChecks++
		}

		if currentCount != lastCommentCount {
			logrus.Infof("✓ 评论数变化: %d -> %d", lastCommentCount, currentCount)
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
			if stagnantChecks%5 == 0 {
				logrus.Infof("评论数停滞 %d 次", stagnantChecks)
			}
		}

		// === 3. 快速失败：一直是0，说明会话/DOM异常 ===
		if zeroCountChecks >= 8 {
			return nil, fmt.Errorf("评论元素持续为0，疑似web登录态失效或DOM结构变更")
		}

		if stagnantChecks >= 12 {
			if endSeenCount >= 3 {
				logrus.Info("评论数量停滞且多次到底，停止继续滚动")
				break
			}
			logrus.Info("评论数量停滞但未稳定到底，继续尝试")
			stagnantChecks = 8
		}

		// === 4. 向下滚动并重试 ===
		_, err := page.Eval(`() => { window.scrollBy(0, Math.max(420, window.innerHeight * 0.8)); return true; }`)
		if err != nil {
			logrus.Warnf("滚动失败: %v", err)
		}

		time.Sleep(scrollInterval)
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s), 尝试次数: %d", commentID, userID, maxAttempts)
}

func findByCommentID(page *rod.Page, commentID string) *rod.Element {
	selectors := []string{
		fmt.Sprintf("#comment-%s", commentID),
		fmt.Sprintf(`[id="comment-%s"]`, commentID),
		fmt.Sprintf(`[id*="%s"]`, commentID),
		fmt.Sprintf(`[data-comment-id="%s"]`, commentID),
		fmt.Sprintf(`[data-commentid="%s"]`, commentID),
		fmt.Sprintf(`[data-rid="%s"]`, commentID),
	}

	for _, sel := range selectors {
		el, err := page.Timeout(1200 * time.Millisecond).Element(sel)
		if err == nil && el != nil {
			return el
		}
	}

	// 兜底：遍历常见评论节点，检查 id/data-* 是否包含 commentID
	elements, err := page.Timeout(1500 * time.Millisecond).Elements(".parent-comment, .comment-item, .comment, [class*='comment']")
	if err != nil || len(elements) == 0 {
		return nil
	}
	for _, el := range elements {
		idAttr, _ := el.Attribute("id")
		if idAttr != nil && strings.Contains(*idAttr, commentID) {
			return el
		}
		for _, k := range []string{"data-comment-id", "data-commentid", "data-id"} {
			v, _ := el.Attribute(k)
			if v != nil && strings.Contains(*v, commentID) {
				return el
			}
		}
	}
	return nil
}

func findByUserID(page *rod.Page, userID string) *rod.Element {
	// 优先 XPath：匹配个人主页链接再回溯到评论容器
	xpath := fmt.Sprintf("//a[contains(@href,'/user/profile/%s')]/ancestor::*[contains(@class,'comment')][1]", userID)
	if el, err := page.Timeout(1200 * time.Millisecond).ElementX(xpath); err == nil && el != nil {
		return el
	}

	elements, err := page.Timeout(1800 * time.Millisecond).Elements(".parent-comment, .comment-item, .comment, [class*='comment']")
	if err != nil || len(elements) == 0 {
		return nil
	}

	for _, el := range elements {
		for _, sel := range []string{
			fmt.Sprintf(`[data-user-id="%s"]`, userID),
			fmt.Sprintf(`[data-userid="%s"]`, userID),
			fmt.Sprintf(`a[href*="/user/profile/%s"]`, userID),
		} {
			u, err := el.Timeout(350 * time.Millisecond).Element(sel)
			if err == nil && u != nil {
				return el
			}
		}
	}
	return nil
}

func getCommentCountRobust(page *rod.Page) int {
	selectors := []string{
		".parent-comment",
		".comment-item",
		".comments-container .comment",
	}
	best := 0
	for _, sel := range selectors {
		elems, err := page.Timeout(1200 * time.Millisecond).Elements(sel)
		if err == nil {
			if n := len(elems); n > best {
				best = n
			}
		}
	}
	return best
}

func findReplyButton(commentEl *rod.Element) (*rod.Element, error) {
	// 常见结构优先
	for _, sel := range []string{
		".right .interactions .reply",
		".interactions .reply",
		"[class*='reply']",
	} {
		el, err := commentEl.Timeout(1200 * time.Millisecond).Element(sel)
		if err == nil && el != nil {
			return el, nil
		}
	}

	// 文案兜底（找包含“回复”的可点击元素）
	elements, err := commentEl.Timeout(1500 * time.Millisecond).Elements("button, span, div")
	if err != nil {
		return nil, err
	}
	for _, el := range elements {
		t, err := el.Timeout(200 * time.Millisecond).Text()
		if err == nil && strings.Contains(strings.TrimSpace(t), "回复") {
			return el, nil
		}
	}
	return nil, fmt.Errorf("未找到包含“回复”文案的按钮")
}
