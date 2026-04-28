package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/internal/pipeline"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})

	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	if appID == "" || appSecret == "" {
		logrus.Fatal("请设置环境变量 FEISHU_APP_ID 和 FEISHU_APP_SECRET")
	}

	cfg := pipeline.Config{
		DMXAPIKey:     os.Getenv("DMXAPI_KEY"),
		FeishuWebhook: os.Getenv("FEISHU_WEBHOOK_URL"),
		MinLikes:      500,
		MCPServerURL:  os.Getenv("MCP_SERVER_URL"), // 默认走 MCP 服务，不在本地启动浏览器
	}
	if cfg.MCPServerURL == "" {
		cfg.MCPServerURL = "http://xiaohongshu-mcp:18060/mcp" // Docker 网络默认服务名
	}
	if cfg.DMXAPIKey == "" {
		logrus.Fatal("请设置环境变量 DMXAPI_KEY")
	}
	if cfg.FeishuWebhook == "" {
		logrus.Fatal("请设置环境变量 FEISHU_WEBHOOK_URL")
	}

	client := lark.NewClient(appID, appSecret)

	handler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			return handleMessage(ctx, client, cfg, event)
		})

	wsClient := larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(handler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)

	logrus.Info("飞书机器人已启动，等待消息（@机器人 + 关键词）...")
	if err := wsClient.Start(context.Background()); err != nil {
		logrus.Fatalf("WebSocket 连接失败: %v", err)
	}
}

func handleMessage(ctx context.Context, client *lark.Client, cfg pipeline.Config, event *larkim.P2MessageReceiveV1) error {
	msg := event.Event.Message
	if msg == nil || msg.MessageType == nil || *msg.MessageType != "text" {
		return nil
	}

	keyword := parseKeyword(event)
	if keyword == "" {
		return nil
	}

	chatID := ""
	if msg.ChatId != nil {
		chatID = *msg.ChatId
	}

	logrus.Infof("收到触发请求，关键词: %s，群: %s", keyword, chatID)
	sendGroupMsg(ctx, client, chatID, "收到！开始处理关键词："+keyword+"（约5-8分钟，完成后结果推送到飞书）")

	go func() {
		pipeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := pipeline.Run(pipeCtx, cfg, keyword); err != nil {
			logrus.Errorf("流水线执行失败: %v", err)
			sendGroupMsg(context.Background(), client, chatID, "处理失败："+err.Error())
			return
		}
		sendGroupMsg(context.Background(), client, chatID, "完成！「"+keyword+"」分析结果已推送。")
	}()

	return nil
}

type textContent struct {
	Text string `json:"text"`
}

// parseKeyword 从消息中提取关键词，去掉 @mention 占位符（形如 @_user_1）
func parseKeyword(event *larkim.P2MessageReceiveV1) string {
	if event.Event.Message.Content == nil {
		return ""
	}
	var tc textContent
	if err := json.Unmarshal([]byte(*event.Event.Message.Content), &tc); err != nil {
		return ""
	}
	text := tc.Text
	for _, m := range event.Event.Message.Mentions {
		if m.Key != nil {
			text = strings.ReplaceAll(text, *m.Key, "")
		}
	}
	return strings.TrimSpace(text)
}

func sendGroupMsg(ctx context.Context, client *lark.Client, chatID, text string) {
	if chatID == "" {
		return
	}
	content, _ := json.Marshal(map[string]string{"text": text})
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(string(content)).
			Build()).
		Build()
	if _, err := client.Im.Message.Create(ctx, req); err != nil {
		logrus.Errorf("发送飞书群消息失败: %v", err)
	}
}
