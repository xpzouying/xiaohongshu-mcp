package xiaohongshu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// NotificationLikeResult 通知点赞的结果。
type NotificationLikeResult struct {
	CommentID string `json:"comment_id"`
	Nickname  string `json:"nickname"`
	Liked     bool   `json:"liked"`
	// Skipped 表示调用前已是目标状态，本次未操作。
	Skipped bool `json:"skipped"`
}

// likeSettleTimeout 是等待点赞状态生效的上限。
const likeSettleTimeout = 15 * time.Second

// Like 给一条评论点赞或取消点赞；已是目标状态时直接返回。
func (n *NotificationAction) Like(ctx context.Context, commentID string, unlike bool) (*NotificationLikeResult, error) {
	if strings.TrimSpace(commentID) == "" {
		return nil, fmt.Errorf("缺少 comment_id")
	}

	want := !unlike
	page := n.page.Timeout(3 * time.Minute)

	page.MustNavigate("https://www.xiaohongshu.com/notification").MustWaitLoad()
	humanize.Delay(ctx, humanize.AfterNavigate)

	target, index, err := n.locate(ctx, page, commentID)
	if err != nil {
		return nil, err
	}

	result := &NotificationLikeResult{
		CommentID: commentID,
		Nickname:  target.from().Nickname,
		Liked:     want,
	}

	if target.Comment.Liked == want {
		result.Skipped = true
		logrus.Infof("通知点赞跳过（已是目标状态）: comment=%s liked=%v", commentID, want)
		return result, nil
	}

	items, err := page.Elements(`.tabs-content-container > .container`)
	if err != nil {
		return nil, fmt.Errorf("未找到通知条目: %w", err)
	}
	if index >= len(items) {
		return nil, fmt.Errorf("通知条目渲染数(%d)少于目标位置(%d)，页面可能未加载完", len(items), index)
	}

	humanize.Delay(ctx, humanize.Reading)

	btn, err := items[index].Element(`.action-like .like-wrapper`)
	if err != nil {
		return nil, fmt.Errorf("该通知没有点赞入口（评论可能已删除或不可点赞）: %w", err)
	}

	humanize.Delay(ctx, humanize.BeforeClick)
	if err := humanize.Click(btn); err != nil {
		return nil, fmt.Errorf("无法点击点赞: %w", err)
	}

	if err := n.waitLikeSettled(page, commentID, want); err != nil {
		return nil, err
	}

	humanize.Delay(ctx, humanize.AfterInteract)

	logrus.Infof("通知点赞成功: comment=%s liked=%v", commentID, want)
	return result, nil
}

// waitLikeSettled 等待点赞状态变成目标值；判不出来直接报错，不自动重试。
func (n *NotificationAction) waitLikeSettled(page *rod.Page, commentID string, want bool) error {
	if n.likedMatches(page, commentID, want, likeSettleTimeout) {
		return nil
	}
	return fmt.Errorf("点赞未确认成功：%s 内状态未变成 %v。点赞可能已生效但同步慢，"+
		"请先用 list_notifications 读一次当前状态再决定是否重试", likeSettleTimeout, want)
}

// likedMatches 轮询状态，直到目标评论的 liked 等于 want 或超时。
func (n *NotificationAction) likedMatches(page *rod.Page, commentID string, want bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		payload, err := n.readTab(page, TabMentions)
		if err == nil {
			for _, r := range payload.MessageList {
				if r.Comment.ID == commentID {
					if r.Comment.Liked == want {
						return true
					}
					break
				}
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	return false
}
