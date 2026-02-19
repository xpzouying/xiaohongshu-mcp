package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// notificationsTimeout 是通知操作的内部超时时间。
// 整个流程（主页 warmup + 导航 + reload + 等待 API）约需 20-30s，
// 设为 90s 给足余量，避免被 mcporter 的短超时 context 提前取消。
const notificationsTimeout = 90 * time.Second

// NotificationUserInfo 通知中的用户信息
type NotificationUserInfo struct {
	UserID    string `json:"userid"`
	Nickname  string `json:"nickname"`
	Image     string `json:"image"`
	Indicator string `json:"indicator,omitempty"` // 例如 "作者"、"你的粉丝"
}

// NotificationCommentInfo 通知中的评论信息
type NotificationCommentInfo struct {
	ID      string `json:"id"`      // 评论 ID
	Content string `json:"content"` // 评论内容
	// 被回复的评论（type=comment/comment 时有）
	TargetComment *NotificationTargetComment `json:"target_comment,omitempty"`
}

// NotificationTargetComment 被回复的目标评论
type NotificationTargetComment struct {
	ID       string               `json:"id"`
	Content  string               `json:"content"`
	UserInfo NotificationUserInfo `json:"user_info"`
}

// NotificationItemInfo 通知关联的笔记信息
type NotificationItemInfo struct {
	ID        string               `json:"id"`         // 笔记 ID (feed_id)
	Content   string               `json:"content"`    // 笔记标题/摘要
	Image     string               `json:"image"`      // 封面图 URL
	XsecToken string               `json:"xsec_token"` // 访问令牌
	UserInfo  NotificationUserInfo `json:"user_info"`  // 笔记作者信息
}

// NotificationRelationType 描述通知中评论与当前用户的关系
type NotificationRelationType string

const (
	// RelationCommentOnMyNote 有人直接评论了你的笔记（顶级评论）
	RelationCommentOnMyNote NotificationRelationType = "comment_on_my_note"
	// RelationReplyToMyComment 有人在你的评论下直接回复了你（子评论，不含 @他人）
	RelationReplyToMyComment NotificationRelationType = "reply_to_my_comment"
	// RelationAtOthersUnderMyComment 有人在你的评论下 @了其他人（你被间接带到，非直接回复）
	RelationAtOthersUnderMyComment NotificationRelationType = "at_others_under_my_comment"
)

// Notification 单条通知
type Notification struct {
	// 通知 ID（用于去重和分页游标）
	ID string `json:"id"`
	// 通知类型（来自小红书 API）：
	//   "comment/item"    - 有人评论了你的笔记
	//   "comment/comment" - 有人在你的评论下留言
	Type string `json:"type"`
	// 通知标题（小红书原始文本，如"回复了你的评论"）
	Title string `json:"title"`
	// 发通知的用户
	UserInfo NotificationUserInfo `json:"user_info"`
	// 评论详情
	CommentInfo NotificationCommentInfo `json:"comment_info"`
	// 关联的笔记
	ItemInfo NotificationItemInfo `json:"item_info"`
	// Unix 时间戳（秒）
	Time int64 `json:"time"`

	// RelationType 描述该通知中评论与当前登录用户的关系，便于调用方判断如何处理：
	//   "comment_on_my_note"            - 有人直接评论了你的笔记（顶级评论）
	//   "reply_to_my_comment"           - 有人直接回复了你的评论（子评论）
	//   "at_others_under_my_comment"    - 有人在你的评论下 @了其他人（你被间接带到）
	RelationType NotificationRelationType `json:"relation_type"`

	// ParentCommentID 仅对 comment/comment 类型有效：
	// 被回复的那条评论的 ID（即 CommentInfo.TargetComment.ID）。
	// 调用 reply_comment_in_feed 回复子评论时需要传入，
	// 用于定位父评论并展开子评论列表。
	ParentCommentID string `json:"parent_comment_id,omitempty"`
}

// NotificationsResult 获取通知的结果
type NotificationsResult struct {
	Notifications []Notification `json:"notifications"`
	HasMore       bool           `json:"has_more"`
	// 下一页游标（传给 cursor 参数）
	NextCursor string `json:"next_cursor,omitempty"`
}

// mentionsAPIResponse 小红书 mentions API 的原始响应结构
type mentionsAPIResponse struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Data    struct {
		MessageList []mentionsMessage `json:"message_list"`
		HasMore     bool              `json:"has_more"`
		StrCursor   string            `json:"strCursor"`
		Cursor      int64             `json:"cursor"`
	} `json:"data"`
}

