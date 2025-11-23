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

	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)

	if err := page.Navigate(url); err != nil {
		logrus.Warnf("Failed to navigate to feed detail page: %v", err)
		return fmt.Errorf("无法打开帖子详情页，该帖子可能在网页端不可访问: %w", err)
	}

	if err := page.WaitStable(2 * time.Second); err != nil {
		logrus.Warnf("Failed to wait for page stable: %v", err)
		return fmt.Errorf("页面加载超时，该帖子可能在网页端不可访问: %w", err)
	}

	time.Sleep(1 * time.Second)

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
	page := f.page.Context(ctx).Timeout(5 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行回复: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(3 * time.Second)

	// 等待评论容器加载
	waitForCommentsContainer(page)
	time.Sleep(2 * time.Second)

	// 使用新的查找逻辑（完全在 JS 中执行）
	commentEl, err := findCommentElementNew(page, commentID, userID)
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}

	// 多次滚动确保可见
	for i := 0; i < 3; i++ {
		logrus.Infof("第 %d 次滚动到评论位置...", i+1)
		_, _ = commentEl.Eval(`() => { 
			this.scrollIntoView({behavior: "instant", block: "center"}); 
			return true 
		}`)
		time.Sleep(1500 * time.Millisecond)

		// 往下多滚动一点
		page.MustEval(`() => window.scrollBy(0, 150)`)
		time.Sleep(500 * time.Millisecond)
	}

	logrus.Info("滚动完成，准备点击回复按钮")

	// 查找并点击回复按钮
	replyBtn, err := findReplyButton(commentEl)
	if err != nil {
		return fmt.Errorf("无法找到回复按钮: %w", err)
	}

	if !tryClickChainForComment(replyBtn) {
		return fmt.Errorf("点击回复按钮失败")
	}

	time.Sleep(2 * time.Second)

	// 查找回复输入框
	inputEl, err := findReplyInput(page, commentEl)
	if err != nil {
		return fmt.Errorf("无法找到回复输入框: %w", err)
	}

	// 聚焦并输入内容
	if _, evalErr := inputEl.Eval(`() => { 
		try { 
			this.focus(); 
		} catch (e) {} 
		return true 
	}`); evalErr != nil {
		logrus.Warnf("focus reply input failed: %v", evalErr)
	}

	inputEl.MustInput(content)
	time.Sleep(500 * time.Millisecond)

	// 查找并点击提交按钮
	submitBtn, err := findSubmitButton(page)
	if err != nil {
		return fmt.Errorf("无法找到提交按钮: %w", err)
	}

	if !tryClickChainForComment(submitBtn) {
		return fmt.Errorf("点击回复提交按钮失败")
	}

	time.Sleep(3 * time.Second)
	return nil
}

