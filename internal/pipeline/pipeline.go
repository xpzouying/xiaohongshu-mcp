package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

const (
	dmxAPIBase    = "https://www.dmxapi.cn/v1/chat/completions"
	modelAnalyze  = "DeepSeek-V3.2-Thinking"
	modelGenerate = "DeepSeek-V3.2"
)

// Config 流水线配置，所有字段从环境变量注入
type Config struct {
	DMXAPIKey      string // DMXAPI_KEY
	FeishuWebhook  string // FEISHU_WEBHOOK_URL (Flow Webhook，接收结果)
	MinLikes       int    // 最低点赞数阈值
	BrowserBinPath string // 浏览器路径（直连模式，MCPServerURL 为空时生效）
	MCPServerURL   string // MCP 服务地址（如 http://xiaohongshu-mcp:18060/mcp），设置后不启动本地浏览器
}

// Run 执行完整流水线：搜索热帖 -> AI分析 -> 生成帖子 -> 推送飞书Flow
func Run(ctx context.Context, cfg Config, keyword string) error {
	logrus.Infof("开始搜索爆款帖子，关键词: %s", keyword)

	var summary string
	var err error
	if cfg.MCPServerURL != "" {
		logrus.Infof("通过 MCP 服务获取热帖: %s", cfg.MCPServerURL)
		summary, err = fetchSummaryViaMCP(ctx, cfg.MCPServerURL, keyword, cfg.MinLikes)
	} else {
		summary, err = fetchSummaryDirect(ctx, keyword, cfg.MinLikes, cfg.BrowserBinPath)
	}
	if err != nil {
		return fmt.Errorf("搜索失败: %w", err)
	}
	logrus.Infof("热帖摘要 %d 字", len(summary))

	logrus.Infof("分析爆款规律 (%s)...", modelAnalyze)
	analysis, err := callDeepSeek(ctx, cfg.DMXAPIKey, modelAnalyze, buildAnalyzePrompt(keyword, summary))
	if err != nil {
		return fmt.Errorf("爆款分析失败: %w", err)
	}

	logrus.Infof("生成帖子 (%s)...", modelGenerate)
	posts, err := callDeepSeek(ctx, cfg.DMXAPIKey, modelGenerate, buildGeneratePrompt(keyword, summary, analysis))
	if err != nil {
		return fmt.Errorf("生成帖子失败: %w", err)
	}

	logrus.Info("发送到飞书Flow...")
	return sendToFeishuFlow(cfg.FeishuWebhook, keyword, analysis, posts)
}

// fetchSummaryDirect 直连浏览器搜索，返回纯文本摘要
func fetchSummaryDirect(ctx context.Context, keyword string, minLikes int, binPath string) (string, error) {
	b := browser.NewBrowser(true, browser.WithBinPath(binPath))
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewSearchAction(page)
	feeds, err := action.Search(ctx, keyword)
	if err != nil {
		return "", err
	}

	feeds = xiaohongshu.SortFeeds(feeds, "最多点赞")
	feeds = xiaohongshu.FilterByThreshold(feeds, minLikes, 0, 0)
	if len(feeds) > 10 {
		feeds = feeds[:10]
	}
	return buildFeedsSummary(feeds), nil
}

func buildFeedsSummary(feeds []xiaohongshu.Feed) string {
	var sb strings.Builder
	for i, f := range feeds {
		info := f.NoteCard.InteractInfo
		sb.WriteString(fmt.Sprintf("%d. 标题：%s\n", i+1, f.NoteCard.DisplayTitle))
		sb.WriteString(fmt.Sprintf("   作者：%s\n", f.NoteCard.User.Nickname))
		sb.WriteString(fmt.Sprintf("   点赞：%s  收藏：%s  评论：%s\n\n", info.LikedCount, info.CollectedCount, info.CommentCount))
	}
	return sb.String()
}

// fetchSummaryViaMCP 通过 MCP HTTP 服务获取热帖摘要（不启动本地浏览器）
func fetchSummaryViaMCP(ctx context.Context, mcpURL, keyword string, minLikes int) (string, error) {
	hdrs, err := mcpInit(ctx, mcpURL)
	if err != nil {
		return "", fmt.Errorf("MCP 初始化失败: %w", err)
	}

	body, err := mcpCall(ctx, mcpURL, hdrs, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_hot_feeds",
			"arguments": map[string]interface{}{
				"keyword":   keyword,
				"min_likes": minLikes,
				"sort_by":   "最多点赞",
			},
		},
	})
	if err != nil {
		return "", err
	}

	// 解析 JSON-RPC 响应，取 result.content[0].text
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析 MCP 响应失败: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("MCP 错误: %s", resp.Error.Message)
	}
	if len(resp.Result.Content) == 0 {
		return "", fmt.Errorf("MCP 返回空内容")
	}

	text := resp.Result.Content[0].Text
	// 截取 "---" 之前的摘要部分，去掉原始 JSON
	if idx := strings.Index(text, "---"); idx != -1 {
		text = text[:idx]
	}
	return strings.TrimSpace(text), nil
}

