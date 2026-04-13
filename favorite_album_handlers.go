package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

// ==================== MCP 工具参数定义 ====================

// GetFavoriteListArgs 获取收藏列表参数
type GetFavoriteListArgs struct {
	MaxItems int `json:"max_items,omitempty" jsonschema:"最大获取数量，0表示全部获取（默认），建议值：20-200"`
}

// ClassifyFavoritesArgs AI 分类收藏笔记参数
type ClassifyFavoritesArgs struct {
	Categories []string `json:"categories,omitempty" jsonschema:"自定义分类名称列表，如[\"美食\",\"旅行\",\"科技\"]。留空则使用内置分类规则"`
	BatchSize  int      `json:"batch_size,omitempty" jsonschema:"每批处理的笔记数量，默认10。使用 LLM API 时控制单次请求量"`
}

// SyncFavoritesToAlbumsArgs 一键同步收藏到专辑参数
type SyncFavoritesToAlbumsArgs struct {
	Categories       []string `json:"categories,omitempty" jsonschema:"自定义分类名称列表，留空使用内置分类"`
	BatchSize        int      `json:"batch_size,omitempty" jsonschema:"每批处理数量，默认10"`
	AutoCreateAlbums bool     `json:"auto_create_albums,omitempty" jsonschema:"是否自动创建不存在的专辑，默认true"`
	ClassifiedData   string   `json:"classified_data,omitempty" jsonschema:"预分类的 JSON 数据（由外部 LLM 生成），如果提供则跳过内部分类，直接使用此数据同步"`
}

// ManageAlbumsArgs 专辑管理参数
type ManageAlbumsArgs struct {
	Action string `json:"action" jsonschema:"操作类型: list(查看列表), create(创建), delete(删除)"`
	Name   string `json:"name,omitempty" jsonschema:"专辑名称（create/delete 操作时需要）"`
}

// ==================== MCP 工具处理函数 ====================

// handleGetFavoriteList 处理获取收藏列表
func (s *AppServer) handleGetFavoriteList(ctx context.Context, args GetFavoriteListArgs) *MCPToolResult {
	logrus.Info("MCP: 获取收藏列表")

	maxItems := args.MaxItems
	if maxItems < 0 {
		maxItems = 0
	}

	result, err := s.xiaohongshuService.GetFavoriteListService(ctx, maxItems)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "获取收藏列表失败: " + err.Error()}},
			IsError: true,
		}
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("获取成功但序列化失败: %v", err)}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{Type: "text", Text: string(jsonData)}},
	}
}

// handleClassifyFavorites 处理 AI 分类收藏笔记
func (s *AppServer) handleClassifyFavorites(ctx context.Context, args ClassifyFavoritesArgs) *MCPToolResult {
	logrus.Info("MCP: AI 分类收藏笔记")

	// Step 1: 先获取收藏列表
	logrus.Info("  Step 1: 获取收藏列表...")
	favResp, err := s.xiaohongshuService.GetFavoriteListService(ctx, 0)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "获取收藏列表失败: " + err.Error()}},
			IsError: true,
		}
	}

	if len(favResp.Items) == 0 {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "收藏列表为空，请先收藏一些笔记"}},
			IsError: true,
		}
	}

	logrus.Infof("  获取到 %d 条收藏，开始分类...", len(favResp.Items))

	// Step 2: 执行分类
	batchSize := args.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	classifyResult, err := s.xiaohongshuService.ClassifyFavorites(ctx, favResp.Items, args.Categories, batchSize)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "分类失败: " + err.Error()}},
			IsError: true,
		}
	}

	// 构建可读摘要
	summary := fmt.Sprintf("📊 分类完成！共 %d 条笔记\n\n", classifyResult.Total)
	summary += "分类统计:\n"
	for cat, count := range classifyResult.Stats {
		summary += fmt.Sprintf("  %s: %d 条\n", cat, count)
	}
	summary += "\n详细结果 (JSON):\n"

	jsonData, err := json.MarshalIndent(classifyResult, "", "  ")
	if err != nil {
		summary += fmt.Sprintf("(序列化失败: %v)", err)
	} else {
		summary += string(jsonData)
	}

	return &MCPToolResult{
		Content: []MCPContent{{Type: "text", Text: summary}},
	}
}

