package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// commentPageAPIResponse 小红书评论列表 API 的原始响应结构
type commentPageAPIResponse struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Data    struct {
		Comments []struct {
			ID          string `json:"id"`
			Content     string `json:"content"`
			SubComments []struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"sub_comments"`
		} `json:"comments"`
		HasMore bool   `json:"has_more"`
		Cursor  string `json:"cursor"`
	} `json:"data"`
}

// commentAPIEntry 单次评论 API 响应的摘要
type commentAPIEntry struct {
	body    string
	hasMore bool
}

// commentIDExistsInAPIEntries 检查 commentID 是否在已捕获的 API 响应中
func commentIDExistsInAPIEntries(entries []commentAPIEntry, commentID string) bool {
	for _, entry := range entries {
		var resp commentPageAPIResponse
		if err := json.Unmarshal([]byte(entry.body), &resp); err != nil {
			continue
		}
		for _, c := range resp.Data.Comments {
			if c.ID == commentID {
				return true
			}
			for _, sub := range c.SubComments {
				if sub.ID == commentID {
					return true
				}
			}
		}
	}
	return false
}

// findParentCommentIDFromAPIEntries 在已捕获的评论 API 响应中查找某个 commentID 属于哪个顶级父评论。
// 用于容错：当 parentCommentID 本身是子评论 ID 时，从 API 数据反查其真正的顶级父评论。
func findParentCommentIDFromAPIEntries(apiEntries *[]commentAPIEntry, apiMu *sync.Mutex, commentID string) string {
	apiMu.Lock()
	entries := make([]commentAPIEntry, len(*apiEntries))
	copy(entries, *apiEntries)
	apiMu.Unlock()

	for _, entry := range entries {
		var resp commentPageAPIResponse
		if err := json.Unmarshal([]byte(entry.body), &resp); err != nil {
			continue
		}
		for _, c := range resp.Data.Comments {
			for _, sub := range c.SubComments {
				if sub.ID == commentID {
					return c.ID
				}
			}
		}
	}
	return ""
}

// findParentCommentIDWithScroll 在评论 API 数据中查找某个 commentID 的顶级父评论 ID。
// 与 findParentCommentIDFromAPIEntries 不同，当已有数据不足时，会触发页面滚动加载更多评论数据再重试。
// 用于 parentCommentID 本身是子评论 ID 的容错场景。
func findParentCommentIDWithScroll(page *rod.Page, apiEntries *[]commentAPIEntry, apiMu *sync.Mutex, commentID string) string {
	const maxScrollRounds = 10

	for round := 0; round <= maxScrollRounds; round++ {
		if found := findParentCommentIDFromAPIEntries(apiEntries, apiMu, commentID); found != "" {
			return found
		}

		// 检查是否已无更多数据
		apiMu.Lock()
		entries := make([]commentAPIEntry, len(*apiEntries))
		copy(entries, *apiEntries)
		apiMu.Unlock()

		if len(entries) > 0 && !entries[len(entries)-1].hasMore {
			logrus.Infof("反查父评论：API数据已全部加载，commentID=%s 不在任何顶级评论的子评论中", commentID)
			return ""
		}

		if round == maxScrollRounds {
			break
		}

		// 触发滚动加载更多评论
		lastCount := len(entries)
		logrus.Infof("反查父评论：第 %d 轮，已有 %d 批数据，滚动加载更多...", round+1, lastCount)
		_, _ = page.Eval(`() => { window.scrollBy(0, window.innerHeight * 0.8); return true; }`)

		// 等待新数据
		for i := 0; i < 5; i++ {
			time.Sleep(1 * time.Second)
			apiMu.Lock()
			newCount := len(*apiEntries)
			apiMu.Unlock()
			if newCount > lastCount {
				break
			}
		}
	}

	return ""
}

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
	page := f.page.Timeout(5 * time.Minute)

	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)

	// 小红书是 SPA，直接打开帖子 URL 时路由初始化不完整
	// 需要先访问主页让 SPA 完全初始化，再导航到帖子页面
	logrus.Info("预热：先访问小红书主页初始化 SPA...")
	page.MustNavigate("https://www.xiaohongshu.com/")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	logrus.Infof("导航到帖子详情页: %s", url)
	page.MustNavigate(url)
	page.MustWaitDOMStable()

	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		result := page.MustEval(`() => document.querySelector('#noteContainer, .note-container, .note-scroller, .comments-container') ? 1 : 0`)
		if result.Int() == 1 {
			logrus.Infof("页面内容已渲染（%ds）", i+1)
			break
		}
	}

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
// parentCommentID 为可选参数：当目标评论是子评论时，传入父评论 ID，
// 浏览器会先找到并展开父评论的"查看回复"列表，再定位目标子评论。
func (f *CommentFeedAction) ReplyToComment(ctx context.Context, feedID, xsecToken, commentID, userID, parentCommentID, content string) error {
	// 注意：不使用 Context(ctx)，避免继承外部 context 的超时
	page := f.page.Timeout(5 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行回复: %s", url)

	// 方案A+B：在导航之前注册 HijackRequests，拦截评论 API 响应。
	// HijackRequests 必须在页面导航前注册，才能捕获页面加载时发出的 API 请求。
	var commentAPIEntries []commentAPIEntry
	var commentAPIMu sync.Mutex
	var commentAPIWorked bool

	if commentID != "" {
		router := page.HijackRequests()
		go router.Run()
		defer router.Stop()

		router.MustAdd("*/api/sns/web/v2/comment/page*", func(ctx *rod.Hijack) {
			ctx.MustLoadResponse()
			body := ctx.Response.Body()
			if body == "" {
				return
			}
			var resp commentPageAPIResponse
			if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.Success {
				return
			}
			commentAPIMu.Lock()
			commentAPIEntries = append(commentAPIEntries, commentAPIEntry{
				body:    body,
				hasMore: resp.Data.HasMore,
			})
			commentAPIMu.Unlock()
			logrus.Infof("API预检：捕获到评论API响应（%d条评论，has_more=%v）",
				len(resp.Data.Comments), resp.Data.HasMore)
		})
		logrus.Info("API预检：已注册评论API拦截器")
	}

	// 小红书是 SPA，直接打开帖子 URL 时路由初始化不完整，评论区不会渲染
	// 需要先访问主页让 SPA 完全初始化，再导航到帖子页面
	logrus.Info("预热：先访问小红书主页初始化 SPA...")
	page.MustNavigate("https://www.xiaohongshu.com/")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 再导航到帖子详情页（此时拦截器已就绪，会自动捕获评论API请求）
	logrus.Infof("导航到帖子详情页: %s", url)
	page.MustNavigate(url)
	page.MustWaitDOMStable()

	// 等待评论区容器渲染（最多 15 秒）
	logrus.Info("等待评论区渲染...")
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		result := page.MustEval(`() => document.querySelector('#noteContainer, .note-container, .note-scroller, .comments-container') ? 1 : 0`)
		if result.Int() == 1 {
			logrus.Infof("评论区已渲染（%ds）", i+1)
			break
		}
	}

	// 检测页面是否可访问
	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 方案A+B：利用已拦截的 API 数据 + DOM 滚动联合查找评论。
	// API 拦截器在导航前已注册，页面加载时会自动捕获评论 API 响应。
	// findCommentElementWithAPICheck 会在每次滚动后同步检查 API 数据，
	// 一旦 API 返回 has_more=false 且未找到目标评论，立即终止，无需滚到 DOM 底部。
	var commentEl *rod.Element
	var err error
	if parentCommentID != "" {
		// 子评论路径：先找到父评论，展开"查看回复"，再在子评论列表中找目标评论。
		// 注意：调用方传入的 parentCommentID 可能来自通知 API 的 target_comment_id，
		// 该值语义是"被回复的评论"，不一定是顶级评论——当对话已有多轮时，
		// target_comment_id 可能是 Liko 自己的子评论，而非顶级父评论。
		// 因此这里先尝试直接用 parentCommentID 走子评论路径，
		// 若失败则检查 parentCommentID 本身是否是子评论，并反查其真正的顶级父评论。
		logrus.Infof("目标是子评论，先找父评论 %s，然后展开子评论列表", parentCommentID)
		commentEl, err = findSubComment(page, parentCommentID, commentID, userID, &commentAPIEntries, &commentAPIMu)
		if err != nil {
			// 容错：parentCommentID 找不到，可能它本身是子评论（来自通知 API 的 target_comment_id）。
			// 小红书评论只有两层，尝试把 parentCommentID 当子评论 ID，从 API 数据中反查其顶级父评论。
			// 使用带滚动的版本，确保 API 数据不足时能加载更多再重试。
			logrus.Warnf("父评论 %s 未找到，尝试将其作为子评论 ID 反查顶级父评论（通知 API 的 target_comment_id 可能是子评论）", parentCommentID)
			if trueParentID := findParentCommentIDWithScroll(page, &commentAPIEntries, &commentAPIMu, parentCommentID); trueParentID != "" {
				logrus.Infof("从API数据中发现 %s 是 %s 的子评论，使用真正的顶级父评论重试", parentCommentID, trueParentID)
				commentEl, err = findSubComment(page, trueParentID, commentID, userID, &commentAPIEntries, &commentAPIMu)
			}
		}
	} else {
		commentEl, err = findCommentElementWithAPICheck(page, commentID, userID, &commentAPIEntries, &commentAPIMu)
		if err != nil && commentID != "" {
			// 容错：顶级评论中没找到，可能是子评论但调用方未传 parentCommentID。
			// 尝试从 API 数据中查找 commentID 所在的父评论，然后走子评论路径。
			// 使用带滚动的版本，确保 API 数据不足时能加载更多再重试。
			logrus.Warnf("顶级评论未找到 %s，尝试在API数据中搜索其父评论（调用方可能遗漏了 parent_comment_id）", commentID)
			if foundParentID := findParentCommentIDWithScroll(page, &commentAPIEntries, &commentAPIMu, commentID); foundParentID != "" {
				logrus.Infof("从API数据中发现 %s 是 %s 的子评论，切换到子评论查找路径", commentID, foundParentID)
				commentEl, err = findSubComment(page, foundParentID, commentID, userID, &commentAPIEntries, &commentAPIMu)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("无法找到评论: %w", err)
	}
	_ = commentAPIWorked

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

// findCommentElementWithAPICheck 查找指定评论元素，同时利用 API 拦截数据加速判断。
//
// 终止条件（按优先级）：
//  1. 在 DOM 中找到目标评论 → 返回元素
//  2. API 数据确认 has_more=false 且未找到 → 评论已删除，立即返回错误
//  3. 检测到 .end-container → 已加载全部评论，目标不存在
//  4. 评论数停滞 3 次 → 已加载完毕，目标不存在
//  5. 超过 maxScrollRounds 轮 → 安全上限
//
// apiEntries/apiMu 是在导航前注册的拦截器收集的 API 响应，
// 每次滚动后会有新的 API 响应追加进来，实时检查。
func findCommentElementWithAPICheck(page *rod.Page, commentID, userID string, apiEntries *[]commentAPIEntry, apiMu *sync.Mutex) (*rod.Element, error) {
	logrus.Infof("开始查找评论 - commentID: %s, userID: %s", commentID, userID)

	const maxScrollRounds = 15
	const scrollInterval = 500 * time.Millisecond

	// 先滚动到评论区
	scrollToCommentsArea(page)
	time.Sleep(1 * time.Second)

	// 预检：直接查找（评论数少或页面已完全加载时直接命中）
	if commentID != "" {
		selector := fmt.Sprintf("#comment-%s", commentID)
		el, err := page.Timeout(3 * time.Second).Element(selector)
		if err == nil && el != nil {
			// 滚动到视口触发虚拟化渲染，验证内容非空
			_ = el.ScrollIntoView()
			time.Sleep(500 * time.Millisecond)
			text, _ := el.Text()
			if text != "" {
				logrus.Infof("✓ 预检直接找到评论（内容已渲染）: %s", commentID)
				return el, nil
			}
			logrus.Infof("预检：找到占位元素但内容为空，进入滚动查找")
		}

		// 检查初始 API 数据
		apiMu.Lock()
		initialEntries := make([]commentAPIEntry, len(*apiEntries))
		copy(initialEntries, *apiEntries)
		apiMu.Unlock()

		if len(initialEntries) > 0 {
			logrus.Infof("预检：已有 %d 批API数据，开始检查", len(initialEntries))
			if commentIDExistsInAPIEntries(initialEntries, commentID) {
				logrus.Infof("✓ 预检API数据中找到 commentID=%s，进入DOM定位", commentID)
			} else {
				lastHasMore := initialEntries[len(initialEntries)-1].hasMore
				if !lastHasMore {
					logrus.Infof("API确认：评论API已无更多页，commentID=%s 不存在（已删除）", commentID)
					return nil, fmt.Errorf("评论不存在（已被删除或不可见）: commentID=%s", commentID)
				}
				logrus.Infof("预检API数据未找到，评论可能在后续页，进入滚动查找（最多 %d 轮）", maxScrollRounds)
			}
		} else {
			logrus.Infof("预检：暂无API数据，进入滚动查找（最多 %d 轮）", maxScrollRounds)
		}
	}

	var lastCommentCount = 0
	var lastAPICount = 0
	stagnantChecks := 0

	for round := 0; round < maxScrollRounds; round++ {
		logrus.Infof("=== 滚动轮次 %d/%d ===", round+1, maxScrollRounds)

		// 1. 先尝试在 DOM 中查找目标评论（需验证内容非空，避免虚拟化渲染占位符）
		if commentID != "" {
			selector := fmt.Sprintf("#comment-%s", commentID)
			el, err := page.Timeout(2 * time.Second).Element(selector)
			if err == nil && el != nil {
				// scrollIntoView 触发虚拟化渲染
				_ = el.ScrollIntoView()
				time.Sleep(400 * time.Millisecond)
				text, _ := el.Text()
				if text != "" {
					logrus.Infof("✓ 找到评论（内容已渲染）: %s（第 %d 轮）", commentID, round+1)
					return el, nil
				}
				logrus.Infof("找到DOM占位元素但内容为空，继续滚动等待渲染（第 %d 轮）", round+1)
			}
		}

		// 2. 检查新增的 API 数据（每次滚动后可能有新响应）
		if commentID != "" {
			apiMu.Lock()
			currentEntries := make([]commentAPIEntry, len(*apiEntries))
			copy(currentEntries, *apiEntries)
			apiMu.Unlock()

			if len(currentEntries) > lastAPICount {
				logrus.Infof("API检查：新增 %d 批数据（共 %d 批），检查 commentID=%s",
					len(currentEntries)-lastAPICount, len(currentEntries), commentID)
				lastAPICount = len(currentEntries)

				if commentIDExistsInAPIEntries(currentEntries, commentID) {
					logrus.Infof("✓ API数据确认评论存在，继续DOM定位")
					// 评论存在，继续 DOM 滚动查找，不提前终止
				} else {
					lastHasMore := currentEntries[len(currentEntries)-1].hasMore
					if !lastHasMore {
						logrus.Infof("API确认：所有评论页已加载完毕，commentID=%s 不存在（已删除）", commentID)
						return nil, fmt.Errorf("评论不存在（已被删除或不可见）: commentID=%s", commentID)
					}
				}
			}
		}

		// 3. 检查是否已到 DOM 底部
		if checkEndContainer(page) {
			logrus.Info("已到达评论底部，目标评论不存在（可能已被删除）")
			break
		}

		// 4. 获取当前评论数量，检测停滞
		currentCount := getCommentCount(page)
		logrus.Infof("当前评论数: %d", currentCount)

		if currentCount != lastCommentCount {
			logrus.Infof("评论数增加: %d -> %d", lastCommentCount, currentCount)
			lastCommentCount = currentCount
			stagnantChecks = 0
		} else {
			stagnantChecks++
			logrus.Infof("评论数停滞（%d/3）", stagnantChecks)
			if stagnantChecks >= 3 {
				logrus.Info("评论数连续停滞，已加载全部评论，目标评论不存在")
				break
			}
		}

		// 5. 通过 userID 查找（备用方式）
		if userID != "" {
			elements, err := page.Timeout(2 * time.Second).Elements(".comment-item, .comment, .parent-comment")
			if err == nil && len(elements) > 0 {
				for i, el := range elements {
					userEl, err := el.Timeout(500 * time.Millisecond).Element(fmt.Sprintf(`[data-user-id="%s"]`, userID))
					if err == nil && userEl != nil {
						logrus.Infof("✓ 通过 userID 找到评论（第 %d 个元素，第 %d 轮）", i+1, round+1)
						return el, nil
					}
				}
			}
		}

		// 6. 滚动到最后一个评论触发懒加载
		elements, err := page.Timeout(2 * time.Second).Elements(".parent-comment, .comment-item, .comment")
		if err == nil && len(elements) > 0 {
			_ = elements[len(elements)-1].ScrollIntoView()
			time.Sleep(300 * time.Millisecond)
		}

		// 7. 继续向下滚动
		_, _ = page.Eval(`() => { window.scrollBy(0, window.innerHeight * 0.8); return true; }`)
		time.Sleep(scrollInterval)
	}

	return nil, fmt.Errorf("未找到评论 (commentID: %s, userID: %s)，已滚动 %d 轮，评论可能已被删除", commentID, userID, maxScrollRounds)
}

// findCommentElement 查找指定评论元素（不使用 API 预检，用于无拦截器场景）
func findCommentElement(page *rod.Page, commentID, userID string) (*rod.Element, error) {
	return findCommentElementWithAPICheck(page, commentID, userID, &[]commentAPIEntry{}, &sync.Mutex{})
}

// findSubComment 查找子评论元素。
// 流程：先用 findCommentElementWithAPICheck 定位父评论，展开"查看回复"按钮，
// 再等待子评论渲染后在子评论列表中查找目标 commentID。
func findSubComment(page *rod.Page, parentCommentID, commentID, userID string, apiEntries *[]commentAPIEntry, apiMu *sync.Mutex) (*rod.Element, error) {
	logrus.Infof("findSubComment: 查找父评论 parentID=%s, 目标子评论 commentID=%s", parentCommentID, commentID)

	// 1. 找到父评论
	parentEl, err := findCommentElementWithAPICheck(page, parentCommentID, "", apiEntries, apiMu)
	if err != nil {
		return nil, fmt.Errorf("找不到父评论 %s: %w", parentCommentID, err)
	}
	logrus.Infof("找到父评论 %s，准备展开子评论", parentCommentID)

	// 2. 展开子评论
	// 小红书的 DOM 结构：
	//   .list-container
	//     └── .parent-comment (包含评论和展开按钮)
	//           ├── .comment-item#comment-{parentID}  (评论内容)
	//           └── .reply-container > .show-more "展开 N 条回复"  (展开按钮)
	// 展开按钮在 .parent-comment 内部（作为子元素），通过 parentEl.parentElement.querySelector('.show-more') 找到。
	expandBtnResult, err := page.Eval(fmt.Sprintf(`() => {
		const parentEl = document.getElementById('comment-%s');
		if (!parentEl) return 'parent-not-found';
		const parentComment = parentEl.parentElement; // .parent-comment
		if (!parentComment) return 'parent-comment-not-found';
		const showMore = parentComment.querySelector('.show-more');
		if (!showMore) return 'no-show-more';
		showMore.scrollIntoView({behavior: 'smooth', block: 'center'});
		showMore.click();
		return showMore.textContent.trim();
	}`, parentCommentID))
	if err != nil {
		logrus.Warnf("展开子评论JS执行失败: %v，尝试继续", err)
	} else {
		expandResult := expandBtnResult.Value.String()
		if expandResult == "parent-not-found" || expandResult == "parent-comment-not-found" {
			logrus.Warnf("无法找到父评论元素: %s", expandResult)
		} else if expandResult == "no-show-more" {
			logrus.Info("父评论下无展开按钮，子评论可能已全部展示（或此时不需要展开）")
		} else {
			logrus.Infof("已点击展开按钮: %s，等待子评论渲染", expandResult)
			time.Sleep(2 * time.Second)
		}
	}

	// 3. 循环：尝试找到目标子评论，每轮之间点击"展开更多回复"
	// 小红书使用虚拟化渲染：DOM 中存在带 id 的占位元素，但内容为空，
	// 必须将元素 scrollIntoView 到视口后才会渲染真实内容。
	// 判断条件：getElementById 找到元素 AND textContent 非空。
	const maxExpandRounds = 30
	checkSubCommentVisible := func() (*rod.Element, bool) {
		result, err := page.Eval(fmt.Sprintf(`() => {
			const el = document.getElementById('comment-%s');
			if (!el) return null;
			// 滚动到视口触发渲染
			el.scrollIntoView({behavior: 'instant', block: 'center'});
			return el.textContent.trim() || null;
		}`, commentID))
		if err != nil || result == nil {
			return nil, false
		}
		val := result.Value.String()
		if val == "null" || val == "" || val == "undefined" {
			// 元素存在但内容为空（占位符），等待渲染
			return nil, false
		}
		// 内容非空，用 rod 拿到真实元素
		el, elErr := page.Timeout(2 * time.Second).Element(fmt.Sprintf("#comment-%s", commentID))
		if elErr != nil || el == nil {
			return nil, false
		}
		return el, true
	}

	for i := 0; i < maxExpandRounds; i++ {
		if el, found := checkSubCommentVisible(); found {
			logrus.Infof("✓ 第 %d 轮找到子评论 %s（内容已渲染）", i+1, commentID)
			return el, nil
		}

		// 查找"展开更多回复"按钮并点击
		moreResult, moreErr := page.Eval(fmt.Sprintf(`() => {
			const parentEl = document.getElementById('comment-%s');
			if (!parentEl) return 'parent-lost';
			const parentComment = parentEl.parentElement;
			if (!parentComment) return 'no-parent-comment';
			const showMore = parentComment.querySelector('.show-more');
			if (!showMore) return 'no-more';
			showMore.scrollIntoView({block:'center'});
			showMore.click();
			return showMore.textContent.trim();
		}`, parentCommentID))

		if moreErr != nil {
			logrus.Warnf("点击更多回复JS失败: %v", moreErr)
			break
		}
		moreText := moreResult.Value.String()
		if moreText == "no-more" || moreText == "no-parent-comment" || moreText == "parent-lost" {
			logrus.Infof("无更多按钮（%s），子评论已全部展开，共 %d 轮", moreText, i+1)
			break
		}
		logrus.Infof("第 %d 轮：点击了 '%s'", i+1, moreText)
		time.Sleep(1500 * time.Millisecond)
	}

	// 最终尝试
	if el, found := checkSubCommentVisible(); found {
		logrus.Infof("✓ 最终找到子评论 %s", commentID)
		return el, nil
	}

	// 确认评论不存在
	logrus.Warnf("在父评论 %s 下未找到子评论 %s（已展开全部回复）", parentCommentID, commentID)
	return parentEl, fmt.Errorf("子评论不存在（已被删除或不可见）: commentID=%s", commentID)
}
