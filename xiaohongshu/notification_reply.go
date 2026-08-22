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

// NotificationReplyResult 通知回复的结果。
type NotificationReplyResult struct {
	CommentID string `json:"comment_id"`
	Nickname  string `json:"nickname"`
	FeedID    string `json:"feed_id,omitempty"`
	Content   string `json:"content"`
}

// Reply 回复一条评论。
func (n *NotificationAction) Reply(ctx context.Context, commentID, content string) (*NotificationReplyResult, error) {
	if strings.TrimSpace(commentID) == "" {
		return nil, fmt.Errorf("缺少 comment_id")
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("回复内容不能为空")
	}

	page := n.page.Timeout(3 * time.Minute)

	page.MustNavigate("https://www.xiaohongshu.com/notification").MustWaitLoad()
	humanize.Delay(ctx, humanize.AfterNavigate)

	target, index, err := n.locate(ctx, page, commentID)
	if err != nil {
		return nil, err
	}

	items, err := page.Elements(`.tabs-content-container > .container`)
	if err != nil {
		return nil, fmt.Errorf("未找到通知条目: %w", err)
	}
	if index >= len(items) {
		return nil, fmt.Errorf("通知条目渲染数(%d)少于目标位置(%d)，页面可能未加载完", len(items), index)
	}
	item := items[index]

	humanize.Delay(ctx, humanize.Reading)

	replyBtn, err := item.Element(`.action-reply`)
	if err != nil {
		return nil, fmt.Errorf("该通知没有回复入口（评论可能已删除或不可回复）: %w", err)
	}

	humanize.Delay(ctx, humanize.BeforeClick)
	if err := humanize.Click(replyBtn); err != nil {
		return nil, fmt.Errorf("无法点击回复: %w", err)
	}
	humanize.Delay(ctx, humanize.AfterClick)

	input, err := item.Element(`textarea.comment-input`)
	if err != nil {
		return nil, fmt.Errorf("回复输入框未出现: %w", err)
	}

	if err := verifyReplyTarget(input, target.from().Nickname); err != nil {
		return nil, err
	}

	if err := humanize.Type(ctx, input, content); err != nil {
		return nil, fmt.Errorf("无法输入回复内容: %w", err)
	}
	humanize.Delay(ctx, humanize.AfterType)

	submit, err := item.Element(`button.submit`)
	if err != nil {
		return nil, fmt.Errorf("未找到发送按钮: %w", err)
	}

	humanize.Delay(ctx, humanize.BeforeSubmit)
	if err := humanize.Click(submit); err != nil {
		return nil, fmt.Errorf("无法点击发送: %w", err)
	}

	if err := waitReplyAccepted(item, 8*time.Second); err != nil {
		return nil, err
	}

	humanize.Delay(ctx, humanize.AfterInteract)

	logrus.Infof("通知回复成功: comment=%s", commentID)
	return &NotificationReplyResult{
		CommentID: commentID,
		Nickname:  target.from().Nickname,
		FeedID:    target.Item.ID,
		Content:   content,
	}, nil
}

// locate 找到目标评论在当前分区里的位置，必要时滚动加载。
func (n *NotificationAction) locate(ctx context.Context, page *rod.Page, commentID string) (*rawNotification, int, error) {
	const maxRounds = 20

	for range maxRounds {
		payload, err := n.readTab(page, TabMentions)
		if err != nil {
			return nil, 0, err
		}

		for i, r := range payload.MessageList {
			if r.Comment.ID != commentID {
				continue
			}
			if !r.visible() {
				return nil, 0, fmt.Errorf("该评论已删除或不可见，不能回复: %s", commentID)
			}
			return &r, i, nil
		}

		if !payload.HasMore {
			return nil, 0, fmt.Errorf("未找到评论 %s，它可能不在「评论和@」里或已被清理", commentID)
		}

		if err := page.Mouse.Scroll(0, 800, 5); err != nil {
			return nil, 0, fmt.Errorf("滚动查找失败: %w", err)
		}
		humanize.Delay(ctx, humanize.BetweenScroll)

		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
	}
	return nil, 0, fmt.Errorf("翻找 %d 轮仍未定位到评论 %s", maxRounds, commentID)
}

// verifyReplyTarget 用输入框的 placeholder 核对回复对象。
func verifyReplyTarget(input *rod.Element, nickname string) error {
	placeholder, err := input.Attribute("placeholder")
	if err != nil || placeholder == nil {
		return fmt.Errorf("读不到回复框提示文字，无法确认回复对象，已中止")
	}
	if nickname != "" && !strings.Contains(*placeholder, nickname) {
		return fmt.Errorf("回复对象不符：期望 %q，实际提示为 %q，已中止", nickname, *placeholder)
	}
	return nil
}

// waitReplyAccepted 等待回复提交完成。
func waitReplyAccepted(item *rod.Element, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		has, _, err := item.Has(`textarea.comment-input`)
		if err == nil && !has {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("回复未确认成功：发送后输入框仍未收起（可能被限制或发送失败）")
}
