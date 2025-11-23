package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
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

	// === 检测「笔记暂时无法浏览」或类似不可访问页面 ===
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
			'因违规无法查看',
			'这是一片荒地点击评论'
		];

		for (const kw of keywords) {
			if (text.includes(kw)) {
				return kw.trim();
			}
		}
		return null;
	}`)

	// The result is a gson.JSON object. We need to get its raw JSON representation to check for "null".
	rawJSON, err := unavailableResult.MarshalJSON()
	if err != nil {
		logrus.Errorf("无法解析页面状态检查的结果: %v", err)
		return nil, fmt.Errorf("无法解析页面状态检查的结果: %w", err)
	}

	if string(rawJSON) != "null" {
		var reason string
		// JS 返回的字符串会被 JSON 编码，所以需要 Unmarshal
		if err := json.Unmarshal(rawJSON, &reason); err == nil {
			logrus.Warnf("笔记不可访问: %s", reason)
			return nil, fmt.Errorf("笔记不可访问: %s", reason)
		} else {
			// 如果解析失败，直接使用原始值
			rawReason := string(rawJSON)
			logrus.Warnf("笔记不可访问，且无法解析原因: %s", rawReason)
			return nil, fmt.Errorf("笔记不可访问，无法解析原因: %s", rawReason)
		}
	}

	// === 加载全部评论（简化版本）===
	if loadAllComments {
		scrollAllCommentsJS := `() => {
		const INTERVAL_MS = 900;
		const STAGNANT_LIMIT = 8;
		const NO_CHANGE_SCROLL_LIMIT = 3;
		const DELTA_MIN = 480;
		const SCROLL_TIMEOUT = 900;
		const MAX_ATTEMPTS = 200;
		const CLICK_MORE_INTERVAL = 2; // 每滚动2次检查一次"更多"按钮
		const CLICK_WAIT_TIME = 300; // 点击后等待时间

		const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
		const scrollRoot = () => document.scrollingElement || document.documentElement || document.body;
		const getContainer = () => document.querySelector('.comments-container');
		const getCommentCount = (container) =>
			container ? container.querySelectorAll('.comment-item, .comment-item-sub, .comment').length : 0;
		const getTotalCount = (container) => {
			if (!container) return null;
			const text = (container.querySelector('.total')?.textContent || '').replace(/\s+/g, '');
			const match = text.match(/共(\d+)条评论/);
			return match ? parseInt(match[1], 10) : null;
		};
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
		
		// 简化的点击"更多"按钮函数 - 只使用 .show-more 选择器
		const clickShowMoreButtons = () => {
			let clickedCount = 0;
			
			const elements = document.querySelectorAll('.show-more');
			
			elements.forEach((el) => {
				try {
					// 检查元素是否可见
					const rect = el.getBoundingClientRect();
					const style = window.getComputedStyle(el);
					const isVisible = (
						rect.height > 0 &&
						rect.width > 0 &&
						style.display !== 'none' &&
						style.visibility !== 'hidden' &&
						style.opacity !== '0' &&
						rect.top < window.innerHeight + 500 && // 允许元素在视口下方500px内
						rect.bottom > -500 // 允许元素在视口上方500px内
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

		let cachedTarget = null;
		const collectCandidates = () => {
			const container = getContainer();
			const candidatesSet = new Set();
			if (container) {
				let current = container;
				while (current) {
					if (current instanceof HTMLElement) {
						candidatesSet.add(current);
					}
					current = current.parentElement;
				}
				container.querySelectorAll('*').forEach((node) => {
					if (node instanceof HTMLElement) {
						candidatesSet.add(node);
					}
				});
			}
			[document.body, document.documentElement].forEach((node) => {
				if (node instanceof HTMLElement) {
					candidatesSet.add(node);
				}
			});
			const candidates = [];
			candidatesSet.forEach((node) => {
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
					candidates.push({ node, weight });
				}
			});
			candidates.sort((a, b) => b.weight - a.weight);
			return candidates.map((candidate) => candidate.node);
		};
		const findScrollTarget = () => {
			if (cachedTarget && cachedTarget.isConnected) {
				return cachedTarget;
			}
			const candidates = collectCandidates();
			cachedTarget = candidates.find((node) => {
				const metrics = getScrollMetrics(node);
				return metrics.max > 30 || metrics.client > 0;
			}) || scrollRoot();
			return cachedTarget;
		};
		const performScroll = (target) => {
			const scrollTarget = target || findScrollTarget();
			if (!scrollTarget) {
				window.scrollBy(0, window.innerHeight * 0.8);
				return;
			}
			const metrics = getScrollMetrics(scrollTarget);
			const beforeTop = metrics.top;
			const desired = metrics.max > 0 ? Math.min(metrics.top + Math.max(metrics.client * 0.85, DELTA_MIN), metrics.max) : metrics.top + Math.max(metrics.client * 0.85, DELTA_MIN);
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
		
		return (async () => {
			let lastCount = 0;
			let stagnantChecks = 0;
			let noScrollChangeCount = 0;
			let totalClickedButtons = 0;
			
			for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt++) {
				const container = getContainer();
				if (!container) {
					await sleep(300);
					continue;
				}
				
				// 每隔一定次数检查并点击"更多"按钮
				if (attempt % CLICK_MORE_INTERVAL === 0) {
					const clicked = clickShowMoreButtons();
					if (clicked > 0) {
						totalClickedButtons += clicked;
						console.log('点击了 ' + clicked + ' 个"更多"按钮，累计: ' + totalClickedButtons);
						await sleep(CLICK_WAIT_TIME); // 等待内容展开
						
						// 点击后再次检查是否有新的"更多"按钮出现
						await sleep(200);
						const clicked2 = clickShowMoreButtons();
						if (clicked2 > 0) {
							totalClickedButtons += clicked2;
							console.log('二次检查点击了 ' + clicked2 + ' 个"更多"按钮');
							await sleep(CLICK_WAIT_TIME);
						}
					}
				}
				
				const total = getTotalCount(container);
				const count = getCommentCount(container);
				if (total && count >= total) {
					return { 
						status: 'complete', 
						reason: 'total', 
						attempts: attempt + 1, 
						count, 
						total,
						clickedButtons: totalClickedButtons
					};
				}
				if (count === lastCount) {
					stagnantChecks += 1;
				} else {
					lastCount = count;
					stagnantChecks = 0;
				}
				if (stagnantChecks >= STAGNANT_LIMIT) {
					return { 
						status: 'complete', 
						reason: 'stagnant', 
						attempts: attempt + 1, 
						count, 
						total,
						clickedButtons: totalClickedButtons
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
				}
				if (noScrollChangeCount >= NO_CHANGE_SCROLL_LIMIT) {
					return { 
						status: 'complete', 
						reason: 'no-scroll-change', 
						attempts: attempt + 1, 
						count, 
						total,
						clickedButtons: totalClickedButtons
					};
				}
				if (INTERVAL_MS > SCROLL_TIMEOUT) {
					await sleep(INTERVAL_MS - SCROLL_TIMEOUT);
				}
			}
			return { 
				status: 'timeout',
				clickedButtons: totalClickedButtons
			};
		})()
			.then((res) => JSON.stringify(res))
			.catch((err) => JSON.stringify({ status: 'error', message: err && err.message ? err.message : String(err) }));
	}`

		if res, err := page.Eval(scrollAllCommentsJS); err != nil {
			logrus.Warnf("加载全部评论失败: %v", err)
		} else if res != nil {
			if str := res.Value.Str(); str != "" {
				logrus.Infof("评论滚动结果: %s", str)
			}
		}
	}

	// === 提取笔记详情数据 ===
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
