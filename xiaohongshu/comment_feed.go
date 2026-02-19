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
func (f *CommentFeedAction) ReplyToComment(ctx context.Context, feedID, xsecToken, commentID, userID, content string) error {
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
	commentEl, err := findCommentElementWithAPICheck(page, commentID, userID, &commentAPIEntries, &commentAPIMu)
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
			logrus.Infof("✓ 预检直接找到评论: %s", commentID)
			return el, nil
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

		// 1. 先尝试在 DOM 中查找目标评论
		if commentID != "" {
			selector := fmt.Sprintf("#comment-%s", commentID)
			el, err := page.Timeout(2 * time.Second).Element(selector)
			if err == nil && el != nil {
				logrus.Infof("✓ 找到评论: %s（第 %d 轮）", commentID, round+1)
				return el, nil
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
