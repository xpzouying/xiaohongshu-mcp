package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// StreamableHTTPHandler 处理 Streamable HTTP 协议的 MCP 请求
func (s *AppServer) StreamableHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 设置 CORS 头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id")

		// 处理 OPTIONS 请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 根据方法处理
		switch r.Method {
		case "GET":
			// GET 请求用于建立 SSE 连接（可选功能）
			s.handleSSEConnection(w, r)
		case "POST":
			// POST 请求处理 JSON-RPC
			s.handleJSONRPCRequest(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleSSEConnection 处理 SSE 连接（可选，用于服务器推送）
func (s *AppServer) handleSSEConnection(w http.ResponseWriter, r *http.Request) {
	// 检查是否支持 SSE
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "SSE not requested", http.StatusBadRequest)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 发送初始化消息
	fmt.Fprintf(w, "event: open\n")
	fmt.Fprintf(w, "data: {\"type\":\"connection\",\"status\":\"connected\"}\n\n")

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// 保持连接打开（实际使用中可以在这里推送通知）
	<-r.Context().Done()
}

// handleJSONRPCRequest 处理 JSON-RPC 请求
func (s *AppServer) handleJSONRPCRequest(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendStreamableError(w, nil, -32700, "Parse error")
		return
	}
	defer r.Body.Close()

	// 解析 JSON-RPC 请求
	var request JSONRPCRequest
	if err := json.Unmarshal(body, &request); err != nil {
		s.sendStreamableError(w, nil, -32700, "Parse error")
		return
	}

	logrus.WithField("method", request.Method).Info("Received Streamable HTTP request")

	// 检查 Accept 头，判断客户端是否支持 SSE
	acceptSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	// 处理请求
	response := s.processJSONRPCRequest(&request, r.Context())

	// 如果需要 SSE 且是支持流式的方法，使用 SSE 响应
	if acceptSSE && s.isStreamableMethod(request.Method) {
		s.sendSSEResponse(w, response)
	} else {
		// 否则使用普通 JSON 响应
		s.sendJSONResponse(w, response)
	}
}

// processJSONRPCRequest 处理 JSON-RPC 请求并返回响应
func (s *AppServer) processJSONRPCRequest(request *JSONRPCRequest, ctx context.Context) *JSONRPCResponse {
	switch request.Method {
	case "initialize":
		return s.processInitialize(request)
	case "initialized":
		// 客户端确认初始化完成
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]interface{}{},
			ID:      request.ID,
		}
	case "ping":
		// 处理 ping 请求
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]interface{}{},
			ID:      request.ID,
		}
	case "tools/list":
		return s.processToolsList(request)
	case "tools/call":
		return s.processToolCall(ctx, request)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			ID: request.ID,
		}
	}
}

// processInitialize 处理初始化请求
func (s *AppServer) processInitialize(request *JSONRPCRequest) *JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": "2025-03-26", // 使用新的协议版本
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "xiaohongshu-mcp",
			"version": "2.0.0",
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}
}

// processToolsList 处理工具列表请求
func (s *AppServer) processToolsList(request *JSONRPCRequest) *JSONRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "check_login_status",
			"description": "检查小红书登录状态",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "publish_content",
			"description": "发布小红书图文内容",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "内容标题（小红书限制：最多20个中文字或英文单词）",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "正文内容，不包含以#开头的标签内容，所有话题标签都用tags参数来生成和提供即可",
					},
					"images": map[string]interface{}{
						"type":        "array",
						"description": "图片路径列表（至少需要1张图片）。支持两种方式：1. HTTP/HTTPS图片链接（自动下载）；2. 本地图片绝对路径（推荐，如:/Users/user/image.jpg）",
						"items": map[string]interface{}{
							"type": "string",
						},
						"minItems": 1,
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "话题标签列表（可选），如 [\"美食\", \"旅行\", \"生活\"]",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"title", "content", "images"},
			},
		},
		{
			"name":        "list_feeds",
			"description": "获取用户发布的内容列表",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "search_feeds",
			"description": "搜索小红书内容（需要已登录）",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				"required": []string{"keyword"},
			},
		},
		{
			"name":        "get_feed_detail",
			"description": "获取小红书笔记详情，返回笔记内容、图片、作者信息、互动数据（点赞/收藏/分享数）及评论列表",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"feed_id": map[string]interface{}{
						"type":        "string",
						"description": "小红书笔记ID，从Feed列表获取",
					},
					"xsec_token": map[string]interface{}{
						"type":        "string",
						"description": "访问令牌，从Feed列表的xsecToken字段获取",
					},
				},
				"required": []string{"feed_id", "xsec_token"},
			},
		},
		{
			"name":        "user_profile",
			"description": "获取小红书用户主页，返回用户基本信息，关注、粉丝、获赞量及其笔记内容",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "小红书用户ID，从Feed列表获取",
					},
					"xsec_token": map[string]interface{}{
						"type":        "string",
						"description": "访问令牌，从Feed列表的xsecToken字段获取",
					},
				},
				"required": []string{"user_id", "xsec_token"},
			},
		},
		{
			"name":        "post_comment_to_feed",
			"description": "发表评论到小红书笔记",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"feed_id": map[string]interface{}{
						"type":        "string",
						"description": "小红书笔记ID，从Feed列表获取",
					},
					"xsec_token": map[string]interface{}{
						"type":        "string",
						"description": "访问令牌，从Feed列表的xsecToken字段获取",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "评论内容",
					},
				},
				"required": []string{"feed_id", "xsec_token", "content"},
			},
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"tools": tools,
		},
		ID: request.ID,
	}
}

// processToolCall 处理工具调用
func (s *AppServer) processToolCall(ctx context.Context, request *JSONRPCRequest) *JSONRPCResponse {
	// 解析参数
	params, ok := request.Params.(map[string]interface{})
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			ID: request.ID,
		}
	}

	toolName, _ := params["name"].(string)
	toolArgs, _ := params["arguments"].(map[string]interface{})

	var result *MCPToolResult

	switch toolName {
	case "check_login_status":
		result = s.handleCheckLoginStatus(ctx)
	case "publish_content":
		result = s.handlePublishContent(ctx, toolArgs)
	case "list_feeds":
		result = s.handleListFeeds(ctx)
	case "search_feeds":
		result = s.handleSearchFeeds(ctx, toolArgs)
	case "get_feed_detail":
		result = s.handleGetFeedDetail(ctx, toolArgs)
	case "user_profile":
		result = s.handleUserProfile(ctx, toolArgs)
	case "post_comment_to_feed":
		result = s.handlePostComment(ctx, toolArgs)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", toolName),
			},
			ID: request.ID,
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}
}

// isStreamableMethod 判断方法是否支持流式响应
func (s *AppServer) isStreamableMethod(_ string) bool {
	// 目前我们的方法都不需要流式响应
	// 未来可以在这里添加支持流式的方法
	return false
}

// sendJSONResponse 发送普通 JSON 响应
func (s *AppServer) sendJSONResponse(w http.ResponseWriter, response *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logrus.WithError(err).Error("Failed to encode response")
	}
}

// sendSSEResponse 发送 SSE 响应
func (s *AppServer) sendSSEResponse(w http.ResponseWriter, response *JSONRPCResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 将响应转换为 JSON
	data, err := json.Marshal(response)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal SSE response")
		return
	}

	// 发送 SSE 格式的响应
	fmt.Fprintf(w, "data: %s\n\n", string(data))

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// sendStreamableError 发送错误响应
func (s *AppServer) sendStreamableError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := &JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	s.sendJSONResponse(w, response)
}
