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

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	if err := checkPageAccessible(page); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	commentEl, err := findCommentElement(page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	if err := commentEl.ScrollIntoView(); err != nil {
		logrus.Warnf("滚动到评论位置失败: %v", err)
	}
	time.Sleep(1 * time.Second)

	// 查找回复按钮（新旧选择器兼容）
	replyBtn, err := commentEl.Timeout(5 * time.Second).Element(".interactions .reply, .right .interactions .reply")
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}
	if err := replyBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击回复按钮失败: %w", err)
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
