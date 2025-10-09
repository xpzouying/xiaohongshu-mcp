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
	page := f.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("Opening feed detail page: %s", url)

	// 导航到详情页
	if err := page.Navigate(url); err != nil {
		logrus.Warnf("Failed to navigate to feed detail page: %v", err)
		return fmt.Errorf("无法打开帖子详情页，该帖子可能在网页端不可访问: %w", err)
	}

	if err := page.WaitStable(2 * time.Second); err != nil {
		logrus.Warnf("Failed to wait for page stable: %v", err)
		return fmt.Errorf("页面加载超时，该帖子可能在网页端不可访问: %w", err)
	}

	time.Sleep(1 * time.Second)

	// 查找评论输入框
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
	page := f.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("Opening feed detail page for reply: %s", url)
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(3 * time.Second) // 增加等待时间确保页面完全加载
	
	// 等待评论容器加载
	waitForCommentsContainer(page)
	
	// 确保评论区域可见
	ensureCommentsVisible(page)
	
	// 额外等待确保评论内容加载完成
	time.Sleep(2 * time.Second)
	
	// 尝试多次查找评论元素
	var commentEl *rod.Element
	var err error
	for attempt := 0; attempt < 5; attempt++ { // 增加尝试次数
		commentEl, err = findCommentElement(page, commentID, userID)
		if err == nil {
			break
		}
		logrus.Warnf("Attempt %d: Failed to find comment: %v", attempt+1, err)
		time.Sleep(2 * time.Second) // 增加等待时间
		ensureCommentsVisible(page)
		scrollComments(page) // 每次尝试后滚动
	}
	
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}
	
	// 滚动到评论位置
	_, _ = commentEl.Eval(`() => { try { this.scrollIntoView({behavior: "instant", block: "center"}); } catch (e) {} return true }`)
	time.Sleep(1 * time.Second) // 增加等待时间
	
	// 尝试多次点击回复按钮
	var replyBtn *rod.Element
	for attempt := 0; attempt < 5; attempt++ { // 增加尝试次数
		replyBtn, err = findReplyButton(commentEl)
		if err == nil {
			if tryClickChainForComment(replyBtn) {
				break
			}
		}
		logrus.Warnf("Attempt %d: Failed to click reply button: %v", attempt+1, err)
		time.Sleep(1 * time.Second) // 增加等待时间
	}
	
	if err != nil || replyBtn == nil {
		return fmt.Errorf("无法点击回复按钮")
	}
	
	time.Sleep(2 * time.Second) // 增加等待时间确保回复输入框出现
	
	// 查找回复输入框
	inputEl, err := findReplyInput(page, commentEl)
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}
	
	// 聚焦并输入内容
	if _, evalErr := inputEl.Eval(`() => { try { this.focus(); } catch (e) {} return true }`); evalErr != nil {
		logrus.Warnf("focus reply input failed: %v", evalErr)
	}
	
	inputEl.MustInput(content)
	time.Sleep(500 * time.Millisecond) // 增加等待时间
	
	// 查找并点击提交按钮
	submitBtn, err := findSubmitButton(page)
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}
	
	if !tryClickChainForComment(submitBtn) {
		return fmt.Errorf("点击回复提交按钮失败")
	}
	
	time.Sleep(3 * time.Second) // 增加等待时间确保回复提交完成
	return nil
}

func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	var lastErr error
	
	// 首先尝试确保评论区域可见
	ensureCommentsVisible(page)
	
	for attempt := 0; attempt < 20; attempt++ { // 增加尝试次数
		logrus.Infof("查找评论，尝试次数: %d", attempt+1)
		el, err := locateCommentElement(page, commentID, userID)
		if err == nil && el != nil {
			logrus.Infof("成功找到评论")
			return el, nil
		}
		if err != nil {
			lastErr = err
		}
		
		// 每3次尝试后进行一次更彻底的滚动
		if attempt%3 == 0 {
			// 更彻底的滚动策略
			performFullScroll(page)
		} else {
			// 常规滚动
			if !scrollComments(page) {
				logrus.Infof("滚动到底部，无法继续滚动")
				break
			}
		}
		time.Sleep(800 * time.Millisecond) // 增加等待时间
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到评论: %s", buildIdentifier(commentID, userID))
}

func locateCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	// 首先在comments-container内查找
	if commentsContainer, err := page.Element(".comments-container"); err == nil && commentsContainer != nil {
		if commentID != "" {
			if el, err := locateCommentElementByCommentIDInContainer(commentsContainer, commentID); err == nil && el != nil {
				return el, nil
			}
		}
		if userID != "" {
			if el, err := locateCommentElementByUserIDInContainer(commentsContainer, userID); err == nil && el != nil {
				return el, nil
			}
		}
	}
	
	// 如果在comments-container内没有找到，尝试在整个页面查找
	if commentID != "" {
		if el, err := locateCommentElementByCommentID(page, commentID); err == nil && el != nil {
			return el, nil
		}
	}
	if userID != "" {
		if el, err := locateCommentElementByUserID(page, userID); err == nil && el != nil {
			return el, nil
		}
	}
	
	identifier := buildIdentifier(commentID, userID)
	if identifier == "" {
		return nil, fmt.Errorf("未提供评论标识")
	}
	return nil, fmt.Errorf("未找到评论: %s", identifier)
}

func locateCommentElementByCommentID(page *rod.Page, commentID string) (*rod.Element, error) {
	if commentID == "" {
		return nil, fmt.Errorf("评论ID为空")
	}
	
	// 首先尝试直接通过ID查找（根据HTML结构中的id="comment-68d9df3e0000000002015818"）
	idSelector := fmt.Sprintf("#comment-%s", commentID)
	if el, err := page.Element(idSelector); err == nil && el != nil {
		return el, nil
	}
	
	// 尝试其他data属性
	selectors := []string{
		fmt.Sprintf(`[data-comment-id="%s"]`, commentID),
		fmt.Sprintf(`[data-comment_id="%s"]`, commentID),
		fmt.Sprintf(`[data-commentid="%s"]`, commentID),
		fmt.Sprintf(`[data-id="%s"]`, commentID),
		fmt.Sprintf(`[comment-id="%s"]`, commentID),
	}
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			return el, nil
		}
	}
	
	return nil, fmt.Errorf("未找到评论ID: %s", commentID)
}

func locateCommentElementByUserID(page *rod.Page, userID string) (*rod.Element, error) {
	if userID == "" {
		return nil, fmt.Errorf("用户ID为空")
	}
	
	selectors := []string{
		fmt.Sprintf(`[data-user-id="%s"]`, userID),
		fmt.Sprintf(`[data-user_id="%s"]`, userID),
		fmt.Sprintf(`[data-userid="%s"]`, userID),
		fmt.Sprintf(`[data-uid="%s"]`, userID),
		fmt.Sprintf(`a[data-user-id="%s"]`, userID),
		fmt.Sprintf(`a[href*="%s"]`, userID),
	}
	
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			// 使用JavaScript查找父级评论元素
			jsCode := `() => {
				let current = this;
				while (current) {
					if (current.classList && (current.classList.contains('comment-item') || current.classList.contains('comment'))) {
						return current;
					}
					current = current.parentElement;
				}
				return this;
			}`
			if _, err := el.Eval(jsCode); err == nil {
				return el, nil
			}
			return el, nil
		}
	}
	
	return nil, fmt.Errorf("未找到用户ID: %s", userID)
}

// 在指定容器内查找评论元素
func locateCommentElementByCommentIDInContainer(container *rod.Element, commentID string) (*rod.Element, error) {
	if commentID == "" {
		return nil, fmt.Errorf("评论ID为空")
	}
	
	// 首先尝试直接通过ID查找
	idSelector := fmt.Sprintf("#comment-%s", commentID)
	if el, err := container.Element(idSelector); err == nil && el != nil {
		return el, nil
	}
	
	// 尝试其他data属性
	selectors := []string{
		fmt.Sprintf(`[data-comment-id="%s"]`, commentID),
		fmt.Sprintf(`[data-comment_id="%s"]`, commentID),
		fmt.Sprintf(`[data-commentid="%s"]`, commentID),
		fmt.Sprintf(`[data-id="%s"]`, commentID),
		fmt.Sprintf(`[comment-id="%s"]`, commentID),
	}
	for _, selector := range selectors {
		if el, err := container.Element(selector); err == nil && el != nil {
			return el, nil
		}
	}
	
	return nil, fmt.Errorf("在容器内未找到评论ID: %s", commentID)
}

