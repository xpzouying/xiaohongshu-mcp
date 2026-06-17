package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var (
		endpoint   = flag.String("endpoint", "http://127.0.0.1:18060/mcp", "MCP server endpoint")
		title      = flag.String("title", "每日心情分享", "publish title")
		content    = flag.String("content", "今天天气不错，心情也很好，分享一下今天的日常。", "publish content")
		visibility = flag.String("visibility", "仅自己可见", "visibility")
	)
	flag.Parse()

	imagePath, err := resolveLocalPublishImage()
	if err != nil {
		fatalf("resolve image failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	fmt.Printf("Using MCP endpoint: %s\n", *endpoint)
	fmt.Printf("Using local image: %s\n", imagePath)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "xiaohongshu-mcp-publish-script",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: *endpoint,
	}, nil)
	if err != nil {
		fatalf("connect MCP server failed: %v", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		fatalf("list tools failed: %v", err)
	}
	if !hasToolNamed(toolsResult.Tools, "publish_content") {
		fatalf("publish_content tool not found")
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "publish_content",
		Arguments: map[string]any{
			"title":       *title,
			"content":     *content,
			"images":      []string{imagePath},
			"tags":        []string{"日常", "心情", "分享"},
			"is_original": false,
			"visibility":  *visibility,
		},
	})
	if err != nil {
		fatalf("call publish_content failed: %v", err)
	}

	if len(result.Content) == 0 {
		fatalf("publish_content returned empty content")
	}

	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok {
			fmt.Println(text.Text)
		}
	}

	if result.IsError {
		os.Exit(1)
	}
}

func resolveLocalPublishImage() (string, error) {
	if imagePath := os.Getenv("XHS_PUBLISH_IMAGE"); imagePath != "" {
		return filepath.Abs(imagePath)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(homeDir, "Downloads", "123.jpg"),
		filepath.Join(repoRoot(), "Downloads", "123.jpg"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Abs(candidate)
		}
	}

	return "", fmt.Errorf("local image not found, tried: %s", strings.Join(candidates, ", "))
}

func hasToolNamed(tools []*mcp.Tool, name string) bool {
	for _, tool := range tools {
		if tool != nil && tool.Name == name {
			return true
		}
	}
	return false
}

func repoRoot() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		fatalf("resolve script path failed")
	}
	return filepath.Dir(filepath.Dir(currentFile))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
