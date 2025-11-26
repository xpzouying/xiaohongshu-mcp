package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// FeedDetailAction 表示 Feed 详情页动作
type FeedDetailAction struct {
	page *rod.Page
}

// NewFeedDetailAction 创建 Feed 详情页动作
func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// GetFeedDetail 获取 Feed 详情页数据
func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string, loadAllComments bool) (*FeedDetailResponse, error) {
	page := f.page.Context(ctx).Timeout(5 * time.Minute)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	// 检测页面是否不可访问
	if err := checkPageAccessible(page); err != nil {
		return nil, err
	}

	// 加载全部评论
	if loadAllComments {
		if err := f.loadAllComments(page); err != nil {
			logrus.Warnf("加载全部评论失败: %v", err)
		}
	}

	// 提取笔记详情数据
	return f.extractFeedDetail(page, feedID)
}

// checkPageAccessible 检查页面是否可访问
func checkPageAccessible(page *rod.Page) error {
	unavailableResult := page.MustEval(`() => {
		const wrapper = document.querySelector('.access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper');
		if (!wrapper) return null;

		const text = wrapper.textContent || '';
		const keywords = [
			'当前笔记暂时无法浏览',
			'该内容因违规已被删除',
			'该笔记已被删除',
			'内容不存在',
			'笔记不存在',
			'已失效',
			'私密笔记',
			'仅作者可见',
			'因用户设置，你无法查看',
			'因违规无法查看'
		];

		for (const kw of keywords) {
			if (text.includes(kw)) {
				return kw.trim();
			}
		}
		return null;
	}`)

	rawJSON, err := unavailableResult.MarshalJSON()
	if err != nil {
		logrus.Errorf("无法解析页面状态检查的结果: %v", err)
		return fmt.Errorf("无法解析页面状态检查的结果: %w", err)
	}

	if string(rawJSON) != "null" {
		var reason string
		if err := json.Unmarshal(rawJSON, &reason); err == nil {
			logrus.Warnf("笔记不可访问: %s", reason)
			return fmt.Errorf("笔记不可访问: %s", reason)
		}
		rawReason := string(rawJSON)
		logrus.Warnf("笔记不可访问，且无法解析原因: %s", rawReason)
		return fmt.Errorf("笔记不可访问，无法解析原因: %s", rawReason)
	}

	return nil
}