// 在指定容器内通过用户ID查找评论元素
func locateCommentElementByUserIDInContainer(container *rod.Element, userID string) (*rod.Element, error) {
	if userID == "" {
		return nil, fmt.Errorf("用户ID为空")
	}
	
	selectors := []string{
		fmt.Sprintf(`[data-user-id="%s"]`, userID),
		fmt.Sprintf(`[data-user_id="%s"]`, userID),
		fmt.Sprintf(`[data-userid="%s"]`, userID),
		fmt.Sprintf(`[data-uid="%s"]`, userID),
		fmt.Sprintf(`a[data-user-id="%s"]`, userID),
		fmt.Sprintf(`a[href*="%s"]`, userID),
	}
	
	for _, selector := range selectors {
		if el, err := container.Element(selector); err == nil && el != nil {
			// 找到用户链接，返回其父级评论元素
			if parent, err := el.Element(".comment-item"); err == nil && parent != nil {
				return parent, nil
			}
			if parent, err := el.Element(".comment"); err == nil && parent != nil {
				return parent, nil
			}
			return el, nil
		}
	}
	
	return nil, fmt.Errorf("在容器内未找到用户ID: %s", userID)
}

// 等待评论容器加载完成
func waitForCommentsContainer(page *rod.Page) {
	jsCode := `() => {
		// 等待comments-container元素出现
		let attempts = 0;
		const maxAttempts = 10;
		
		const checkContainer = () => {
			const container = document.querySelector('.comments-container');
			if (container) {
				// 检查容器内是否有评论内容
				const comments = container.querySelectorAll('.comment-item, .comment');
				return comments.length > 0;
			}
			return false;
		};
		
		// 定期检查评论容器是否加载完成
		const interval = setInterval(() => {
			attempts++;
			if (checkContainer() || attempts >= maxAttempts) {
				clearInterval(interval);
			}
		}, 500);
		
		return checkContainer();
	}`
	
	page.Eval(jsCode)
	time.Sleep(2 * time.Second) // 等待检查完成
}

func ensureCommentsVisible(page *rod.Page) {
	// 专门针对comments-container元素的JavaScript代码
	jsCode := `() => {
		// 查找comments-container元素
		const commentsContainer = document.querySelector('.comments-container');
		
		// 如果找到comments-container，尝试滚动到视图中并在其内部滚动
		if (commentsContainer) {
			// 先滚动到视图中
			commentsContainer.scrollIntoView({behavior: 'instant', block: 'start'});
			
			// 等待一下再在容器内部滚动
			setTimeout(() => {
				// 在comments-container内部滚动以显示评论
				if (commentsContainer.scrollHeight > commentsContainer.clientHeight) {
					const maxScroll = commentsContainer.scrollHeight - commentsContainer.clientHeight;
					if (maxScroll > 0) {
						// 滚动到一半位置
						commentsContainer.scrollTop = Math.min(maxScroll, commentsContainer.clientHeight * 0.5);
					}
				}
			}, 200);
			
			return true;
		}
		
		return false;
	}`
	
	page.Eval(jsCode)
	time.Sleep(1 * time.Second)
}

func scrollComments(page *rod.Page) bool {
	scrollJS := `() => {
		let scrolled = false;
		
		// 专门查找comments-container元素
		const commentsContainer = document.querySelector('.comments-container');
		
		if (commentsContainer) {
			const maxScroll = commentsContainer.scrollHeight - commentsContainer.clientHeight;
			if (maxScroll > 0 && commentsContainer.scrollTop < maxScroll) {
				// 滚动更多内容
				const delta = Math.max(commentsContainer.clientHeight * 0.8, 400);
				commentsContainer.scrollTop = Math.min(maxScroll, commentsContainer.scrollTop + delta);
				scrolled = true;
			}
		}

		return scrolled;
	}`
	res, err := page.Eval(scrollJS)
	if err != nil {
		logrus.Warnf("scroll comments failed: %v", err)
		return false
	}
	if res == nil {
		return false
	}
	return res.Value.Bool()
}