// handleSyncFavoritesToAlbums 处理一键同步收藏到专辑
func (s *AppServer) handleSyncFavoritesToAlbums(ctx context.Context, args SyncFavoritesToAlbumsArgs) *MCPToolResult {
	logrus.Info("MCP: 同步收藏到专辑")

	batchSize := args.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	var result *ClassifyFavoritesResult
	var err error

	// 如果提供了预分类数据，直接使用
	if args.ClassifiedData != "" {
		logrus.Info("  使用预分类数据进行同步...")
		var classified ClassifyFavoritesResult
		if err := json.Unmarshal([]byte(args.ClassifiedData), &classified); err != nil {
			return &MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "解析预分类数据失败: " + err.Error()}},
				IsError: true,
			}
		}
		result = &classified
	} else {
		// 否则走内部分类流程
		logrus.Info("  Step 1: 获取收藏列表...")
		favResp, err := s.xiaohongshuService.GetFavoriteListService(ctx, 0)
		if err != nil {
			return &MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "获取收藏列表失败: " + err.Error()}},
				IsError: true,
			}
		}

		if len(favResp.Items) == 0 {
			return &MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "收藏列表为空，请先收藏一些笔记"}},
				IsError: true,
			}
		}

		logrus.Infof("  获取到 %d 条收藏，开始分类...", len(favResp.Items))
		result, err = s.xiaohongshuService.ClassifyFavorites(ctx, favResp.Items, args.Categories, batchSize)
		if err != nil {
			return &MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "分类失败: " + err.Error()}},
				IsError: true,
			}
		}
	}

	// 同步到专辑（需要浏览器操作）
	logrus.Info("  📚 开始同步到专辑...")
	syncResult, err := s.xiaohongshuService.ApplyClassificationToAlbums(ctx, result, args.AutoCreateAlbums)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "同步专辑失败: " + err.Error()}},
			IsError: true,
		}
	}

	// 构建可读摘要
	summary := "🎉 同步完成！\n\n"
	summary += fmt.Sprintf("共 %d 条笔记，同步到 %d 个专辑\n\n", result.Total, len(result.Categories))
	summary += "专辑同步结果:\n"
	for cat, count := range result.Stats {
		syncStatus := "✅"
		if syncResult.FailedAlbums[cat] > 0 {
			syncStatus = fmt.Sprintf("⚠️ 失败 %d 条", syncResult.FailedAlbums[cat])
		}
		summary += fmt.Sprintf("  %s: %d 条 - %s\n", cat, count, syncStatus)
	}

	return &MCPToolResult{
		Content: []MCPContent{{Type: "text", Text: summary}},
	}
}

// handleManageAlbums 处理专辑管理
func (s *AppServer) handleManageAlbums(ctx context.Context, args ManageAlbumsArgs) *MCPToolResult {
	logrus.Infof("MCP: 专辑管理 - action=%s, name=%s", args.Action, args.Name)

	switch args.Action {
	case "list":
		return s.handleListAlbums(ctx)
	case "create":
		return s.handleCreateAlbum(ctx, args.Name)
	case "delete":
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "删除专辑功能暂未实现，请在小红书网页端手动删除"}},
			IsError: true,
		}
	default:
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("未知操作: %s，支持的操作: list, create, delete", args.Action)}},
			IsError: true,
		}
	}
}

// handleListAlbums 列出专辑
func (s *AppServer) handleListAlbums(ctx context.Context) *MCPToolResult {
	result, err := s.xiaohongshuService.GetAlbumList(ctx)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "获取专辑列表失败: " + err.Error()}},
			IsError: true,
		}
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("获取成功但序列化失败: %v", err)}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{Type: "text", Text: string(jsonData)}},
	}
}

// handleCreateAlbum 创建专辑
func (s *AppServer) handleCreateAlbum(ctx context.Context, name string) *MCPToolResult {
	if name == "" {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "创建专辑失败: 缺少专辑名称 (name 参数)"}},
			IsError: true,
		}
	}

	result, err := s.xiaohongshuService.CreateAlbumService(ctx, name)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "创建专辑失败: " + err.Error()}},
			IsError: true,
		}
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &MCPToolResult{
		Content: []MCPContent{{Type: "text", Text: string(jsonData)}},
	}
}
