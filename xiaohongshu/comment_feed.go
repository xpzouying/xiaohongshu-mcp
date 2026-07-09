package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	retry "github.com/avast/retry-go/v4"
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
	page := f.page.Timeout(60 * time.Second)

	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 获取评论前的评论数（用于验证）
	beforeCount := getCommentCountByJS(page)

	// 点击输入框激活
	err := retry.Do(func() error {
		elem, err := page.Timeout(5 * time.Second).Element(".content-edit .inner-when-not-active, div.input-box div.content-edit span")
		if err != nil {
			return fmt.Errorf("未找到评论输入框: %w", err)
		}
		return elem.Click(proto.InputMouseButtonLeft, 1)
	}, retry.Attempts(3), retry.Delay(500*time.Millisecond))
	if err != nil {
		return fmt.Errorf("评论输入框激活失败: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 输入评论内容
	inputEl, err := page.Timeout(5 * time.Second).Element("#content-textarea, div.input-box div.content-edit p.content-input")
	if err != nil {
		return fmt.Errorf("未找到评论输入区域: %w", err)
	}
	if err := inputEl.Input(content); err != nil {
		return fmt.Errorf("无法输入评论内容: %w", err)
	}

	time.Sleep(1 * time.Second)

	// 点击提交按钮
	submitBtn, err := page.Timeout(5 * time.Second).Element("div.bottom button.submit, button.submit")
	if err != nil {
		return fmt.Errorf("未找到提交按钮: %w", err)
	}
	if err := submitBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("无法点击提交按钮: %w", err)
	}

	time.Sleep(2 * time.Second)

	// 验证评论是否成功发送
	afterCount := getCommentCountByJS(page)
	if afterCount > beforeCount {
		logrus.Infof("评论发送成功 (评论数 %d -> %d), feed: %s", beforeCount, afterCount, feedID)
	} else {
		logrus.Warnf("评论可能未成功发送 (评论数未变化: %d), feed: %s", beforeCount, feedID)
	}

	return nil
}

// getCommentCountByJS 通过 JS 从 __INITIAL_STATE__ 获取评论数
func getCommentCountByJS(page *rod.Page) int {
	result, err := page.Timeout(3 * time.Second).Eval(`() => {
		const el = document.querySelector('.comments-container .total');
		if (!el) return 0;
		const m = el.textContent.match(/(\d+)/);
		return m ? parseInt(m[1]) : 0;
	}`)
	if err != nil {
		return 0
	}
	return result.Value.Int()
}

