package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// rawMentionUser 原始 API 返回的用户信息
type rawMentionUser struct {
	UserID   string `json:"userid"`
	Nickname string `json:"nickname"`
	Image    string `json:"image"`
}

// rawMentionItemInfo 原始 API 返回的笔记信息
type rawMentionItemInfo struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Link    string `json:"link"`
	Type    string `json:"type"`
}

// rawMentionTargetComment 原始 API 返回的被回复评论
type rawMentionTargetComment struct {
	ID        string           `json:"id"`
	Content   string           `json:"content"`
	UserInfo  rawMentionUser   `json:"userInfo"`
	LikeCount int              `json:"likeCount"`
}

// rawMentionCommentInfo 原始 API 返回的评论信息
type rawMentionCommentInfo struct {
	ID           string                    `json:"id"`
	Content      string                    `json:"content"`
	LikeCount    int                       `json:"likeCount"`
	TargetComment *rawMentionTargetComment `json:"targetComment"`
}

// rawMention 原始 API 返回的提及消息项
type rawMention struct {
	ID          string                  `json:"id"`
	Type        string                  `json:"type"`
	Title       string                  `json:"title"`
	Time        int64                   `json:"time"`
	UserInfo    rawMentionUser          `json:"userInfo"`
	ItemInfo    *rawMentionItemInfo     `json:"itemInfo"`
	CommentInfo *rawMentionCommentInfo  `json:"commentInfo"`
}

// MentionUser 提及消息中的用户信息
type MentionUser struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// MentionNoteInfo 提及关联的笔记信息
type MentionNoteInfo struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Link    string `json:"link"`
}

// MentionTargetComment 被回复的评论
type MentionTargetComment struct {
	ID        string       `json:"id"`
	Content   string       `json:"content"`
	User      MentionUser  `json:"user"`
	LikeCount int          `json:"likeCount"`
}

// MentionCommentInfo 提及中的评论信息
type MentionCommentInfo struct {
	ID            string               `json:"id"`
	Content       string               `json:"content"`
	LikeCount     int                  `json:"likeCount"`
	TargetComment *MentionTargetComment `json:"targetComment,omitempty"`
}

// Mention 提及消息（提取后的关键信息）
type Mention struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`        // comment/comment, comment/item, mention/comment
	Title       string            `json:"title"`       // 回复了你的评论、评论了你的笔记、在评论中@了你
	Time        int64             `json:"time"`       // Unix 时间戳
	User        MentionUser       `json:"user"`       // 触发者
	Note        *MentionNoteInfo  `json:"note,omitempty"`
	Comment     *MentionCommentInfo `json:"comment,omitempty"`
}

// MentionsAction 提及消息操作
type MentionsAction struct {
	page *rod.Page
}

// NewMentionsAction 创建提及消息操作
func NewMentionsAction(page *rod.Page) *MentionsAction {
	pp := page.Timeout(60 * time.Second)
	return &MentionsAction{page: pp}
}

// GetMentions 获取提及消息列表
func (m *MentionsAction) GetMentions(ctx context.Context, limit int) ([]Mention, error) {
	page := m.page.Context(ctx)

	if limit <= 0 {
		limit = 20
	}

	url := makeMentionsURL()
	page.MustNavigate(url)
	page.MustWaitStable()
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)
	time.Sleep(1 * time.Second)

	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
			window.__INITIAL_STATE__.notification &&
			window.__INITIAL_STATE__.notification.notificationMap &&
			window.__INITIAL_STATE__.notification.notificationMap.mentions) {
			const mentions = window.__INITIAL_STATE__.notification.notificationMap.mentions;
			const msgList = mentions.messageList;
			if (!msgList) return "";
			const listData = msgList.value !== undefined ? msgList.value :
				(msgList._value !== undefined ? msgList._value : msgList);
			if (listData) {
				const arr = Array.isArray(listData) ? listData : [];
				return JSON.stringify(arr);
			}
		}
		return "";
	}`).String()

	if result == "" {
		return nil, errors.ErrNoFeeds
	}

	var raw []rawMention
	if err := json.Unmarshal([]byte(result), &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mentions: %w", err)
	}

	mentions := make([]Mention, 0, len(raw))
	for _, r := range raw {
		m := rawToMention(r)
		mentions = append(mentions, m)
	}

	if len(mentions) > limit {
		mentions = mentions[:limit]
	}

	return mentions, nil
}

// rawToMention 将原始 API 数据转换为 Mention
func rawToMention(r rawMention) Mention {
	m := Mention{
		ID:    r.ID,
		Type:  r.Type,
		Title: r.Title,
		Time:  r.Time,
		User: MentionUser{
			UserID:   r.UserInfo.UserID,
			Nickname: r.UserInfo.Nickname,
			Avatar:   r.UserInfo.Image,
		},
	}

	if r.ItemInfo != nil {
		m.Note = &MentionNoteInfo{
			ID:      r.ItemInfo.ID,
			Content: r.ItemInfo.Content,
			Link:    r.ItemInfo.Link,
		}
	}

	if r.CommentInfo != nil {
		ci := &MentionCommentInfo{
			ID:        r.CommentInfo.ID,
			Content:   r.CommentInfo.Content,
			LikeCount: r.CommentInfo.LikeCount,
		}
		if r.CommentInfo.TargetComment != nil {
			tc := r.CommentInfo.TargetComment
			ci.TargetComment = &MentionTargetComment{
				ID:        tc.ID,
				Content:   tc.Content,
				LikeCount: tc.LikeCount,
				User: MentionUser{
					UserID:   tc.UserInfo.UserID,
					Nickname: tc.UserInfo.Nickname,
					Avatar:   tc.UserInfo.Image,
				},
			}
		}
		m.Comment = ci
	}

	return m
}

// makeMentionsURL 构造小红书消息通知(@人/提及)页 URL
func makeMentionsURL() string {
	return "https://www.xiaohongshu.com/notification"
}