func findCommentElementNew(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	logrus.Infof("🔍 开始查找评论（新方法）- commentID: %s, userID: %s", commentID, userID)

	// 修改 JS：找到后记录元素的 ID
	findCommentJS := fmt.Sprintf(`async () => {
		const INTERVAL_MS = 900;
		const STAGNANT_LIMIT = 8;
		const NO_CHANGE_SCROLL_LIMIT = 3;
		const DELTA_MIN = 480;
		const SCROLL_TIMEOUT = 900;
		const MAX_ATTEMPTS = 100;
		const CLICK_MORE_INTERVAL = 2;
		const CLICK_WAIT_TIME = 300;

		const TARGET_COMMENT_ID = %q;
		const TARGET_USER_ID = %q;

		console.log('开始查找评论 - TARGET_COMMENT_ID:', TARGET_COMMENT_ID, 'TARGET_USER_ID:', TARGET_USER_ID);

		const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
		const scrollRoot = () => document.scrollingElement || document.documentElement || document.body;
		const getContainer = () => document.querySelector('.comments-container');
		
		const clickShowMoreButtons = () => {
			let clickedCount = 0;
			const elements = document.querySelectorAll('.show-more');
			
			elements.forEach((el) => {
				try {
					const rect = el.getBoundingClientRect();
					const style = window.getComputedStyle(el);
					const isVisible = (
						rect.height > 0 &&
						rect.width > 0 &&
						style.display !== 'none' &&
						style.visibility !== 'hidden' &&
						style.opacity !== '0' &&
						rect.top < window.innerHeight + 500 &&
						rect.bottom > -500
					);
					
					if (isVisible) {
						el.click();
						clickedCount++;
					}
				} catch (err) {
					console.debug('点击失败', err);
				}
			});
			
			return clickedCount;
		};

		// === 修改：返回元素的稳定标识符 ===
		const findTargetComment = () => {
			// 优先通过 commentID 查找
			if (TARGET_COMMENT_ID) {
				const byId = document.querySelector('#comment-' + TARGET_COMMENT_ID);
				if (byId) {
					console.log('通过 commentID 找到评论:', TARGET_COMMENT_ID);
					// 返回包含完整信息的对象
					return {
						element: byId,
						selector: '#comment-' + TARGET_COMMENT_ID,
						commentId: TARGET_COMMENT_ID
					};
				}
			}
			
			// 通过 userID 查找
			if (TARGET_USER_ID) {
				const allComments = document.querySelectorAll('.comment-item, .comment');
				for (const comment of allComments) {
					const userIdEl = comment.querySelector('[data-user-id="' + TARGET_USER_ID + '"]');
					if (userIdEl) {
						console.log('通过 userID 找到评论:', TARGET_USER_ID);
						
						// 尝试获取评论的 ID
						const commentId = comment.id;
						if (commentId) {
							return {
								element: comment,
								selector: '#' + commentId,
								commentId: commentId.replace('comment-', '')
							};
						} else {
							// 如果没有 ID，给它添加一个唯一标识
							const uniqueId = 'xhs-found-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
							comment.id = uniqueId;
							return {
								element: comment,
								selector: '#' + uniqueId,
								commentId: null
							};
						}
					}
				}
			}
			
			return null;
		};

		// ... (保留原有的滚动逻辑) ...
		const getScrollMetrics = (el) => {
			if (!el) {
				return { top: 0, max: 0, client: window.innerHeight };
			}
			if (el === window || el === document || el === document.body || el === document.documentElement) {
				const root = scrollRoot();
				return {
					top: root.scrollTop,
					max: Math.max(root.scrollHeight - root.clientHeight, 0),
					client: root.clientHeight || window.innerHeight
				};
			}
			return {
				top: el.scrollTop,
				max: Math.max(el.scrollHeight - el.clientHeight, 0),
				client: el.clientHeight
			};
		};

		const setScrollTop = (el, value) => {
			if (!el) return;
			if (el === window || el === document || el === document.body || el === document.documentElement) {
				const root = scrollRoot();
				root.scrollTop = value;
				window.scrollTo(0, value);
				return;
			}
			el.scrollTop = value;
		};

		const dispatchWheel = (el, delta) => {
			if (!el) return;
			try {
				const wheel = new WheelEvent('wheel', {
					deltaY: delta,
					bubbles: true,
					cancelable: true
				});
				el.dispatchEvent(wheel);
				el.dispatchEvent(new Event('scroll', { bubbles: true }));
			} catch (err) {
				console.debug('dispatchWheel error', err);
			}
		};

		const findScrollTarget = () => {
			const container = getContainer();
			const candidates = new Set();
			
			if (container) {
				let current = container;
				while (current) {
					if (current instanceof HTMLElement) {
						candidates.add(current);
					}
					current = current.parentElement;
				}
			}
			
			candidates.add(document.body);
			candidates.add(document.documentElement);
			
			const weighted = Array.from(candidates).map((node) => {
				const style = window.getComputedStyle(node);
				const overflowY = style.overflowY;
				const scrollable = node.scrollHeight - node.clientHeight > 40;
				const hasScrollStyle = /auto|scroll|overlay/i.test(overflowY);
				const weight =
					(node.contains(container) ? 1000 : 0) +
					(node === container ? 800 : 0) +
					(hasScrollStyle ? 400 : 0) +
					(scrollable ? 300 : 0) -
					(node === document.body || node === document.documentElement ? 50 : 0);
				
				if (scrollable || hasScrollStyle || node === document.body || node === document.documentElement) {
					return { node, weight };
				}
				return null;
			}).filter(Boolean);
			
			weighted.sort((a, b) => b.weight - a.weight);
			
			return weighted.length > 0 ? weighted[0].node : scrollRoot();
		};

		const performScroll = (target) => {
			const scrollTarget = target || findScrollTarget();
			if (!scrollTarget) {
				window.scrollBy(0, window.innerHeight * 0.8);
				return;
			}
			
			const metrics = getScrollMetrics(scrollTarget);
			const beforeTop = metrics.top;
			const desired = metrics.max > 0 
				? Math.min(metrics.top + Math.max(metrics.client * 0.85, DELTA_MIN), metrics.max) 
				: metrics.top + Math.max(metrics.client * 0.85, DELTA_MIN);
			const applied = Math.max(0, desired - metrics.top);
			
			setScrollTop(scrollTarget, desired);
			dispatchWheel(scrollTarget, applied);
			
			const afterTop = getScrollMetrics(scrollTarget).top;
			if (Math.abs(afterTop - beforeTop) < 5 && scrollTarget !== scrollRoot()) {
				const root = scrollRoot();
				const rootBefore = root.scrollTop;
				root.scrollTop = rootBefore + applied;
				window.scrollBy(0, applied);
				dispatchWheel(root, applied);
			}
		};

		// 主查找逻辑
		let lastScrollTop = 0;
		let stagnantChecks = 0;
		let noScrollChangeCount = 0;
		let totalClickedButtons = 0;

		for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt++) {
			const container = getContainer();
			if (!container) {
				await sleep(300);
				continue;
			}

			if (attempt %% CLICK_MORE_INTERVAL === 0) {
				const clicked = clickShowMoreButtons();
				if (clicked > 0) {
					totalClickedButtons += clicked;
					console.log('点击了 ' + clicked + ' 个"更多"按钮，累计: ' + totalClickedButtons);
					await sleep(CLICK_WAIT_TIME);
					
					await sleep(200);
					const clicked2 = clickShowMoreButtons();
					if (clicked2 > 0) {
						totalClickedButtons += clicked2;
						console.log('二次检查点击了 ' + clicked2 + ' 个"更多"按钮');
						await sleep(CLICK_WAIT_TIME);
					}
					
					const foundInfo = findTargetComment();
					if (foundInfo) {
						console.log('点击"更多"后找到评论，总共点击了 ' + totalClickedButtons + ' 个按钮');
						return { 
							status: 'found', 
							attempts: attempt + 1, 
							clickedButtons: totalClickedButtons,
							selector: foundInfo.selector,
							commentId: foundInfo.commentId
						};
					}
				}
			}

			const foundInfo = findTargetComment();
			if (foundInfo) {
				console.log('找到评论，尝试次数: ' + (attempt + 1) + '，总共点击了 ' + totalClickedButtons + ' 个按钮');
				return { 
					status: 'found', 
					attempts: attempt + 1, 
					clickedButtons: totalClickedButtons,
					selector: foundInfo.selector,
					commentId: foundInfo.commentId
				};
			}

			const target = findScrollTarget();
			const beforeTop = getScrollMetrics(target).top;
			performScroll(target);
			await sleep(SCROLL_TIMEOUT);
			const afterTop = getScrollMetrics(target).top;

			if (Math.abs(afterTop - beforeTop) < 5) {
				noScrollChangeCount += 1;
			} else {
				noScrollChangeCount = 0;
				lastScrollTop = afterTop;
			}

			if (noScrollChangeCount >= NO_CHANGE_SCROLL_LIMIT) {
				return { status: 'not_found', reason: 'no-scroll-change', attempts: attempt + 1, clickedButtons: totalClickedButtons };
			}

			if (INTERVAL_MS > SCROLL_TIMEOUT) {
				await sleep(INTERVAL_MS - SCROLL_TIMEOUT);
			}
		}

		return { status: 'not_found', reason: 'timeout', attempts: MAX_ATTEMPTS, clickedButtons: totalClickedButtons };
	}`, commentID, userID)

	// 执行 JS
	result, err := page.Eval(findCommentJS)
	if err != nil {
		logrus.Errorf("执行查找评论 JS 失败: %v", err)
		return nil, fmt.Errorf("执行查找评论 JS 失败: %w", err)
	}

	// 解析结果
	resultJSON, err := page.ObjectToJSON(result)
	if err != nil {
		logrus.Errorf("无法将结果转换为 JSON: %v", err)
		return nil, fmt.Errorf("无法将结果转换为 JSON: %w", err)
	}

	status := resultJSON.Get("status").Str()
	reason := resultJSON.Get("reason").Str()
	attempts := resultJSON.Get("attempts").Int()
	clickedButtons := resultJSON.Get("clickedButtons").Int()
	selector := resultJSON.Get("selector").Str()

	logrus.Infof("查找结果: status=%s, reason=%s, attempts=%d, clickedButtons=%d, selector=%s",
		status, reason, attempts, clickedButtons, selector)

	if status != "found" {
		return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s), 原因: %s, 尝试次数: %d, 点击按钮: %d",
			commentID, userID, reason, attempts, clickedButtons)
	}

	// === 关键修改：使用返回的稳定选择器而不是临时标记 ===
	el, err := page.Element(selector)
	if err != nil {
		logrus.Errorf("找到评论但无法获取元素，选择器: %s, 错误: %v", selector, err)

		// 如果稳定选择器失败，尝试重新查找
		logrus.Info("尝试通过 commentID 重新查找...")
		if commentID != "" {
			fallbackSelector := fmt.Sprintf("#comment-%s", commentID)
			el, err = page.Element(fallbackSelector)
			if err == nil {
				logrus.Infof("通过备用选择器 %s 成功找到元素", fallbackSelector)
				return el, nil
			}
		}

		return nil, fmt.Errorf("找到评论但无法获取元素: %w", err)
	}

	logrus.Infof("✓ 成功获取评论元素，选择器: %s", selector)
	return el, nil
}
func waitForCommentsContainer(page *rod.Page) {
	jsCode := `() => {
		let attempts = 0;
		const maxAttempts = 10;
		
		const checkContainer = () => {
			const container = document.querySelector('.comments-container');
			if (container) {
				const comments = container.querySelectorAll('.comment-item, .comment');
				return comments.length > 0;
			}
			return false;
		};
		
		const interval = setInterval(() => {
			attempts++;
			if (checkContainer() || attempts >= maxAttempts) {
				clearInterval(interval);
			}
		}, 500);
		
		return checkContainer();
	}`

	page.Eval(jsCode)
	time.Sleep(2 * time.Second)
}