// ReplyToComment 回复指定评论
func (f *CommentFeedAction) ReplyToComment(ctx context.Context, feedID, xsecToken, commentID, userID, content string) error {
	page := f.page.Timeout(5 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行回复: %s", url)

	// 先访问首页建立正常浏览会话（避免无 referrer 直接访问被拦截）
	logrus.Info("先导航到首页建立浏览会话...")
	page.MustNavigate("https://www.xiaohongshu.com")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 再导航到详情页（此时有正常的浏览上下文）
	logrus.Infof("导航到详情页: %s", url)
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	if err := checkPageAccessible(page); err != nil {
		// 如果仍然不可访问，尝试通过 JS 跳转（模拟页面内导航）
		logrus.Warnf("直接导航失败: %v, 尝试 JS 跳转...", err)
		_, _ = page.Eval(fmt.Sprintf(`() => { window.location.href = "%s"; }`, url))
		time.Sleep(3 * time.Second)
		page.MustWaitDOMStable()
		if err2 := checkPageAccessible(page); err2 != nil {
			return fmt.Errorf("页面不可访问（已尝试多种导航方式）: %w", err2)
		}
	}

	time.Sleep(2 * time.Second)

	// findCommentElement 中使用短超时 context 查找，返回的 element 可能绑定到已过期的 context
	// 这里先确认评论存在，再用主 page context 重新获取（避免 context deadline exceeded）
	if _, findErr := findCommentElement(page, commentID, userID); findErr != nil {
		return fmt.Errorf("无法找到评论: %w", findErr)
	}

	// 用主 page context（5min 超时）重新获取评论元素
	selector := fmt.Sprintf("#comment-%s", commentID)
	commentEl, err := page.Timeout(10 * time.Second).Element(selector)
	if err != nil {
		return fmt.Errorf("重新获取评论元素失败: %w", err)
	}
	logrus.Infof("已用主 context 重新获取评论元素: %s", selector)

	if err := commentEl.ScrollIntoView(); err != nil {
		logrus.Warnf("滚动到评论位置失败: %v", err)
	}
	time.Sleep(1 * time.Second)

	// 多策略查找并点击回复按钮
	if err := clickReplyButton(page, commentEl); err != nil {
		return fmt.Errorf("回复按钮点击失败: %w", err)
	}

	time.Sleep(1 * time.Second)

	// 查找回复输入框
	inputEl, err := page.Timeout(5 * time.Second).Element("#content-textarea, div.input-box div.content-edit p.content-input")
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}
	if err := inputEl.Input(content); err != nil {
		return fmt.Errorf("输入回复内容失败: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	submitBtn, err := page.Timeout(5 * time.Second).Element("div.bottom button.submit, button.submit")
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

// findCommentElement 查找指定评论元素（参考 feed_detail.go 的滚动逻辑）
func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	const maxAttempts = 100
	const scrollInterval = 800 * time.Millisecond

	scrollToCommentsArea(page)
	time.Sleep(1 * time.Second)

	// 先尝试直接查找（评论可能已在可视区域内）
	if el := tryFindComment(page, commentID, userID); el != nil {
		logrus.Info("直接找到目标评论，无需滚动")
		return el, nil
	}

	var lastCommentCount int
	stagnantChecks := 0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt%10 == 0 {
			logrus.Infof("查找尝试 %d/%d", attempt+1, maxAttempts)
		}

		if checkEndContainer(page) {
			logrus.Info("已到达评论底部，未找到目标评论")
			break
		}

		currentCount := getCommentCount(page)
		if currentCount != lastCommentCount {
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
		}

		if stagnantChecks >= 10 {
			logrus.Info("评论数量停滞超过10次，可能已加载完所有评论")
			break
		}

		// 滚动触发懒加载
		if currentCount > 0 {
			elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment")
			if err == nil && len(elements) > 0 {
				_ = elements[len(elements)-1].ScrollIntoView()
			}
			time.Sleep(300 * time.Millisecond)
		}

		_, _ = page.Eval(`() => { window.scrollBy(0, window.innerHeight * 0.8); return true; }`)
		time.Sleep(500 * time.Millisecond)

		// 滚动后查找
		if el := tryFindComment(page, commentID, userID); el != nil {
			logrus.Infof("在第 %d 次尝试找到目标评论", attempt+1)
			return el, nil
		}

		time.Sleep(scrollInterval)
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s), 尝试次数: %d", commentID, userID, maxAttempts)
}

// clickReplyButton 多策略查找并点击评论的回复按钮
func clickReplyButton(page *rod.Page, commentEl *rod.Element) error {
	// 调试: 用 page.Eval + element 引用获取 HTML（rod 中 Element.Eval 使用 this）
	html, err := commentEl.Eval(`function() { return this.outerHTML.substring(0, 3000); }`)
	if err != nil {
		logrus.Warnf("获取评论 HTML 失败: %v", err)
	} else {
		logrus.Infof("评论元素 outerHTML (截取): %s", html.Value.Str())
	}

	// 策略1: CSS 选择器（覆盖多种可能的类名）
	selectors := []string{
		".right .interactions .reply",
		".interactions .reply",
		".reply-btn",
		"span.reply",
		"[class*='reply']",
		".comment-op .reply",
		".operation .reply",
	}
	for _, sel := range selectors {
		btn, err := commentEl.Timeout(1 * time.Second).Element(sel)
		if err == nil && btn != nil {
			logrus.Infof("策略1: 通过选择器 '%s' 找到回复按钮", sel)
			if clickErr := btn.Click(proto.InputMouseButtonLeft, 1); clickErr == nil {
				return nil
			}
			logrus.Warnf("点击失败，尝试下一个选择器")
		}
	}
	logrus.Warn("策略1(CSS选择器)均失败，尝试策略2(JS文本搜索)")

	// 策略2: JS 搜索含"回复"文本的元素并点击（使用 function 而非箭头函数）
	clicked, err := commentEl.Eval(`function() {
		var candidates = this.querySelectorAll('span, a, div, button, p, label');
		for (var i = 0; i < candidates.length; i++) {
			var text = candidates[i].textContent.trim();
			if (text === '回复' || text === 'Reply' || text === '回复评论') {
				candidates[i].click();
				return true;
			}
		}
		return false;
	}`)
	if err == nil && clicked.Value.Bool() {
		logrus.Info("策略2: 通过JS文本搜索找到并点击了回复按钮")
		return nil
	}
	logrus.Warn("策略2(JS文本搜索)失败，尝试策略3(点击评论文字)")

	// 策略3: 直接点击评论文字区域（小红书点击评论文字可触发回复输入框）
	contentClicked, err := commentEl.Eval(`function() {
		// 尝试点击评论文本内容
		var all = this.querySelectorAll('*');
		for (var i = 0; i < all.length; i++) {
			var el = all[i];
			if (el.children.length === 0 && el.textContent.trim().length > 5) {
				el.click();
				return true;
			}
		}
		return false;
	}`)
	if err == nil && contentClicked.Value.Bool() {
		logrus.Info("策略3: 点击评论文字触发回复")
		return nil
	}

	return fmt.Errorf("所有策略均失败，请检查页面DOM结构")
}

// tryFindComment 尝试在当前 DOM 中查找目标评论
func tryFindComment(page *rod.Page, commentID, userID string) *rod.Element {
	// 优先通过 commentID 查找
	if commentID != "" {
		selector := fmt.Sprintf("#comment-%s", commentID)
		el, err := page.Timeout(1 * time.Second).Element(selector)
		if err == nil && el != nil {
			return el
		}
	}

	// 通过 userID 查找
	if userID != "" {
		elements, err := page.Timeout(1 * time.Second).Elements(".comment-item")
		if err == nil {
			for _, el := range elements {
				userEl, err := el.Timeout(300 * time.Millisecond).Element(fmt.Sprintf(`[data-user-id="%s"]`, userID))
				if err == nil && userEl != nil {
					return el
				}
			}
		}
	}

	return nil
}