type mentionsMessage struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Time      int64  `json:"time"`
	Score     int64  `json:"score"`
	TimeFlag  int    `json:"time_flag"`
	Liked     bool   `json:"liked"`
	TrackType string `json:"track_type"`

	UserInfo struct {
		UserID    string `json:"userid"`
		Nickname  string `json:"nickname"`
		Image     string `json:"image"`
		Indicator string `json:"indicator,omitempty"`
		XsecToken string `json:"xsec_token,omitempty"`
	} `json:"user_info"`

	CommentInfo struct {
		ID        string `json:"id"`
		Content   string `json:"content"`
		Status    int    `json:"status"`
		Liked     bool   `json:"liked"`
		LikeCount int    `json:"like_count"`
		TargetComment *struct {
			ID       string `json:"id"`
			Content  string `json:"content"`
			UserInfo struct {
				UserID   string `json:"userid"`
				Nickname string `json:"nickname"`
				Image    string `json:"image"`
			} `json:"user_info"`
		} `json:"target_comment,omitempty"`
	} `json:"comment_info"`

	ItemInfo struct {
		ID        string `json:"id"`
		Content   string `json:"content"`
		Image     string `json:"image"`
		XsecToken string `json:"xsec_token"`
		Type      string `json:"type"`
		Status    int    `json:"status"`
		UserInfo  struct {
			UserID   string `json:"userid"`
			Nickname string `json:"nickname"`
			Image    string `json:"image"`
		} `json:"user_info"`
	} `json:"item_info"`
}

// NotificationsAction 获取通知的操作
type NotificationsAction struct {
	page *rod.Page
}

// NewNotificationsAction 创建通知操作实例
func NewNotificationsAction(page *rod.Page) *NotificationsAction {
	return &NotificationsAction{page: page}
}

// GetNotifications 获取通知列表（单页，最多 20 条）
// cursor 为空时获取最新通知，非空时获取下一页（通过滚动触发）
func (n *NotificationsAction) GetNotifications(ctx context.Context, cursor string, limit int) (*NotificationsResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}

	// 使用独立的内部 context，避免被 mcporter 的短超时 context 提前取消。
	// 整个通知流程（warmup + 导航 + reload + 等待 API）约需 20-30s，
	// 外部 ctx 仍作为父 context，若调用方主动取消则内部也会取消。
	innerCtx, cancel := context.WithTimeout(ctx, notificationsTimeout)
	defer cancel()

	// 收集所有拦截到的 API 响应（按 cursor 索引）
	type apiEntry struct {
		cursor string
		body   string
	}
	var mu sync.Mutex
	var apiEntries []apiEntry

	page := n.page.Context(innerCtx)

	router := page.HijackRequests()
	go router.Run()
	defer router.Stop()

	router.MustAdd("*/api/sns/web/v1/you/mentions*", func(ctx *rod.Hijack) {
		ctx.MustLoadResponse()
		reqURL := ctx.Request.URL()
		c := reqURL.Query().Get("cursor")
		body := ctx.Response.Body()
		mu.Lock()
		apiEntries = append(apiEntries, apiEntry{cursor: c, body: body})
		mu.Unlock()
	})

	// 先访问主页让 SPA 完全初始化，再强制 reload 通知页面。
	// 不能直接从主页 SPA 路由切换到 /notification（SPA 会复用内存中的旧数据不重新请求 API）；
	// 也不能用 about:blank 冷启动（SPA 未初始化，DOM stable 时 JS 还未执行，API 请求来不及发出）。
	// 正确做法：主页 warmup → 导航到通知页 → Reload 强制浏览器重新发起所有请求。
	logrus.Info("通知：主页 warmup...")
	page.MustNavigate("https://www.xiaohongshu.com/")
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	logrus.Info("通知：导航到通知页并强制 reload...")
	page.MustNavigate("https://www.xiaohongshu.com/notification")
	page.MustWaitDOMStable()
	// Reload 强制浏览器丢弃 SPA 内存缓存，重新发起 /api/sns/web/v1/you/mentions 请求
	page.MustReload()
	page.MustWaitDOMStable()

	// 等待第一页 API 响应
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		mu.Lock()
		count := len(apiEntries)
		mu.Unlock()
		if count > 0 {
			break
		}
		logrus.Infof("通知：等待 API 响应... (%ds)", i+1)
	}

	mu.Lock()
	firstEntries := make([]apiEntry, len(apiEntries))
	copy(firstEntries, apiEntries)
	mu.Unlock()

	if len(firstEntries) == 0 {
		return nil, fmt.Errorf("无法获取通知数据，请确认已登录")
	}

	// 找到最后一个有效响应（通常是第二个，第一个可能为空）
	var targetBody string
	for i := len(firstEntries) - 1; i >= 0; i-- {
		if firstEntries[i].body != "" && firstEntries[i].cursor == "" {
			targetBody = firstEntries[i].body
			break
		}
	}

	// 如果需要 cursor 分页，通过滚动触发
	if cursor != "" {
		logrus.Infof("通知：需要 cursor=%s 的页面，通过滚动触发...", cursor)
		mu.Lock()
		apiEntries = nil
		mu.Unlock()

		// 滚动到底部触发加载更多
		page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)

		// 等待带 cursor 的 API 响应
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			mu.Lock()
			var found string
			for _, e := range apiEntries {
				if e.cursor != "" && e.body != "" {
					found = e.body
				}
			}
			mu.Unlock()
			if found != "" {
				targetBody = found
				break
			}
			logrus.Infof("通知：等待 cursor 页 API 响应... (%ds)", i+1)
		}
	}

	if targetBody == "" {
		return nil, fmt.Errorf("未获取到有效的通知数据")
	}

	return parseNotificationsResponse(targetBody)
}