// loadAllComments 加载所有评论
func (f *FeedDetailAction) loadAllComments(page *rod.Page) error {
	const (
		maxAttempts          = 500
		scrollInterval       = 600 * time.Millisecond
		clickMoreInterval    = 1  // 每次滚动都检查"更多"按钮
		stagnantLimit        = 20 // 增加停滞容忍度
		noScrollChangeLimit  = 15 // 增加滚动停滞容忍度
		minScrollDelta       = 10 // 最小有效滚动距离
		aggressiveClickEvery = 5  // 每5次尝试进行一次激进点击
	)

	logrus.Info("开始加载所有评论...")

	// 先滚动到评论区
	scrollToCommentsArea(page)
	time.Sleep(1 * time.Second)

	var (
		lastCount           = 0
		lastScrollTop       = 0
		stagnantChecks      = 0
		noScrollChangeCount = 0
		totalClickedButtons = 0
		attempt             = 0
	)

	for attempt = 0; attempt < maxAttempts; attempt++ {
		logrus.Debugf("=== 尝试 %d/%d ===", attempt+1, maxAttempts)

		// === 1. 检查是否到达底部 ===
		if checkEndContainer(page) {
			logrus.Infof("✓ 检测到 'THE END' 元素，已滑动到底部")
			// 到底部后再做最后一轮点击
			finalClicked := clickShowMoreButtons(page)
			totalClickedButtons += finalClicked
			if finalClicked > 0 {
				logrus.Infof("底部最后点击了 %d 个按钮", finalClicked)
				time.Sleep(1 * time.Second)
			}

			currentCount := getCommentCount(page)
			logrus.Infof("✓ 加载完成: %d 条评论, 尝试次数: %d, 点击按钮: %d",
				currentCount, attempt+1, totalClickedButtons)
			return nil
		}

		// === 2. 每次都点击"更多"按钮 ===
		if attempt%clickMoreInterval == 0 {
			clicked := clickShowMoreButtons(page)
			if clicked > 0 {
				totalClickedButtons += clicked
				logrus.Infof("点击了 %d 个'更多'按钮，累计: %d", clicked, totalClickedButtons)
				time.Sleep(500 * time.Millisecond)

				// 多轮检查
				for round := 0; round < 2; round++ {
					time.Sleep(300 * time.Millisecond)
					clicked2 := clickShowMoreButtons(page)
					if clicked2 > 0 {
						totalClickedButtons += clicked2
						logrus.Infof("第 %d 轮再次点击了 %d 个按钮", round+2, clicked2)
						time.Sleep(500 * time.Millisecond)
					} else {
						break
					}
				}
			}
		}

		// === 4. 获取当前评论数量 ===
		currentCount := getCommentCount(page)
		totalCount := getTotalCommentCount(page)

		logrus.Debugf("当前评论: %d, 目标: %d", currentCount, totalCount)

		// 检查是否已加载所有评论（但继续滚动到底部确认）
		if totalCount > 0 && currentCount >= totalCount {
			logrus.Infof("评论数量已达标: %d/%d，继续滚动到底部确认...", currentCount, totalCount)
			// 不要立即返回，继续滚动到底部
		}

		// === 5. 检查评论数量变化 ===
		if currentCount != lastCount {
			logrus.Infof("✓ 评论数量增加: %d -> %d (+%d)", lastCount, currentCount, currentCount-lastCount)
			lastCount = currentCount
			stagnantChecks = 0 // 重置停滞计数
		} else {
			stagnantChecks++
			if stagnantChecks%5 == 0 {
				logrus.Debugf("评论数量停滞 %d 次", stagnantChecks)
			}
		}

		// 只有在严重停滞时才考虑退出
		if stagnantChecks >= stagnantLimit {
			logrus.Infof("评论数量长期停滞，尝试最后冲刺...")
			// 最后冲刺：大幅滚动 + 点击
			finalPush(page)
			finalClicked := clickShowMoreButtons(page)
			totalClickedButtons += finalClicked

			if checkEndContainer(page) {
				logrus.Infof("✓ 最终到达底部，评论数: %d, 点击按钮: %d",
					currentCount, totalClickedButtons)
				return nil
			}

			// 还没到底部，继续
			logrus.Infof("未到底部，重置停滞计数，继续加载...")
			stagnantChecks = 0
		}

		// === 6. 执行滚动 ===
		_, scrollDelta, currentScrollTop := scrollWithMouse(page)

		// === 7. 检查滚动变化 ===
		if scrollDelta < minScrollDelta || currentScrollTop == lastScrollTop {
			noScrollChangeCount++
			if noScrollChangeCount%5 == 0 {
				logrus.Debugf("滚动停滞 %d 次，尝试大幅滚动", noScrollChangeCount)
				// 尝试更大幅度滚动
				largeScroll(page)
				time.Sleep(300 * time.Millisecond)
			}
		} else {
			noScrollChangeCount = 0
			lastScrollTop = currentScrollTop
		}

		// 只有严重滚动停滞时才考虑结束
		if noScrollChangeCount >= noScrollChangeLimit {
			logrus.Infof("滚动严重停滞，尝试最后冲刺...")
			finalPush(page)

			if checkEndContainer(page) {
				currentCount := getCommentCount(page)
				logrus.Infof("✓ 最终到达底部，评论数: %d, 点击按钮: %d",
					currentCount, totalClickedButtons)
				return nil
			}

			// 重置计数继续
			logrus.Infof("未到底部，重置滚动计数，继续加载...")
			noScrollChangeCount = 0
			lastScrollTop = 0
		}

		// === 8. 等待内容加载 ===
		time.Sleep(scrollInterval)
	}

	// === 9. 达到最大尝试次数，做最后的冲刺 ===
	logrus.Infof("达到最大尝试次数 %d，执行最后冲刺...", maxAttempts)
	finalPush(page)
	finalClicked := clickShowMoreButtons(page)
	totalClickedButtons += finalClicked

	currentCount := getCommentCount(page)
	hasEnd := checkEndContainer(page)

	logrus.Infof("✓ 加载结束: %d 条评论, 总点击按钮: %d, 到达底部: %v",
		currentCount, totalClickedButtons, hasEnd)

	return nil
}

