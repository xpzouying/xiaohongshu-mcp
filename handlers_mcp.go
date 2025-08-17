package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

// MCP 工具处理函数

// handleCheckLoginStatus 处理检查登录状态
func (s *AppServer) handleCheckLoginStatus(ctx context.Context) *MCPToolResult {
	logrus.Info("MCP: 检查登录状态")

	status, err := s.xiaohongshuService.CheckLoginStatus(ctx)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "检查登录状态失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	resultText := fmt.Sprintf("登录状态检查成功: %+v", status)
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: resultText,
		}},
	}
}

// handlePublishContent 处理发布内容
func (s *AppServer) handlePublishContent(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	logrus.Info("MCP: 发布内容")

	// 解析参数
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	imagePathsInterface, _ := args["images"].([]interface{})

	var imagePaths []string
	for _, path := range imagePathsInterface {
		if pathStr, ok := path.(string); ok {
			imagePaths = append(imagePaths, pathStr)
		}
	}

	logrus.Infof("MCP: 发布内容 - 标题: %s, 图片数量: %d", title, len(imagePaths))

	// 构建发布请求
	req := &PublishRequest{
		Title:   title,
		Content: content,
		Images:  imagePaths,
	}

	// 执行发布
	result, err := s.xiaohongshuService.PublishContent(ctx, req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发布失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	resultText := fmt.Sprintf("内容发布成功: %+v", result)
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: resultText,
		}},
	}
}

// handleListFeeds 处理获取Feeds列表
func (s *AppServer) handleListFeeds(ctx context.Context) *MCPToolResult {
	logrus.Info("MCP: 获取Feeds列表")

	result, err := s.xiaohongshuService.ListFeeds(ctx)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取Feeds列表失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	// 格式化输出，转换为JSON字符串
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("获取Feeds列表成功，但序列化失败: %v", err),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

// handleMCPRequest 处理 MCP 请求
func (s *AppServer) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("解析请求失败: %v", err)
		s.sendJSONRPCError(w, req.ID, -32700, "Parse error", nil)
		return
	}

	logrus.Infof("收到MCP请求: %s", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, req)
	case "tools/list":
		s.handleToolsList(w, req)
	case "tools/call":
		s.handleToolsCall(w, r, req)
	default:
		logrus.Warnf("不支持的方法: %s", req.Method)
		s.sendJSONRPCError(w, req.ID, -32601, "Method not found", nil)
	}
}

// handleInitialize 处理初始化请求
func (s *AppServer) handleInitialize(w http.ResponseWriter, req JSONRPCRequest) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "xiaohongshu-mcp",
			"version": "v1.0.0",
		},
	}

	s.sendJSONRPCResponse(w, req.ID, result)
}

// handleToolsList 处理工具列表请求
func (s *AppServer) handleToolsList(w http.ResponseWriter, req JSONRPCRequest) {
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
			"description": "发布内容到小红书",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "发布内容的标题",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "发布内容的正文",
					},
					"images": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "图片路径或URL列表（支持本地文件路径和HTTP/HTTPS图片URL，至少一个）",
						"minItems":    1,
					},
				},
				"required": []string{"title", "content", "images"},
			},
		},
		{
			"name":        "list_feeds",
			"description": "获取小红书首页Feeds列表",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	s.sendJSONRPCResponse(w, req.ID, result)
}

// handleToolsCall 处理工具调用请求
func (s *AppServer) handleToolsCall(w http.ResponseWriter, r *http.Request, req JSONRPCRequest) {
	var toolCall MCPToolCall
	paramsBytes, _ := json.Marshal(req.Params)
	if err := json.Unmarshal(paramsBytes, &toolCall); err != nil {
		logrus.Errorf("解析工具调用参数失败: %v", err)
		s.sendJSONRPCError(w, req.ID, -32602, "Invalid params", nil)
		return
	}

	ctx := r.Context()
	var result *MCPToolResult

	switch toolCall.Name {
	case "check_login_status":
		result = s.handleCheckLoginStatus(ctx)
	case "publish_content":
		result = s.handlePublishContent(ctx, toolCall.Arguments)
	case "list_feeds":
		result = s.handleListFeeds(ctx)
	default:
		logrus.Warnf("不支持的工具: %s", toolCall.Name)
		s.sendJSONRPCError(w, req.ID, -32601, "Tool not found", nil)
		return
	}

	s.sendJSONRPCResponse(w, req.ID, result)
}

// sendJSONRPCResponse 发送JSON-RPC响应
func (s *AppServer) sendJSONRPCResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logrus.Errorf("Failed to encode JSON-RPC response: %v", err)
	}
}

// sendJSONRPCError 发送JSON-RPC错误响应
func (s *AppServer) sendJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC错误仍然返回200状态码
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logrus.Errorf("Failed to encode JSON-RPC error response: %v", err)
	}
}

// createMCPHandler 创建MCP HTTP处理器
func (s *AppServer) createMCPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 设置 CORS 头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 处理 MCP JSON-RPC 请求
		s.handleMCPRequest(w, r)
	}
}
