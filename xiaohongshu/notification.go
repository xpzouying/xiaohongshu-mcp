package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// NotificationAction 通知操作
type NotificationAction struct {
	page *rod.Page
}

// NewNotificationAction 创建通知操作实例
func NewNotificationAction(page *rod.Page) *NotificationAction {
	return &NotificationAction{page: page}
}

// GetNotifications 获取通知列表
func (n *NotificationAction) GetNotifications(ctx context.Context) (*NotificationList, error) {
	page := n.page.Context(ctx).Timeout(60 * time.Second)

	// 导航到通知页面
	notificationURL := "https://www.xiaohongshu.com/notification"
	logrus.Infof("导航到通知页面: %s", notificationURL)

	page.MustNavigate(notificationURL)
	page.MustWaitDOMStable()

	// 等待页面加载
	time.Sleep(2 * time.Second)

	// 点击通知 tab 触发异步数据加载
	n.clickNotificationTabs(page)

	// 通知数据是异步加载的，等待 notificationMap 中有数据
	result, err := n.waitAndExtractNotifications(page)
	if err == nil && len(result.Notifications) > 0 {
		logrus.Infof("获取到 %d 条通知", len(result.Notifications))
		return result, nil
	}
	if err != nil {
		logrus.Infof("从 __INITIAL_STATE__ 提取通知失败: %v", err)
	}

	// fallback 到 DOM 解析
	logrus.Info("尝试 DOM 解析通知")
	return n.extractFromDOM(page)
}

// waitAndExtractNotifications 等待异步通知数据加载完成并提取
func (n *NotificationAction) waitAndExtractNotifications(page *rod.Page) (*NotificationList, error) {
	// 等待 __INITIAL_STATE__ 存在
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 轮询 __INITIAL_STATE__ 中的数据，等待 tab 点击触发的异步加载完成
	for i := 0; i < 10; i++ {
		hasData := page.MustEval(`() => {
			const state = window.__INITIAL_STATE__;
			if (!state.notification || !state.notification.notificationMap) return false;
			const nm = state.notification.notificationMap;
			for (const key of Object.keys(nm)) {
				const val = nm[key];
				const inner = val._value !== undefined ? val._value : (val.value !== undefined ? val.value : val);
				if (inner && inner.messageList && inner.messageList.length > 0) {
					return true;
				}
			}
			return false;
		}`).Bool()

		if hasData {
			logrus.Infof("通知数据已加载（第 %d 次检查）", i+1)
			break
		}
		time.Sleep(1 * time.Second)
	}

	// 提取所有类别的通知数据
	stateJSON := page.MustEval(`() => {
		const state = window.__INITIAL_STATE__;
		if (!state.notification || !state.notification.notificationMap) {
			return JSON.stringify({ notifications: [], hasMore: false });
		}

		const nm = state.notification.notificationMap;
		const allNotifications = [];

		for (const category of Object.keys(nm)) {
			const val = nm[category];
			const inner = val._value !== undefined ? val._value : (val.value !== undefined ? val.value : val);
			if (inner && inner.messageList && Array.isArray(inner.messageList)) {
				for (const msg of inner.messageList) {
					allNotifications.push({
						category: category,
						raw: msg
					});
				}
			}
		}

		return JSON.stringify({
			notifications: allNotifications,
			hasMore: false
		});
	}`).String()

	if stateJSON == "" {
		return nil, fmt.Errorf("提取通知数据为空")
	}

	var parsed struct {
		Notifications []struct {
			Category string                 `json:"category"`
			Raw      map[string]interface{} `json:"raw"`
		} `json:"notifications"`
		HasMore bool `json:"hasMore"`
	}

	if err := json.Unmarshal([]byte(stateJSON), &parsed); err != nil {
		return nil, fmt.Errorf("解析通知数据失败: %w", err)
	}

	logrus.Infof("解析到 %d 条原始通知", len(parsed.Notifications))

	// 转换为 Notification 结构
	notifications := make([]Notification, 0, len(parsed.Notifications))
	for _, item := range parsed.Notifications {
		notification := n.convertRawToNotification(item.Raw)
		if notification != nil {
			// 如果类型映射结果为空，根据 category 设置类型
			if notification.Type == "" {
				notification.Type = n.mapCategoryToType(item.Category)
			}
			notifications = append(notifications, *notification)
		}
	}

	return &NotificationList{
		Notifications: notifications,
		HasMore:       parsed.HasMore,
	}, nil
}

// convertRawToNotification 将原始数据转换为 Notification 结构
func (n *NotificationAction) convertRawToNotification(raw map[string]interface{}) *Notification {
	notification := &Notification{}

	// 提取 ID
	if id, ok := raw["id"].(string); ok {
		notification.ID = id
	}

	// 提取类型（如 liked/item, faved/item 等）
	if typeStr, ok := raw["type"].(string); ok {
		notification.Type = n.mapNotificationType(typeStr)
	}

	// 提取标题作为内容（如 "赞了你的笔记"）
	if title, ok := raw["title"].(string); ok {
		notification.Content = title
	}

	// 提取时间戳（秒级）
	if t, ok := raw["time"].(float64); ok {
		notification.CreateTime = int64(t)
	}

	// 提取用户信息（通知发起人）
	if userInfo, ok := raw["userInfo"].(map[string]interface{}); ok {
		notification.FromUser = NotificationUser{
			UserID:   getStringFromMap(userInfo, "userid"),
			Nickname: getStringFromMap(userInfo, "nickname"),
			Avatar:   getStringFromMap(userInfo, "image"),
		}
	}

	// 提取笔记信息（被操作的笔记）
	if itemInfo, ok := raw["itemInfo"].(map[string]interface{}); ok {
		notification.TargetNote = &NotificationNote{
			NoteID:    getStringFromMap(itemInfo, "id"),
			XsecToken: getStringFromMap(itemInfo, "xsecToken"),
			Title:     getStringFromMap(itemInfo, "content"),
			Cover:     getStringFromMap(itemInfo, "image"),
		}
	}

	return notification
}

