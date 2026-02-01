package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

// WebhookPayload webhook 发送的数据结构
type WebhookPayload struct {
	PublishedNote interface{} `json:"published_note"` // 发布的笔记信息
	UserInfo      interface{} `json:"user_info"`      // 用户信息
	Timestamp     int64       `json:"timestamp"`      // 发送时间戳
	Event         string      `json:"event"`          // 事件类型（publish_content/publish_video）
}

// WebhookSender webhook 发送器
type WebhookSender struct {
	client  *http.Client
	timeout time.Duration
}

// NewWebhookSender 创建 webhook 发送器
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		timeout: 10 * time.Second,
	}
}

// SendAsync 异步发送 webhook
//
// 参数：
//   - webhookURL: webhook 接收地址
//   - publishedNote: 发布的笔记信息
//   - userInfo: 用户信息
//   - eventType: 事件类型（publish_content/publish_video）
//
// 特点：
//   - 异步执行，不阻塞主流程
//   - 失败只记录日志，不影响发布结果
//   - 自动添加 panic 恢复
func (w *WebhookSender) SendAsync(webhookURL string, publishedNote interface{}, userInfo interface{}, eventType string) {
	go func() {
		// panic 恢复
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("webhook panic: %v", r)
			}
		}()

		// 发送 webhook
		if err := w.send(webhookURL, publishedNote, userInfo, eventType); err != nil {
			logrus.Errorf("webhook 发送失败 [%s]: %v", webhookURL, err)
		} else {
			logrus.Infof("webhook 发送成功 [%s]", webhookURL)
		}
	}()
}

// send 实际发送 webhook（同步）
func (w *WebhookSender) send(webhookURL string, publishedNote interface{}, userInfo interface{}, eventType string) error {
	// 1. 验证 URL
	if err := w.validateURL(webhookURL); err != nil {
		return fmt.Errorf("无效的 webhook URL: %w", err)
	}

	// 2. 构建 payload
	payload := WebhookPayload{
		PublishedNote: publishedNote,
		UserInfo:      userInfo,
		Timestamp:     time.Now().Unix(),
		Event:         eventType,
	}

	// 3. 序列化为 JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 payload 失败: %w", err)
	}

	// 4. 创建请求
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 5. 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "xiaohongshu-mcp-webhook/1.0")

	// 6. 发送请求
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 7. 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回非成功状态码: %d", resp.StatusCode)
	}

	return nil
}

// validateURL 验证 webhook URL 是否有效
func (w *WebhookSender) validateURL(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL 不能为空")
	}

	u, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("URL 格式错误: %w", err)
	}

	// 只允许 http 和 https
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("只支持 http 和 https 协议")
	}

	// 必须有 host
	if u.Host == "" {
		return fmt.Errorf("URL 必须包含 host")
	}

	return nil
}