// GetNotificationsSince 获取指定时间之后的所有通知（自动翻页）
// sinceUnix 为 Unix 时间戳（秒），0 表示获取所有
func (n *NotificationsAction) GetNotificationsSince(ctx context.Context, sinceUnix int64) (*NotificationsResult, error) {
	var allNotifications []Notification
	var lastCursor string
	hasMore := true
	pageNum := 0

	// 使用独立的内部 context，避免被 mcporter 的短超时 context 提前取消。
	// since 模式可能需要翻多页，给更充裕的时间（最多 10 页 × 约 15s/页）。
	innerCtx, cancel := context.WithTimeout(ctx, notificationsTimeout)
	defer cancel()

	// 收集所有拦截到的 API 响应
	type apiEntry struct {
		cursor string
		body   string
	}
	var mu sync.Mutex
	var apiEntries []apiEntry

	page := n.page.Context(innerCtx)

	router := page.HijackRequests()
	go router.Run()
	defer router.Stop()

	router.MustAdd("*/api/sns/web/v1/you/mentions*", func(ctx *rod.Hijack) {
		ctx.MustLoadResponse()
		reqURL := ctx.Request.URL()
		c := reqURL.Query().Get("cursor")
		body := ctx.Response.Body()
		if body != "" {
			mu.Lock()
			apiEntries = append(apiEntries, apiEntry{cursor: c, body: body})
			mu.Unlock()
		}
	})

	// 主页 warmup → 导航通知页 → Reload（原因同 GetNotifications）
	logrus.Info("通知(since)：主页 warmup...")
	page.MustNavigate("https://www.xiaohongshu.com/")
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	logrus.Info("通知(since)：导航到通知页并强制 reload...")
	page.MustNavigate("https://www.xiaohongshu.com/notification")
	page.MustWaitDOMStable()
	page.MustReload()
	page.MustWaitDOMStable()

	// 等待第一页 API 响应
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		mu.Lock()
		count := len(apiEntries)
		mu.Unlock()
		if count > 0 {
			break
		}
		logrus.Infof("通知(since)：等待第一页 API 响应... (%ds)", i+1)
	}

	for hasMore && pageNum < 10 { // 最多翻 10 页（200 条）
		// 获取当前页的响应
		mu.Lock()
		var pageBody string
		for _, e := range apiEntries {
			if e.cursor == lastCursor && e.body != "" {
				pageBody = e.body
			}
		}
		mu.Unlock()

		if pageBody == "" {
			if pageNum == 0 {
				return nil, fmt.Errorf("无法获取通知数据，请确认已登录")
			}
			break
		}

		result, err := parseNotificationsResponse(pageBody)
		if err != nil {
			return nil, err
		}

		// 过滤时间范围，并检查是否需要继续翻页
		reachedOldData := false
		for _, n := range result.Notifications {
			if sinceUnix > 0 && n.Time < sinceUnix {
				reachedOldData = true
				break
			}
			allNotifications = append(allNotifications, n)
		}

		logrus.Infof("通知(since)：第 %d 页获取 %d 条，累计 %d 条，has_more=%v",
			pageNum+1, len(result.Notifications), len(allNotifications), result.HasMore)

		if reachedOldData || !result.HasMore {
			break
		}

		// 滚动触发下一页
		lastCursor = result.NextCursor
		mu.Lock()
		apiEntries = nil
		mu.Unlock()

		logrus.Infof("通知(since)：滚动加载下一页（cursor=%s）...", lastCursor)
		page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)

		// 等待下一页响应
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			mu.Lock()
			found := false
			for _, e := range apiEntries {
				if e.cursor != "" && e.body != "" {
					found = true
				}
			}
			mu.Unlock()
			if found {
				break
			}
			logrus.Infof("通知(since)：等待下一页 API 响应... (%ds)", i+1)
		}

		pageNum++
	}

	return &NotificationsResult{
		Notifications: allNotifications,
		HasMore:       false,
		NextCursor:    lastCursor,
	}, nil
}