// scrollToCommentsArea 滚动到评论区
func scrollToCommentsArea(page *rod.Page) {
	logrus.Info("滚动到评论区...")
	page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (container) {
			container.scrollIntoView({behavior: 'smooth', block: 'start'});
		}
	}`)
}

// finalPush 最后冲刺：大幅滚动到底部
func finalPush(page *rod.Page) {
	logrus.Info("执行最后冲刺滚动...")

	for i := 0; i < 20; i++ {
		// 检查是否已经到底部
		if checkEndContainer(page) {
			logrus.Debug("已到底部，停止冲刺")
			return
		}

		beforeTop := getScrollTop(page)

		// 大幅滚动
		largeScroll(page)
		time.Sleep(200 * time.Millisecond)

		// 点击出现的按钮
		clicked := clickShowMoreButtons(page)
		if clicked > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		afterTop := getScrollTop(page)

		// 如果滚动没变化，尝试JS滚动
		if afterTop == beforeTop {
			page.MustEval(`() => {
				window.scrollTo(0, document.body.scrollHeight);
			}`)
			time.Sleep(300 * time.Millisecond)
		}
	}
}

// largeScroll 大幅度滚动
func largeScroll(page *rod.Page) {
	// 方法1: Mouse.Scroll 大幅度滚动
	page.Mouse.Scroll(0, 2000, 5)
	time.Sleep(100 * time.Millisecond)
}

// scrollWithMouse 使用 Mouse 模拟滚轮滚动
func scrollWithMouse(page *rod.Page) (bool, int, int) {
	beforeTop := getScrollTop(page)

	// 获取视口高度
	viewportHeight := page.MustEval(`() => window.innerHeight`).Int()

	// 计算滚动距离（每次滚动视口高度的 80%）
	scrollDelta := float64(viewportHeight) * 0.8
	if scrollDelta < 500 {
		scrollDelta = 500
	}

	// 使用 Mouse.Scroll 模拟滚轮滚动
	err := page.Mouse.Scroll(0, scrollDelta, 5)
	if err != nil {
		logrus.Warnf("鼠标滚动失败: %v", err)
		return false, 0, beforeTop
	}

	// 等待滚动完成
	time.Sleep(150 * time.Millisecond)

	afterTop := getScrollTop(page)
	actualDelta := afterTop - beforeTop
	scrolled := actualDelta > 5

	if scrolled {
		logrus.Debugf("滚动: %d -> %d (Δ%d)", beforeTop, afterTop, actualDelta)
	}

	return scrolled, actualDelta, afterTop
}

// getScrollTop 获取当前滚动位置
func getScrollTop(page *rod.Page) int {
	result := page.MustEval(`() => {
		return window.pageYOffset || document.documentElement.scrollTop || document.body.scrollTop || 0;
	}`)
	return result.Int()
}

// clickShowMoreButtons 点击所有可见的"更多"按钮
func clickShowMoreButtons(page *rod.Page) int {
	elements, err := page.Elements(".show-more")
	if err != nil {
		return 0
	}

	clickedCount := 0

	for _, el := range elements {
		// 检查元素是否可见
		visible, err := el.Visible()
		if err != nil || !visible {
			continue
		}

		// 检查是否在 DOM 中
		box, err := el.Shape()
		if err != nil || len(box.Quads) == 0 {
			continue
		}

		// 点击元素
		if err := el.Click(proto.InputMouseButtonLeft, 1); err == nil {
			clickedCount++
			time.Sleep(150 * time.Millisecond)
		}
	}

	return clickedCount
}

// getCommentCount 获取当前评论数量
func getCommentCount(page *rod.Page) int {
	result := page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (!container) return 0;
		return container.querySelectorAll('.comment-item, .comment-item-sub, .comment').length;
	}`)
	return result.Int()
}

// getTotalCommentCount 获取总评论数
func getTotalCommentCount(page *rod.Page) int {
	result := page.MustEval(`() => {
		const container = document.querySelector('.comments-container');
		if (!container) return 0;
		
		const totalEl = container.querySelector('.total');
		if (!totalEl) return 0;
		
		const text = (totalEl.textContent || '').replace(/\s+/g, '');
		const match = text.match(/共(\d+)条评论/);
		return match ? parseInt(match[1], 10) : 0;
	}`)
	return result.Int()
}

// checkEndContainer 检查是否出现 "THE END" 元素
func checkEndContainer(page *rod.Page) bool {
	result := page.MustEval(`() => {
		const endContainer = document.querySelector('.end-container');
		if (!endContainer) return false;
		
		const text = (endContainer.textContent || '').trim().toUpperCase();
		return text.includes('THE END') || text.includes('THEEND');
	}`)
	return result.Bool()
}

// extractFeedDetail 提取 Feed 详情数据
func (f *FeedDetailAction) extractFeedDetail(page *rod.Page, feedID string) (*FeedDetailResponse, error) {
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.note &&
		    window.__INITIAL_STATE__.note.noteDetailMap) {
			const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
			return JSON.stringify(noteDetailMap);
		}
		return "";
	}`).String()

	if result == "" {
		return nil, errors.ErrNoFeedDetail
	}

	var noteDetailMap map[string]struct {
		Note     FeedDetail  `json:"note"`
		Comments CommentList `json:"comments"`
	}

	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal noteDetailMap: %w", err)
	}

	noteDetail, exists := noteDetailMap[feedID]
	if !exists {
		return nil, fmt.Errorf("feed %s not found in noteDetailMap", feedID)
	}

	return &FeedDetailResponse{
		Note:     noteDetail.Note,
		Comments: noteDetail.Comments,
	}, nil
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}
