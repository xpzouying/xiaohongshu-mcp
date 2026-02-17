package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// NotificationAction 通知相关操作
type NotificationAction struct {
	page *rod.Page
}

func NewNotificationAction(page *rod.Page) *NotificationAction {
	pp := page.Timeout(60 * time.Second)
	pp.MustNavigate("https://www.xiaohongshu.com/explore")
	pp.MustWaitDOMStable()
	time.Sleep(2 * time.Second)
	return &NotificationAction{page: pp}
}

// signedFetch 通过浏览器JS上下文发起带签名的API请求
func (n *NotificationAction) signedFetch(ctx context.Context, apiPath string) (string, error) {
	page := n.page.Context(ctx)

	js := fmt.Sprintf(`() => {
		return new Promise(async (resolve) => {
			try {
				const url = "%s";
				let headers = {"Content-Type": "application/json"};
				if (window._webmsxyw) {
					const sign = window._webmsxyw(url, "");
					if (sign) {
						headers["x-s"] = sign["X-s"];
						headers["x-t"] = sign["X-t"].toString();
					}
				}
				const resp = await fetch("https://edith.xiaohongshu.com" + url, {
					credentials: "include",
					headers: headers
				});
				const text = await resp.text();
				resolve(text);
			} catch(e) {
				resolve(JSON.stringify({"error": e.message}));
			}
		});
	}`, apiPath)

	result, err := page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("eval failed: %w", err)
	}
	return result.Value.String(), nil
}

// NotificationResponse API通用响应
type NotificationResponse struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

// MentionsResponse 评论和@通知响应
type MentionsResponse struct {
	NotificationResponse
	Data struct {
		HasMore   bool              `json:"has_more"`
		Cursor    int64             `json:"cursor"`
		StrCursor string            `json:"strCursor"`
		Messages  []MentionMessage  `json:"message_list"`
	} `json:"data"`
}

type MentionMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Time      int64           `json:"time"`
	UserInfo  NotifUserInfo   `json:"user_info"`
	ItemInfo  *NotifItemInfo  `json:"item_info,omitempty"`
	SubType   string          `json:"sub_type,omitempty"`
	Content   string          `json:"content,omitempty"`
}

// LikesResponse 赞和收藏通知响应
type LikesResponse struct {
	NotificationResponse
	Data struct {
		HasMore   bool            `json:"has_more"`
		Cursor    int64           `json:"cursor"`
		StrCursor string          `json:"strCursor"`
		Messages  []LikeMessage   `json:"message_list"`
	} `json:"data"`
}

type LikeMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Time      int64           `json:"time"`
	Score     int64           `json:"score"`
	TrackType string          `json:"track_type"`
	Title     string          `json:"title"`
	UserInfo  NotifUserInfo   `json:"user_info"`
	ItemInfo  *NotifItemInfo  `json:"item_info,omitempty"`
}

// ConnectionsResponse 新增关注通知响应
type ConnectionsResponse struct {
	NotificationResponse
	Data struct {
		HasMore   bool                  `json:"has_more"`
		Cursor    int64                 `json:"cursor"`
		StrCursor string                `json:"strCursor"`
		Messages  []ConnectionMessage   `json:"message_list"`
	} `json:"data"`
}

type ConnectionMessage struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Time     int64         `json:"time"`
	UserInfo NotifUserInfo `json:"user_info"`
}

type NotifUserInfo struct {
	UserID   string `json:"userid"`
	Nickname string `json:"nickname"`
	Image    string `json:"image"`
	FStatus  string `json:"fstatus,omitempty"`
}

type NotifItemInfo struct {
	Content string `json:"content,omitempty"`
	Image   string `json:"image,omitempty"`
	Link    string `json:"link,omitempty"`
}

// GetMentions 获取评论和@通知
func (n *NotificationAction) GetMentions(ctx context.Context, num int, cursor string) (*MentionsResponse, error) {
	if num <= 0 {
		num = 20
	}
	path := fmt.Sprintf("/api/sns/web/v1/you/mentions?num=%d", num)
	if cursor != "" {
		path += "&cursor=" + cursor
	}

	raw, err := n.signedFetch(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp MentionsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal mentions failed: %w, raw: %s", err, raw[:min(len(raw), 200)])
	}
	if !resp.Success && resp.Code != 0 {
		if resp.Code == -1 {
			return &resp, nil
		}
		return nil, fmt.Errorf("mentions API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return &resp, nil
}

// GetLikes 获取赞和收藏通知
func (n *NotificationAction) GetLikes(ctx context.Context, num int, cursor string) (*LikesResponse, error) {
	if num <= 0 {
		num = 20
	}
	path := fmt.Sprintf("/api/sns/web/v1/you/likes?num=%d", num)
	if cursor != "" {
		path += "&cursor=" + cursor
	}

	raw, err := n.signedFetch(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp LikesResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal likes failed: %w, raw: %s", err, raw[:min(len(raw), 200)])
	}
	if !resp.Success && resp.Code != 0 {
		if resp.Code == -1 {
			return &resp, nil
		}
		return nil, fmt.Errorf("likes API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return &resp, nil
}

// GetConnections 获取新增关注通知
func (n *NotificationAction) GetConnections(ctx context.Context, num int, cursor string) (*ConnectionsResponse, error) {
	if num <= 0 {
		num = 20
	}
	path := fmt.Sprintf("/api/sns/web/v1/you/connections?num=%d", num)
	if cursor != "" {
		path += "&cursor=" + cursor
	}

	raw, err := n.signedFetch(ctx, path)
	if err != nil {
		return nil, err
	}

	var resp ConnectionsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal connections failed: %w, raw: %s", err, raw[:min(len(raw), 200)])
	}
	if !resp.Success && resp.Code != 0 {
		// code=-1 with empty msg may mean no data, return empty result
		if resp.Code == -1 {
			return &resp, nil
		}
		return nil, fmt.Errorf("connections API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return &resp, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
