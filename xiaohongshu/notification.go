package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// NotificationTab 通知页的三个分区。
type NotificationTab string

const (
	TabMentions    NotificationTab = "mentions"    // 评论和@
	TabLikes       NotificationTab = "likes"       // 赞和收藏
	TabConnections NotificationTab = "connections" // 新增关注
)

// tabLabels 分区对应的页面标签文字，用于切换分区时定位。
var tabLabels = map[NotificationTab]string{
	TabMentions:    "评论和@",
	TabLikes:       "赞和收藏",
	TabConnections: "新增关注",
}

// ParseNotificationTab 解析分区名，空值默认为「评论和@」。
func ParseNotificationTab(s string) (NotificationTab, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "", string(TabMentions):
		return TabMentions, nil
	case string(TabLikes):
		return TabLikes, nil
	case string(TabConnections):
		return TabConnections, nil
	}
	return "", fmt.Errorf("未知的通知分区 %q，可选：mentions / likes / connections", s)
}

// statusNormal 是内容状态里表示「正常可见」的取值，未知取值按不可见处理。
const statusNormal = "NORMAL"

// NotificationCount 三个分区各自的未读数。
type NotificationCount struct {
	Mentions    int `json:"mentions"`
	Likes       int `json:"likes"`
	Connections int `json:"connections"`
	Unread      int `json:"unread"`
}

// NotificationUser 通知里涉及的用户。
type NotificationUser struct {
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	XsecToken string `json:"xsec_token,omitempty"`
}

// NotificationItem 一条通知。
//
// FeedID / FeedXsecToken 取自笔记信息，与 get_feed_detail、reply_comment_in_feed
// 所需的参数一致，调用方可以直接拿去读原帖或走笔记页回复。
type NotificationItem struct {
	ID            string           `json:"id"`
	Type          string           `json:"type"`
	Title         string           `json:"title"`
	Time          int64            `json:"time"`
	From          NotificationUser `json:"from"`
	CommentID     string           `json:"comment_id,omitempty"`
	CommentText   string           `json:"comment_text,omitempty"`
	Liked         bool             `json:"liked"`
	FeedID        string           `json:"feed_id,omitempty"`
	FeedXsecToken string           `json:"feed_xsec_token,omitempty"`
	FeedTitle     string           `json:"feed_title,omitempty"`
}

// NotificationList 一个分区的通知列表。
type NotificationList struct {
	Tab NotificationTab `json:"tab"`
	// Filtered 是被过滤掉的条目数：评论已删除、或笔记/评论处于非正常状态。
	// 单独计数是为了让调用方知道「列表比实际条目少」，而不是以为一条不少。
	Filtered int                `json:"filtered"`
	Items    []NotificationItem `json:"items"`
}

// NotificationAction 通知中心相关动作。
type NotificationAction struct {
	page *rod.Page
}

func NewNotificationAction(page *rod.Page) *NotificationAction {
	return &NotificationAction{page: page}
}

// UnreadCount 读取三个分区的未读数。
func (n *NotificationAction) UnreadCount(ctx context.Context) (*NotificationCount, error) {
	page := n.page.Timeout(60 * time.Second)

	page.MustNavigate("https://www.xiaohongshu.com/explore").MustWaitLoad()
	humanize.Delay(ctx, humanize.AfterNavigate)

	if err := page.WaitStable(time.Second); err != nil {
		logrus.Warnf("explore 页未稳定，继续读取未读数: %v", err)
	}

	res, err := page.Eval(`() => {
		const s = window.__INITIAL_STATE__;
		const c = s && s.notification && s.notification.notificationCount;
		return c ? JSON.stringify(c) : "";
	}`)
	if err != nil {
		return nil, fmt.Errorf("读取未读数失败: %w", err)
	}

	raw := res.Value.String()
	if raw == "" {
		return nil, fmt.Errorf("页面状态里没有未读数，可能未登录或页面结构已变化")
	}

	var count rawCount
	if err := json.Unmarshal([]byte(raw), &count); err != nil {
		return nil, fmt.Errorf("解析未读数失败: %w", err)
	}
	return &NotificationCount{
		Mentions:    count.Mentions,
		Likes:       count.Likes,
		Connections: count.Connections,
		Unread:      count.Unread,
	}, nil
}

// rawCount 是未读数在页面状态里的原始结构。
type rawCount struct {
	Mentions    int `json:"mentions"`
	Likes       int `json:"likes"`
	Connections int `json:"connections"`
	Unread      int `json:"unreadCount"`
}

// List 读取指定分区的通知列表。
func (n *NotificationAction) List(ctx context.Context, tab NotificationTab, limit int) (*NotificationList, error) {
	if limit <= 0 {
		limit = 20
	}

	page := n.page.Timeout(3 * time.Minute)

	page.MustNavigate("https://www.xiaohongshu.com/notification").MustWaitLoad()
	humanize.Delay(ctx, humanize.AfterNavigate)

	if err := n.switchTab(ctx, page, tab); err != nil {
		return nil, err
	}

	if err := n.loadUntil(ctx, page, tab, limit); err != nil {
		return nil, err
	}

	payload, err := n.readTab(page, tab)
	if err != nil {
		return nil, err
	}

	items, filtered := convertNotifications(payload.MessageList, limit)
	return &NotificationList{Tab: tab, Filtered: filtered, Items: items}, nil
}