// mcpInit 初始化 MCP 会话，返回带 session ID 的请求头
func mcpInit(ctx context.Context, mcpURL string) (map[string]string, error) {
	hdrs := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}
	body, err := mcpCall(ctx, mcpURL, hdrs, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "feishu-bot", "version": "1.0"},
		},
	})
	if err != nil {
		return nil, err
	}
	_ = body

	// initialized 通知
	_, _ = mcpCallRaw(ctx, mcpURL, hdrs, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]interface{}{},
	})
	return hdrs, nil
}

func mcpCall(ctx context.Context, mcpURL string, hdrs map[string]string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 记录 session ID（首次初始化时服务端可能下发）
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		hdrs["Mcp-Session-Id"] = sid
	}
	return io.ReadAll(resp.Body)
}

func mcpCallRaw(ctx context.Context, mcpURL string, hdrs map[string]string, payload interface{}) ([]byte, error) {
	return mcpCall(ctx, mcpURL, hdrs, payload)
}

func buildAnalyzePrompt(keyword, feedsSummary string) string {
	return fmt.Sprintf(`你是一位专业的小红书内容分析师。请深度分析以下关于"%s"的爆款帖子数据：

%s
请从以下维度进行结构化分析：
1. 标题规律（情绪词、数字、疑问句、痛点词等）
2. 内容结构（开头钩子类型、叙事方式、结尾互动策略）
3. 高互动核心原因（情感共鸣点、用户痛点、利益驱动点）
4. 可复制的写作技巧和话题角度

请给出具体可操作的分析结论，用于指导创作新帖子。`, keyword, feedsSummary)
}

func buildGeneratePrompt(keyword, feedsSummary, analysis string) string {
	return fmt.Sprintf(`你是一位小红书爆款内容创作者。

【爆款规律分析】
%s

【参考爆款数据】
%s

请基于以上分析，围绕"%s"创作5篇原创小红书帖子。要求：
1. 每篇标题20字以内，结合爆款规律设计钩子
2. 正文200-400字，口语化、有互动感，符合小红书风格
3. 5-8个话题标签
4. 5篇主题各有侧重，覆盖不同角度（如：教程/避坑/好物/故事/测评）

严格按以下格式输出，不要输出其他内容：

===帖子1===
标题：xxx
正文：xxx
标签：#xxx #xxx #xxx
===

===帖子2===
标题：xxx
正文：xxx
标签：#xxx #xxx #xxx
===

===帖子3===
标题：xxx
正文：xxx
标签：#xxx #xxx #xxx
===

===帖子4===
标题：xxx
正文：xxx
标签：#xxx #xxx #xxx
===

===帖子5===
标题：xxx
正文：xxx
标签：#xxx #xxx #xxx
===`, analysis, feedsSummary, keyword)
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callDeepSeek(ctx context.Context, apiKey, model, prompt string) (string, error) {
	reqBody := openAIRequest{
		Model:    model,
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dmxAPIBase, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DMXAPI(%s) 返回错误 %d: %s", model, resp.StatusCode, string(body))
	}

	var or openAIResponse
	if err := json.Unmarshal(body, &or); err != nil {
		return "", fmt.Errorf("解析响应失败: %w, body: %s", err, string(body))
	}
	if len(or.Choices) == 0 || or.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("模型 %s 返回空内容", model)
	}
	return or.Choices[0].Message.Content, nil
}

type feishuFlowPayload struct {
	MsgType string                 `json:"msg_type"`
	Content map[string]interface{} `json:"content"`
}

func sendToFeishuFlow(webhookURL, keyword, analysis, posts string) error {
	payload := feishuFlowPayload{
		MsgType: "text",
		Content: map[string]interface{}{
			"title":    fmt.Sprintf("小红书爆款分析 | %s | %s", keyword, time.Now().Format("2006-01-02")),
			"analysis": analysis,
			"posts":    posts,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书Flow Webhook 返回错误 %d: %s", resp.StatusCode, string(body))
	}
	logrus.Infof("飞书Flow响应: %s", string(body))
	return nil
}
