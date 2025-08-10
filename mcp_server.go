package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

// JSON-RPC 结构定义

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCP 工具处理函数

// handleCheckLoginStatus 处理检查登录状态
func handleCheckLoginStatus(ctx context.Context) *MCPToolResult {
	logrus.Info("MCP: 检查登录状态")

	status, err := xiaohongshuService.CheckLoginStatus(ctx)
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
func handlePublishContent(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	logrus.Info("MCP: 发布内容")

	// 解析参数
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	imagePathsInterface, _ := args["image_paths"].([]interface{})

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
	result, err := xiaohongshuService.PublishContent(ctx, req)
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

// handleMCPRequest 处理 MCP 请求
func handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("解析请求失败: %v", err)
		sendJSONRPCError(w, req.ID, -32700, "Parse error", nil)
		return
	}

	logrus.Infof("收到MCP请求: %s", req.Method)

	switch req.Method {
	case "initialize":
		handleInitialize(w, req)
	case "tools/list":
		handleToolsList(w, req)
	case "tools/call":
		handleToolsCall(w, r, req)
	default:
		logrus.Warnf("不支持的方法: %s", req.Method)
		sendJSONRPCError(w, req.ID, -32601, "Method not found", nil)
	}
}

// handleInitialize 处理初始化请求
func handleInitialize(w http.ResponseWriter, req JSONRPCRequest) {
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

	sendJSONRPCResponse(w, req.ID, result)
}

// handleToolsList 处理工具列表请求
func handleToolsList(w http.ResponseWriter, req JSONRPCRequest) {
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
					"image_ids": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "图片ID列表（至少一个）",
						"minItems":    1,
					},
				},
				"required": []string{"title", "content", "image_ids"},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	sendJSONRPCResponse(w, req.ID, result)
}

// handleToolsCall 处理工具调用请求
func handleToolsCall(w http.ResponseWriter, r *http.Request, req JSONRPCRequest) {
	var toolCall MCPToolCall
	paramsBytes, _ := json.Marshal(req.Params)
	if err := json.Unmarshal(paramsBytes, &toolCall); err != nil {
		logrus.Errorf("解析工具调用参数失败: %v", err)
		sendJSONRPCError(w, req.ID, -32602, "Invalid params", nil)
		return
	}

	ctx := r.Context()
	var result *MCPToolResult

	switch toolCall.Name {
	case "check_login_status":
		result = handleCheckLoginStatus(ctx)
	case "publish_content":
		result = handlePublishContent(ctx, toolCall.Arguments)
	default:
		logrus.Warnf("不支持的工具: %s", toolCall.Name)
		sendJSONRPCError(w, req.ID, -32601, "Tool not found", nil)
		return
	}

	sendJSONRPCResponse(w, req.ID, result)
}

// sendJSONRPCResponse 发送JSON-RPC响应
func sendJSONRPCResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendJSONRPCError 发送JSON-RPC错误响应
func sendJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
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
	json.NewEncoder(w).Encode(response)
}

// createMCPHandler 创建MCP HTTP处理器
func createMCPHandler() http.HandlerFunc {
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
		handleMCPRequest(w, r)
	}
}