// mapNotificationType 映射通知类型
func (n *NotificationAction) mapNotificationType(typeStr string) NotificationType {
	switch typeStr {
	case "liked/item", "like", "liked":
		return NotificationTypeLike
	case "faved/item", "favorite", "collect", "collected":
		return NotificationTypeFavorite
	case "comment", "reply", "comment/item", "reply/item":
		return NotificationTypeComment
	case "follow", "followed", "follow/item":
		return NotificationTypeFollow
	case "mention", "at", "mention/item":
		return NotificationTypeMention
	default:
		return NotificationType(typeStr)
	}
}

// mapCategoryToType 将通知分类映射为通知类型
func (n *NotificationAction) mapCategoryToType(category string) NotificationType {
	switch category {
	case "likes":
		return NotificationTypeLike
	case "mentions":
		return NotificationTypeComment
	case "connections":
		return NotificationTypeFollow
	default:
		return NotificationType(category)
	}
}

// clickNotificationTabs 依次点击通知页面的各个 tab 触发异步数据加载
func (n *NotificationAction) clickNotificationTabs(page *rod.Page) {
	// 用 JS 在页面中查找并点击各个 tab
	tabTexts := []string{"赞和收藏", "评论和@", "新增关注"}

	for _, text := range tabTexts {
		clicked := page.MustEval(`(targetText) => {
			const allEls = document.querySelectorAll('*');
			let best = null;
			let bestChildCount = Infinity;
			for (const el of allEls) {
				const tag = el.tagName.toUpperCase();
				if (tag === 'HTML' || tag === 'BODY' || tag === 'SCRIPT' || tag === 'STYLE') continue;
				if (el.textContent && el.textContent.trim() === targetText) {
					const cc = el.children.length;
					if (cc < bestChildCount) {
						bestChildCount = cc;
						best = el;
					}
				}
			}
			if (best) {
				best.click();
				return best.tagName + '.' + best.className;
			}
			return '';
		}`, text).String()

		if clicked != "" {
			logrus.Infof("点击 tab: %s -> %s", text, clicked)
			time.Sleep(2 * time.Second)
		} else {
			logrus.Infof("未找到 tab: %s", text)
		}
	}
}

// extractFromDOM 从 DOM 元素解析通知数据（fallback 方案）
func (n *NotificationAction) extractFromDOM(page *rod.Page) (*NotificationList, error) {
	// 等待通知列表容器出现
	_, err := page.Timeout(5 * time.Second).Element(".notification-list, .message-list, .notice-list")
	if err != nil {
		logrus.Warnf("未找到通知列表容器: %v", err)
		return &NotificationList{Notifications: []Notification{}}, nil
	}

	// 获取所有通知项
	elements, err := page.Elements(".notification-item, .message-item, .notice-item")
	if err != nil || len(elements) == 0 {
		logrus.Info("未找到通知项元素")
		return &NotificationList{Notifications: []Notification{}}, nil
	}

	notifications := make([]Notification, 0, len(elements))
	for i, el := range elements {
		notification, err := n.parseNotificationElement(el, i)
		if err != nil {
			logrus.Debugf("解析通知项 %d 失败: %v", i, err)
			continue
		}
		notifications = append(notifications, *notification)
	}

	logrus.Infof("从 DOM 解析到 %d 条通知", len(notifications))

	return &NotificationList{
		Notifications: notifications,
		HasMore:       false,
	}, nil
}

// parseNotificationElement 解析单个通知 DOM 元素
func (n *NotificationAction) parseNotificationElement(el *rod.Element, index int) (*Notification, error) {
	notification := &Notification{
		ID: fmt.Sprintf("dom-%d", index),
	}

	// 提取用户头像和昵称
	if avatar, err := el.Element(".avatar img, .user-avatar img"); err == nil {
		if src, err := avatar.Attribute("src"); err == nil && src != nil {
			notification.FromUser.Avatar = *src
		}
	}

	if nickname, err := el.Element(".nickname, .user-name"); err == nil {
		if text, err := nickname.Text(); err == nil {
			notification.FromUser.Nickname = text
		}
	}

	// 提取通知内容
	if content, err := el.Element(".content, .message-content, .notice-content"); err == nil {
		if text, err := content.Text(); err == nil {
			notification.Content = text
		}
	}

	// 提取笔记封面（如有）
	if cover, err := el.Element(".note-cover img, .cover img"); err == nil {
		if src, err := cover.Attribute("src"); err == nil && src != nil {
			notification.TargetNote = &NotificationNote{
				Cover: *src,
			}
		}
	}

	return notification, nil
}

// getStringFromMap 从 map 中安全获取字符串
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