func findReplyButton(commentEl *rod.Element) (*rod.Element, error) {
	if commentEl == nil {
		return nil, fmt.Errorf("评论元素为空")
	}

	selector := ".right .interactions .reply"
	btn, err := commentEl.Element(selector)
	if err != nil || btn == nil {
		logrus.Warnf("未找到回复按钮，选择器: %s, err: %v", selector, err)
		return nil, fmt.Errorf("未找到回复按钮")
	}

	logrus.Infof("通过选择器 %s 找到回复按钮", selector)
	return btn, nil
}

func verifyClickSuccess(clickedEl *rod.Element) bool {
	page := clickedEl.Page()
	selectors := []string{
		"div.input-box div.content-edit p.content-input",
	}

	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			if visible, _ := el.Visible(); visible {
				logrus.Infof("验证成功：找到可见的回复输入框 (%s)", selector)
				return true
			}
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
		"div.input-box div.content-edit p.content-input",
	}
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

	text, _ := el.Text()
	classAttr, _ := el.Attribute("class")
	class := ""
	if classAttr != nil {
		class = *classAttr
	}
	tagName := ""
	if desc, err := el.Describe(0, false); err == nil && desc != nil {
		tagName = desc.NodeName
	}
	logrus.Infof("准备点击元素 - 文本: '%s', 类: '%s', 标签: %s", text, class, tagName)

	visible, _ := el.Visible()
	logrus.Infof("元素可见性: %v", visible)

	_, _ = el.Eval(`() => { 
		try { 
			this.scrollIntoView({behavior: "instant", block: "center"}); 
		} catch (e) {} 
		return true 
	}`)
	time.Sleep(500 * time.Millisecond)

	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		logrus.Warnf("点击失败: %v", err)
		return false
	}

	logrus.Infof("点击成功")
	time.Sleep(1 * time.Second)

	success := verifyClickSuccess(el)
	if success {
		logrus.Infof("点击执行成功且有效")
		return true
	}

	logrus.Warnf("点击执行成功但无效（没有出现回复输入框）")
	return false
}

func findSubmitButton(page *rod.Page) (*rod.Element, error) {
	selectors := []string{
		"div.bottom button.submit",
	}
	for _, selector := range selectors {
		if el, err := page.Element(selector); err == nil && el != nil {
			disabled, _ := el.Attribute("disabled")
			if disabled == nil {
				return el, nil
			}
		}
	}
	return nil, fmt.Errorf("未找到回复发布按钮")
}