// performFullScroll 执行更彻底的滚动策略
func performFullScroll(page *rod.Page) {
	logrus.Infof("执行彻底滚动策略")
	
	// 策略1: 滚动到评论容器的不同位置
	scrollPositionsJS := `() => {
		const commentsContainer = document.querySelector('.comments-container');
		if (!commentsContainer) return false;
		
		const maxScroll = commentsContainer.scrollHeight - commentsContainer.clientHeight;
		if (maxScroll <= 0) return false;
		
		// 根据当前滚动位置决定下一步滚动
		const currentScroll = commentsContainer.scrollTop;
		const scrollRatio = currentScroll / maxScroll;
		
		if (scrollRatio < 0.3) {
			// 滚动到30%位置
			commentsContainer.scrollTop = maxScroll * 0.3;
		} else if (scrollRatio < 0.6) {
			// 滚动到60%位置
			commentsContainer.scrollTop = maxScroll * 0.6;
		} else if (scrollRatio < 0.9) {
			// 滚动到90%位置
			commentsContainer.scrollTop = maxScroll * 0.9;
		} else {
			// 滚动到底部
			commentsContainer.scrollTop = maxScroll;
		}
		
		return true;
	}`
	
	if _, err := page.Eval(scrollPositionsJS); err != nil {
		logrus.Warnf("彻底滚动失败: %v", err)
	}
	
}

func buildIdentifier(commentID, userID string) string {
	if commentID != "" && userID != "" {
		return fmt.Sprintf("comment_id=%s / user_id=%s", commentID, userID)
	}
	if commentID != "" {
		return commentID
	}
	return userID
}

func findReplyButton(commentEl *rod.Element) (*rod.Element, error) {
	logrus.Infof("开始查找回复按钮...")
	
	// 在right区域内查找interactions
	right, err := commentEl.Element(".right")
	if err != nil {
		logrus.Errorf("未找到.right区域")
		return nil, fmt.Errorf("未找到.right区域")
	}
	
	interactions, err := right.Element(".interactions")
	if err != nil {
		logrus.Errorf("未找到.interactions区域")
		return nil, fmt.Errorf("未找到.interactions区域")
	}
	
	// 选择器列表
	selectors := []string{
		".reply",                           // 回复容器（最通用）
		":nth-child(2)",                    // 第二个子元素（单评论）
		".reply-icon",                      // 回复图标
		".reds-icon.reply-icon",            // 带类的回复图标
		".reply.icon-container",            // 回复图标容器
	}
	
	// 在interactions区域内查找
	for _, selector := range selectors {
		if el, err := interactions.Element(selector); err == nil && el != nil {
			logrus.Infof("通过选择器 %s 找到回复按钮", selector)
			return el, nil
		}
	}
	
	logrus.Errorf("未找到回复按钮")
	return nil, fmt.Errorf("未找到回复按钮")
}

// verifyClickSuccess 验证点击是否真的成功（检查是否出现了回复输入框）
func verifyClickSuccess(clickedEl *rod.Element) bool {
	// 获取页面实例
	page := clickedEl.Page()
	
	// 检查是否出现了回复输入框
	selectors := []string{
		"div.input-box div.content-edit p.content-input",
		"div.input-box [contenteditable='true']",
		"[contenteditable='true']",
		"textarea",
		"input[type='text']",
	}
	
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			// 检查元素是否可见
			if visible, _ := el.Visible(); visible {
				logrus.Infof("验证成功：找到可见的回复输入框 (%s)", selector)
				return true
			}
		}
	}
	
	// 使用JavaScript检查是否有新的输入框出现
	jsCode := `() => {
		// 查找所有可编辑元素
		const editables = document.querySelectorAll('[contenteditable="true"], textarea, input[type="text"]');
		for (const el of editables) {
			// 检查元素是否可见
			const rect = el.getBoundingClientRect();
			if (rect.width > 0 && rect.height > 0) {
				// 检查元素是否在视口中
				const inViewport = rect.top >= 0 && rect.left >= 0 && 
					rect.bottom <= window.innerHeight && 
					rect.right <= window.innerWidth;
				if (inViewport) {
					console.log('找到可见的输入元素:', el);
					return true;
				}
			}
		}
		return false;
	}`
	
	if result, err := page.Eval(jsCode); err == nil && result != nil {
		if result.Value.Bool() {
			logrus.Infof("JavaScript验证成功：找到可见的输入元素")
			return true
		}
	}
	
	logrus.Infof("验证失败：没有找到回复输入框")
	return false
}

