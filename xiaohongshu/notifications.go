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

// Notification 单条通知
type Notification struct {
	// 通知 ID（用于去重和分页游标）
	ID string `json:"id"`
	// 通知类型：
	//   "comment/item"    - 有人评论了你的笔记
	//   "comment/comment" - 有人回复了你的评论
	Type string `json:"type"`
	// 通知标题（中文描述，如"回复了你的评论"）
	Title string `json:"title"`
	// 发通知的用户
	UserInfo NotificationUserInfo `json:"user_info"`
	// 评论详情
	CommentInfo NotificationCommentInfo `json:"comment_info"`
	// 关联的笔记
	ItemInfo NotificationItemInfo `json:"item_info"`
	// Unix 时间戳（秒）
	Time int64 `json:"time"`
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

	// 收集所有拦截到的 API 响应（按 cursor 索引）
	type apiEntry struct {
		cursor string
		body   string
	}
	var mu sync.Mutex
	var apiEntries []apiEntry

	page := n.page.Context(ctx)

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

	// 先访问主页初始化 SPA
	logrus.Info("通知：先访问小红书主页初始化 SPA...")
	page.MustNavigate("https://www.xiaohongshu.com/")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 访问通知页面（触发第一页请求）
	logrus.Info("通知：访问通知页面...")
	page.MustNavigate("https://www.xiaohongshu.com/notification")
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

	// 收集所有拦截到的 API 响应
	type apiEntry struct {
		cursor string
		body   string
	}
	var mu sync.Mutex
	var apiEntries []apiEntry

	page := n.page.Context(ctx)

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

	// 先访问主页初始化 SPA
	logrus.Info("通知(since)：先访问小红书主页初始化 SPA...")
	page.MustNavigate("https://www.xiaohongshu.com/")
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 访问通知页面
	logrus.Info("通知(since)：访问通知页面...")
	page.MustNavigate("https://www.xiaohongshu.com/notification")
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
		}

		notifications = append(notifications, notification)
	}

	return &NotificationsResult{
		Notifications: notifications,
		HasMore:       apiResp.Data.HasMore,
		NextCursor:    apiResp.Data.StrCursor,
	}, nil
}
