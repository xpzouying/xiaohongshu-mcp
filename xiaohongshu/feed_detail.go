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

	var domCommentsPayload string
	if loadAllComments {
		scrollToEndJS := `() => {
			const END_SELECTOR = '.end-container';
			const DELTA_MIN = 520;
			const MAX_ATTEMPTS = 60;
			const WAIT_AFTER_SCROLL = 420;

			const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
			const scrollRoot = document.scrollingElement || document.documentElement || document.body;

			const reachedEnd = () => {
				const endEl = document.querySelector(END_SELECTOR);
				if (!endEl) return false;
				const text = (endEl.textContent || '').toUpperCase();
				if (text.includes('THE END')) return true;
				const rect = endEl.getBoundingClientRect();
				return rect.top >= 0 && rect.top <= (window.innerHeight || document.documentElement.clientHeight || 0);
			};

			const collectCandidates = () => {
				const container = document.querySelector('.comments-container');
				const set = new Set();

				const push = (node) => {
					if (node && node instanceof HTMLElement) {
						set.add(node);
					}
				};

				push(document.body);
				push(document.documentElement);
				push(scrollRoot);

				if (container) {
					let current = container;
					while (current) {
						push(current);
						if (current === document.body || current === document.documentElement) {
							break;
						}
						current = current.parentElement;
					}
					container.querySelectorAll('.comments-el, .list-container, [data-v-4a19279a][name="list"]').forEach(push);
				}

				const ranked = Array.from(set).map((node) => {
					const style = window.getComputedStyle(node);
					const scrollable = node.scrollHeight - node.clientHeight > 40;
					const hasScroll = /auto|scroll|overlay/i.test(style.overflowY || '');
					const weight =
						(node === scrollRoot ? 800 : 0) +
						(container && node === container ? 1200 : 0) +
						(container && node.contains && node.contains(container) ? 600 : 0) +
						(hasScroll ? 300 : 0) +
						(scrollable ? 300 : 0) -
						(node === document.body || node === document.documentElement ? 80 : 0);
					return { node, weight };
				}).sort((a, b) => b.weight - a.weight);

				return ranked.slice(0, 8).map((item) => item.node);
			};

			const metrics = (el) => {
				if (!el || el === document || el === window) {
					const root = scrollRoot;
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
				if (el === document.body || el === document.documentElement || el === scrollRoot || el === document || el === window) {
					scrollRoot.scrollTop = value;
				} else {
					el.scrollTop = value;
				}
			};

			const dispatchWheel = (el, delta) => {
				if (!el) return;
				try {
					el.dispatchEvent(new Event('scroll', { bubbles: true }));
					if (typeof WheelEvent === 'function' && delta !== 0) {
						const wheel = new WheelEvent('wheel', { deltaY: delta, bubbles: true, cancelable: true });
						el.dispatchEvent(wheel);
					}
				} catch (err) {
					console.debug('dispatchWheel error', err);
				}
			};

			const waitForMove = (el, beforeTop) => {
				let tries = 0;
				return new Promise((resolve) => {
					const tick = () => {
						tries++;
						const now = metrics(el).top;
						if (Math.abs(now - beforeTop) >= 6 || tries >= 6) {
							resolve(Math.abs(now - beforeTop) >= 6);
							return;
						}
						setTimeout(tick, 60);
					};
					setTimeout(tick, 60);
				});
			};

			const scrollOnce = async (node) => {
				const before = metrics(node);
				const delta = Math.max(before.client * 0.85, DELTA_MIN);
				const desired = before.max > 0 ? Math.min(before.top + delta, before.max) : before.top + delta;
				const applied = Math.max(0, desired - before.top);
				setScrollTop(node, desired);
				dispatchWheel(node, applied);
				const moved = await waitForMove(node, before.top);
				if (!moved && node !== scrollRoot) {
					const rootBefore = metrics(scrollRoot).top;
					setScrollTop(scrollRoot, rootBefore + applied);
					dispatchWheel(scrollRoot, applied);
					return waitForMove(scrollRoot, rootBefore);
				}
				return moved;
			};

			return (async () => {
				for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt++) {
					const candidates = collectCandidates();
					for (const node of candidates) {
						const moved = await scrollOnce(node);
						if (moved) {
							await sleep(WAIT_AFTER_SCROLL);
							break;
						}
					}
					if (reachedEnd()) {
						return JSON.stringify({ status: 'end', attempts: attempt + 1 });
					}
				}
				return JSON.stringify({ status: 'timeout' });
			})().catch((err) => JSON.stringify({ status: 'error', message: err && err.message ? err.message : String(err) }));
		}`

		if res, err := page.Eval(scrollToEndJS); err != nil {
			logrus.Warnf("加载全部评论失败: %v", err)
		} else if res != nil {
			logrus.Infof("评论滚动结果: %v", res.Value)
		}

		collectCommentsJS := `() => {
			try {
				const container = document.querySelector('.comments-container');
				if (!container) {
					return JSON.stringify({ list: [], reachedEnd: false, error: 'comments container not found' });
				}

				const items = Array.from(container.querySelectorAll('.comment-item'));
				const seen = new Set();
				const list = [];

				const textContent = (node) => (node && node.textContent ? node.textContent.trim() : '');

				for (const item of items) {
					let rawId = item.getAttribute('id') || '';
					if (!rawId && item.dataset) {
						rawId = item.dataset.commentId || item.dataset.id || '';
					}
					const commentId = rawId.replace(/^comment-/, '') || rawId;
					if (!commentId || seen.has(commentId)) {
						continue;
					}
					seen.add(commentId);

					const contentEl = item.querySelector('.comment-content, .content, .content-text, .text, .word');
					const nicknameEl = item.querySelector('.user-name, .nickname, .name, .author-name, .title');
					const userNode = item.querySelector('[data-user-id]');
					const likeEl = item.querySelector('.like .count, .interaction .like span, .interaction-bar .like span, [class*="like"] span');

					list.push({
						id: commentId,
						content: textContent(contentEl),
						nickname: textContent(nicknameEl),
						userId: userNode ? (userNode.getAttribute('data-user-id') || '') : '',
						likeCount: textContent(likeEl),
					});
				}

				const endEl = document.querySelector('.end-container');
				const reachedEnd = !!(endEl && (endEl.textContent || '').toUpperCase().includes('THE END'));
				return JSON.stringify({ list, reachedEnd });
			} catch (err) {
				return JSON.stringify({ list: [], reachedEnd: false, error: err && err.message ? err.message : String(err) });
			}
		}`

		if res, err := page.Eval(collectCommentsJS); err != nil {
			logrus.Warnf("收集评论失败: %v", err)
		} else if res != nil {
			domCommentsPayload = res.Value.Str()
		}
	}

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

	if loadAllComments && domCommentsPayload != "" {
		var payload struct {
			List []struct {
				ID        string `json:"id"`
				Content   string `json:"content"`
				Nickname  string `json:"nickname"`
				UserID    string `json:"userId"`
				LikeCount string `json:"likeCount"`
			}
			ReachedEnd bool   `json:"reachedEnd"`
			Error      string `json:"error"`
		}

		if err := json.Unmarshal([]byte(domCommentsPayload), &payload); err != nil {
			logrus.Warnf("解析 DOM 评论数据失败: %v", err)
		} else if payload.Error != "" {
			logrus.Warnf("DOM 评论数据返回错误: %s", payload.Error)
		} else if len(payload.List) > 0 {
			comments := make([]Comment, 0, len(payload.List))
			for _, item := range payload.List {
				comments = append(comments, Comment{
					ID:        item.ID,
					NoteID:    feedID,
					Content:   item.Content,
					LikeCount: item.LikeCount,
					UserInfo: User{
						UserID:   item.UserID,
						Nickname: item.Nickname,
						NickName: item.Nickname,
					},
					SubComments:     nil,
					SubCommentCount: "0",
				})
			}

			noteDetail.Comments.List = comments
			noteDetail.Comments.Cursor = ""
			noteDetail.Comments.HasMore = !payload.ReachedEnd
		}
	}

	return &FeedDetailResponse{
		Note:     noteDetail.Note,
		Comments: noteDetail.Comments,
	}, nil
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}