// parseNotificationsResponse 解析 mentions API 响应
func parseNotificationsResponse(body string) (*NotificationsResult, error) {
	var apiResp mentionsAPIResponse
	if err := json.Unmarshal([]byte(body), &apiResp); err != nil {
		preview := body
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("解析通知 API 响应失败: %w\n原始响应: %s", err, preview)
	}

	if !apiResp.Success || apiResp.Code != 0 {
		return nil, fmt.Errorf("通知 API 返回错误: code=%d, msg=%s", apiResp.Code, apiResp.Msg)
	}

	notifications := make([]Notification, 0, len(apiResp.Data.MessageList))
	for _, msg := range apiResp.Data.MessageList {
		if msg.Type != "comment/item" && msg.Type != "comment/comment" {
			continue
		}

		notification := Notification{
			ID:    msg.ID,
			Type:  msg.Type,
			Title: msg.Title,
			Time:  msg.Time,
			UserInfo: NotificationUserInfo{
				UserID:    msg.UserInfo.UserID,
				Nickname:  msg.UserInfo.Nickname,
				Image:     msg.UserInfo.Image,
				Indicator: msg.UserInfo.Indicator,
			},
			CommentInfo: NotificationCommentInfo{
				ID:      msg.CommentInfo.ID,
				Content: msg.CommentInfo.Content,
			},
			ItemInfo: NotificationItemInfo{
				ID:        msg.ItemInfo.ID,
				Content:   msg.ItemInfo.Content,
				Image:     msg.ItemInfo.Image,
				XsecToken: msg.ItemInfo.XsecToken,
				UserInfo: NotificationUserInfo{
					UserID:   msg.ItemInfo.UserInfo.UserID,
					Nickname: msg.ItemInfo.UserInfo.Nickname,
					Image:    msg.ItemInfo.UserInfo.Image,
				},
			},
		}

		if msg.Type == "comment/comment" && msg.CommentInfo.TargetComment != nil {
			notification.CommentInfo.TargetComment = &NotificationTargetComment{
				ID:      msg.CommentInfo.TargetComment.ID,
				Content: msg.CommentInfo.TargetComment.Content,
				UserInfo: NotificationUserInfo{
					UserID:   msg.CommentInfo.TargetComment.UserInfo.UserID,
					Nickname: msg.CommentInfo.TargetComment.UserInfo.Nickname,
					Image:    msg.CommentInfo.TargetComment.UserInfo.Image,
				},
			}
			// ParentCommentID：被回复的评论即为父评论（用于 reply_comment_in_feed）
			notification.ParentCommentID = msg.CommentInfo.TargetComment.ID
		}

		// 设置 RelationType：客观描述评论与当前用户的关系
		switch msg.Type {
		case "comment/item":
			// 有人直接评论了你的笔记
			notification.RelationType = RelationCommentOnMyNote
		case "comment/comment":
			content := msg.CommentInfo.Content
			// 启发式判断：评论内容以 @ 开头 → 用户在你的评论下 @了其他人
			// 否则 → 用户直接回复了你的评论
			if len(content) > 0 && content[0] == '@' {
				notification.RelationType = RelationAtOthersUnderMyComment
			} else {
				notification.RelationType = RelationReplyToMyComment
			}
		}

		notifications = append(notifications, notification)
	}

	return &NotificationsResult{
		Notifications: notifications,
		HasMore:       apiResp.Data.HasMore,
		NextCursor:    apiResp.Data.StrCursor,
	}, nil
}