func findReplyInput(page *rod.Page, commentEl *rod.Element) (*rod.Element, error) {
	activeEditableJS := `() => {
        const active = document.activeElement;
        if (active && active.getAttribute && active.getAttribute('contenteditable') === 'true') {
            return active;
        }
        return null;
    }`
	if el, err := page.ElementByJS(rod.Eval(activeEditableJS)); err == nil && el != nil {
		return el, nil
	}
	selectors := []string{
		"div.input-box div.content-edit p.content-input",  // 原有选择器
		"div.input-box [contenteditable='true']",         // 通用输入框
		"[contenteditable='true']",                        // 任何可编辑元素
		"textarea",                                        // 备用textarea
		"input[type='text']",                             // 备用text输入框
		"[data-role='reply-input'] [contenteditable='true']",
	}
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			return el, nil
		}
	}
	// 尝试在评论内部寻找可编辑区域
	if el, err := commentEl.Element("[contenteditable='true']"); err == nil && el != nil {
		return el, nil
	}
	// 最后尝试：等待一下再查找，可能是动态加载的
	time.Sleep(1 * time.Second)
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			return el, nil
		}
	}
	return nil, fmt.Errorf("未找到回复输入框")
}

func tryClickChainForComment(el *rod.Element) bool {
	if el == nil {
		logrus.Errorf("要点击的元素为空")
		return false
	}
	
	// 获取元素信息用于调试
	text, _ := el.Text()
	class, _ := el.Attribute("class")
	tag, _ := el.Describe(0, false)
	logrus.Infof("准备点击元素 - 文本: '%s', 类: '%s', 标签: %s", text, class, tag)
	
	// 检查元素是否可见和可点击
	visible, _ := el.Visible()
	logrus.Infof("元素可见性: %v", visible)
	
	// 滚动到元素位置
	_, _ = el.Eval(`() => { try { this.scrollIntoView({behavior: "instant", block: "center"}); } catch (e) {} return true }`)
	time.Sleep(500 * time.Millisecond)
	
	// 只使用直接点击方式
	clickMethods := []struct {
		name string
		fn   func(*rod.Element) bool
	}{
		{"直接点击", func(e *rod.Element) bool {
			if err := e.Click(proto.InputMouseButtonLeft, 1); err != nil {
				logrus.Warnf("直接点击失败: %v", err)
				return false
			}
			logrus.Infof("直接点击成功")
			return true
		}},
	}
	
	for i, method := range clickMethods {
		logrus.Infof("尝试点击方法 %d: %s", i+1, method.name)
		if method.fn(el) {
			// 点击后等待一下，检查是否有反应
			time.Sleep(1 * time.Second)
			
			// 验证点击是否真的成功（检查是否出现了回复输入框）
			success := verifyClickSuccess(el)
			if success {
				logrus.Infof("点击方法 %s 执行成功且有效", method.name)
				return true
			} else {
				logrus.Warnf("点击方法 %s 执行成功但无效（没有出现回复输入框）", method.name)
				// 继续尝试下一种方法
			}
		}
	}
	
	logrus.Errorf("所有点击方法都失败")
	return false
}

func findSubmitButton(page *rod.Page) (*rod.Element, error) {
	selectors := []string{
		"div.bottom button.submit",
		"button.submit",
		"button.reds-button",
		"button[type='submit']",
		"button:contains('回复')",
		"button:contains('发布')",
		"button:contains('发送')",
	}
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			disabled, _ := el.Attribute("disabled")
			if disabled == nil {
				return el, nil
			}
		}
	}
	// 使用JS查找包含特定文本的按钮
	jsCode := `() => {
		const buttons = document.querySelectorAll('button');
		for (const btn of buttons) {
			const text = btn.textContent || btn.innerText || '';
			if (text.includes('回复') || text.includes('发布') || text.includes('发送')) {
				const disabled = btn.getAttribute('disabled');
				if (!disabled) {
					return btn;
				}
			}
		}
		return null;
	}`
	if el, err := page.ElementByJS(rod.Eval(jsCode)); err == nil && el != nil {
		return el, nil
	}
	return nil, fmt.Errorf("未找到回复发布按钮")
}