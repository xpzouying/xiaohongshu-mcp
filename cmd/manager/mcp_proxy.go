package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// withMCPSession 创建MCP会话并执行操作
func (a *App) withMCPSession(ctx context.Context, port int, timeout time.Duration, fn func(context.Context, *mcp.ClientSession) error) error {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "xiaohongshu-mcp-manager",
		Version: "dev",
	}, nil)

	transport := &mcp.StreamableClientTransport{
		Endpoint:   fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
		HTTPClient: &http.Client{Timeout: timeout},
		MaxRetries: 0, // 不重试
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("连接MCP服务失败: %w", err)
	}
	defer session.Close()

	return fn(ctx, session)
}

// fetchMCPTools 获取MCP工具列表
func (a *App) fetchMCPTools(ctx context.Context, port int) ([]MCPToolInfo, error) {
	var out []MCPToolInfo
	if err := a.withMCPSession(ctx, port, 15*time.Second, func(ctx context.Context, session *mcp.ClientSession) error {
		toolsResult, err := session.ListTools(ctx, nil)
		if err != nil {
			return fmt.Errorf("获取工具列表失败: %w", err)
		}

		out = make([]MCPToolInfo, 0, len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			var schemaMap map[string]any
			if tool.InputSchema != nil {
				b, err := json.Marshal(tool.InputSchema)
				if err != nil {
					return fmt.Errorf("序列化 inputSchema 失败: %w", err)
				}
				if err := json.Unmarshal(b, &schemaMap); err != nil {
					return fmt.Errorf("解析 inputSchema 失败: %w", err)
				}
			}
			out = append(out, MCPToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schemaMap,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// callMCPTool 调用MCP工具
func (a *App) callMCPTool(ctx context.Context, port int, name string, args map[string]any, timeout time.Duration) (*MCPCallResponse, error) {
	var out *MCPCallResponse
	if err := a.withMCPSession(ctx, port, timeout, func(ctx context.Context, session *mcp.ClientSession) error {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		})
		if err != nil {
			return fmt.Errorf("调用工具失败: %w", err)
		}

		out = &MCPCallResponse{
			IsError: res.IsError,
		}
		for _, c := range res.Content {
			switch content := c.(type) {
			case *mcp.TextContent:
				out.Content = append(out.Content, MCPContent{
					Type: "text",
					Text: content.Text,
				})
			default:
				// 其他类型转为JSON字符串
				b, _ := json.Marshal(content)
				out.Content = append(out.Content, MCPContent{
					Type: "unknown",
					Text: string(b),
				})
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("MCP 返回空结果")
	}
	return out, nil
}