// switchTab 切到目标分区。「评论和@」是默认分区，无需点击。
func (n *NotificationAction) switchTab(ctx context.Context, page *rod.Page, tab NotificationTab) error {
	if tab == TabMentions {
		return nil
	}

	label := tabLabels[tab]
	elems, err := page.Elements(`.reds-tab-item`)
	if err != nil {
		return fmt.Errorf("未找到通知分区标签: %w", err)
	}

	for _, elem := range elems {
		text, err := elem.Text()
		if err != nil || strings.TrimSpace(text) != label {
			continue
		}
		humanize.Delay(ctx, humanize.BeforeClick)
		if err := humanize.Click(elem); err != nil {
			return fmt.Errorf("切换到分区 %s 失败: %w", label, err)
		}
		humanize.Delay(ctx, humanize.AfterClick)
		return nil
	}
	return fmt.Errorf("未找到分区标签 %q", label)
}

// loadUntil 滚动加载，直到条目数够用或没有更多。
func (n *NotificationAction) loadUntil(ctx context.Context, page *rod.Page, tab NotificationTab, limit int) error {
	const maxRounds = 20

	for range maxRounds {
		payload, err := n.readTab(page, tab)
		if err != nil {
			return err
		}
		loaded := len(payload.MessageList)
		if loaded >= limit || !payload.HasMore {
			return nil
		}

		if err := page.Mouse.Scroll(0, 800, 5); err != nil {
			return fmt.Errorf("滚动加载失败: %w", err)
		}
		humanize.Delay(ctx, humanize.BetweenScroll)

		if after, err := n.readTab(page, tab); err == nil && len(after.MessageList) == loaded {
			humanize.Delay(ctx, humanize.Reading)
		}

		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

// 页面状态里通知列表的原始结构。字段名与状态一致，解析交给 encoding/json。
type (
	notificationPayload struct {
		HasMore     bool              `json:"hasMore"`
		MessageList []rawNotification `json:"messageList"`
	}

	rawNotification struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Title string `json:"title"`
		Time  int64  `json:"time"`

		UserInfo rawUser `json:"userInfo"`
		User     rawUser `json:"user"`

		Comment rawComment `json:"commentInfo"`
		Item    rawItem    `json:"itemInfo"`
	}

	rawUser struct {
		UserID    string `json:"userid"`
		Nickname  string `json:"nickname"`
		XsecToken string `json:"xsecToken"`
	}

	rawIllegal struct {
		Status string `json:"illegalStatus"`
	}

	rawComment struct {
		ID      string     `json:"id"`
		Content string     `json:"content"`
		Liked   bool       `json:"liked"`
		Illegal rawIllegal `json:"illegalInfo"`
	}

	rawItem struct {
		ID        string     `json:"id"`
		Type      string     `json:"type"`
		Content   string     `json:"content"`
		XsecToken string     `json:"xsecToken"`
		Illegal   rawIllegal `json:"illegalInfo"`
	}
)

// itemTypeNote 标记关联对象是一篇笔记；其他类型的 id 与笔记 id 不通用。
const itemTypeNote = "note_info"

// from 返回发起这条通知的用户。
func (r rawNotification) from() rawUser {
	if r.UserInfo.UserID != "" || r.UserInfo.Nickname != "" {
		return r.UserInfo
	}
	return r.User
}

// readTab 读取指定分区的原始数据，字段映射由结构体承担。
func (n *NotificationAction) readTab(page *rod.Page, tab NotificationTab) (*notificationPayload, error) {
	res, err := page.Eval(`(tab) => {
		const s = window.__INITIAL_STATE__;
		const m = s && s.notification && s.notification.notificationMap;
		return m && m[tab] ? JSON.stringify(m[tab]) : "";
	}`, string(tab))
	if err != nil {
		return nil, fmt.Errorf("读取通知列表失败: %w", err)
	}

	raw := res.Value.String()
	if raw == "" {
		return nil, fmt.Errorf("页面状态里没有分区 %s，可能未登录或页面结构已变化", tab)
	}

	var payload notificationPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("解析通知列表失败: %w", err)
	}
	return &payload, nil
}

// visible 判断一条通知的内容是否仍然正常可见。
func (r rawNotification) visible() bool {
	if r.Comment.ID != "" && r.Comment.Illegal.Status != "" && r.Comment.Illegal.Status != statusNormal {
		return false
	}
	if r.Item.ID != "" && r.Item.Illegal.Status != "" && r.Item.Illegal.Status != statusNormal {
		return false
	}
	return true
}

// convertNotifications 过滤掉不可见条目并转成对外结构，返回条目和被过滤的数量。
func convertNotifications(raw []rawNotification, limit int) ([]NotificationItem, int) {
	items := make([]NotificationItem, 0, len(raw))
	filtered := 0

	for _, r := range raw {
		if len(items) >= limit {
			break
		}
		if !r.visible() {
			filtered++
			continue
		}

		u := r.from()
		item := NotificationItem{
			ID:    r.ID,
			Type:  r.Type,
			Title: r.Title,
			Time:  r.Time,
			From: NotificationUser{
				UserID:    u.UserID,
				Nickname:  u.Nickname,
				XsecToken: u.XsecToken,
			},
			CommentID:   r.Comment.ID,
			CommentText: r.Comment.Content,
			Liked:       r.Comment.Liked,
			FeedTitle:   r.Item.Content,
		}
		if r.Item.Type == itemTypeNote {
			item.FeedID = r.Item.ID
			item.FeedXsecToken = r.Item.XsecToken
		}
		items = append(items, item)
	}
	return items, filtered
}
