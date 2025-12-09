package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
)

// MCP 工具参数结构体定义

// PublishContentArgs 发布内容的参数
type PublishContentArgs struct {
	Title   string   `json:"title" jsonschema:"内容标题（小红书限制：最多20个中文字或英文单词）"`
	Content string   `json:"content" jsonschema:"正文内容，不包含以#开头的标签内容，所有话题标签都用tags参数来生成和提供即可"`
	Images  []string `json:"images" jsonschema:"图片路径列表（至少需要1张图片）。支持两种方式：1. HTTP/HTTPS图片链接（自动下载）；2. 本地图片绝对路径（推荐，如:/Users/user/image.jpg）"`
	Tags    []string `json:"tags,omitempty" jsonschema:"话题标签列表（可选参数），如 [美食, 旅行, 生活]"`
}

// PublishVideoArgs 发布视频的参数（仅支持本地单个视频文件）
type PublishVideoArgs struct {
	Title   string   `json:"title" jsonschema:"内容标题（小红书限制：最多20个中文字或英文单词）"`
	Content string   `json:"content" jsonschema:"正文内容，不包含以#开头的标签内容，所有话题标签都用tags参数来生成和提供即可"`
	Video   string   `json:"video" jsonschema:"本地视频绝对路径（仅支持单个视频文件，如:/Users/user/video.mp4）"`
	Tags    []string `json:"tags,omitempty" jsonschema:"话题标签列表（可选参数），如 [美食, 旅行, 生活]"`
}

// SearchFeedsArgs 搜索内容的参数
type SearchFeedsArgs struct {
	Keyword string       `json:"keyword" jsonschema:"搜索关键词"`
	Filters FilterOption `json:"filters,omitempty" jsonschema:"筛选选项"`
}

// FilterOption 筛选选项结构体
type FilterOption struct {
	SortBy      string `json:"sort_by,omitempty" jsonschema:"排序依据: 综合|最新|最多点赞|最多评论|最多收藏,默认为'综合'"`
	NoteType    string `json:"note_type,omitempty" jsonschema:"笔记类型: 不限|视频|图文,默认为'不限'"`
	PublishTime string `json:"publish_time,omitempty" jsonschema:"发布时间: 不限|一天内|一周内|半年内,默认为'不限'"`
	SearchScope string `json:"search_scope,omitempty" jsonschema:"搜索范围: 不限|已看过|未看过|已关注,默认为'不限'"`
	Location    string `json:"location,omitempty" jsonschema:"位置距离: 不限|同城|附近,默认为'不限'"`
}

// FeedDetailArgs 获取Feed详情的参数
type FeedDetailArgs struct {
	FeedID           string `json:"feed_id" jsonschema:"小红书笔记ID，从Feed列表获取"`
	XsecToken        string `json:"xsec_token" jsonschema:"访问令牌，从Feed列表的xsecToken字段获取"`
	LoadAllComments  bool   `json:"load_all_comments,omitempty" jsonschema:"是否加载全部评论。false仅返回前10条一级评论（默认），true滚动加载更多评论"`
	Limit            int    `json:"limit,omitempty" jsonschema:"【仅当load_all_comments为true时生效】限制加载的一级评论数量。例如20表示最多加载20条，默认20"`
	ClickMoreReplies bool   `json:"click_more_replies,omitempty" jsonschema:"【仅当load_all_comments为true时生效】是否展开二级回复。true展开子评论，false不展开（默认）"`
	ReplyLimit       int    `json:"reply_limit,omitempty" jsonschema:"【仅当click_more_replies为true时生效】跳过回复数过多的评论。例如10表示跳过超过10条回复的，默认10"`
	ScrollSpeed      string `json:"scroll_speed,omitempty" jsonschema:"【仅当load_all_comments为true时生效】滚动速度slow慢速、normal正常、fast快速"`
}

// UserProfileArgs 获取用户主页的参数
type UserProfileArgs struct {
	UserID    string `json:"user_id" jsonschema:"小红书用户ID，从Feed列表获取"`
	XsecToken string `json:"xsec_token" jsonschema:"访问令牌，从Feed列表的xsecToken字段获取"`
}

// PostCommentArgs 发表评论的参数
type PostCommentArgs struct {
	FeedID    string `json:"feed_id" jsonschema:"小红书笔记ID，从Feed列表获取"`
	XsecToken string `json:"xsec_token" jsonschema:"访问令牌，从Feed列表的xsecToken字段获取"`
	Content   string `json:"content" jsonschema:"评论内容"`
}

// ReplyCommentArgs 回复评论的参数
type ReplyCommentArgs struct {
	FeedID    string `json:"feed_id" jsonschema:"小红书笔记ID，从Feed列表获取"`
	XsecToken string `json:"xsec_token" jsonschema:"访问令牌，从Feed列表的xsecToken字段获取"`
	CommentID string `json:"comment_id,omitempty" jsonschema:"目标评论ID，从评论列表获取"`
	UserID    string `json:"user_id,omitempty" jsonschema:"目标评论用户ID，从评论列表获取"`
	Content   string `json:"content" jsonschema:"回复内容"`
}

// LikeFeedArgs 点赞参数
type LikeFeedArgs struct {
	FeedID    string `json:"feed_id" jsonschema:"小红书笔记ID，从Feed列表获取"`
	XsecToken string `json:"xsec_token" jsonschema:"访问令牌，从Feed列表的xsecToken字段获取"`
	Unlike    bool   `json:"unlike,omitempty" jsonschema:"是否取消点赞，true为取消点赞，false或未设置则为点赞"`
}

// FavoriteFeedArgs 收藏参数
type FavoriteFeedArgs struct {
	FeedID     string `json:"feed_id" jsonschema:"小红书笔记ID，从Feed列表获取"`
	XsecToken  string `json:"xsec_token" jsonschema:"访问令牌，从Feed列表的xsecToken字段获取"`
	Unfavorite bool   `json:"unfavorite,omitempty" jsonschema:"是否取消收藏，true为取消收藏，false或未设置则为收藏"`
}

// InitMCPServer 初始化 MCP Server
func InitMCPServer(appServer *AppServer) *mcp.Server {
	// 创建 MCP Server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "xiaohongshu-mcp",
			Version: "2.0.0",
		},
		nil,
	)

	// 注册所有工具
	registerTools(server, appServer)

	logrus.Info("MCP Server initialized with official SDK")

	return server
}

func withPanicRecovery[T any](
	toolName string,
	handler func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error),
) func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error) {

	return func(ctx context.Context, req *mcp.CallToolRequest, args T) (result *mcp.CallToolResult, resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(logrus.Fields{
					"tool":  toolName,
					"panic": r,
				}).Error("Tool handler panicked")

				logrus.Errorf("Stack trace:\n%s", debug.Stack())

				result = &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{
							Text: fmt.Sprintf("工具 %s 执行时发生内部错误: %v\n\n请查看服务端日志获取详细信息。", toolName, r),
						},
					},
					IsError: true,
				}
				resp = nil
				err = nil
			}
		}()

		return handler(ctx, req, args)
	}
}

// registerTools 注册所有 MCP 工具
func registerTools(server *mcp.Server, appServer *AppServer) {
	// 工具 1: 检查登录状态
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "check_login_status",
			Description: "检查小红书登录状态",
		},
		withPanicRecovery("check_login_status", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleCheckLoginStatus(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 2: 获取登录二维码
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_login_qrcode",
			Description: "获取登录二维码（返回 Base64 图片和超时时间）",
		},
		withPanicRecovery("get_login_qrcode", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleGetLoginQrcode(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 3: 删除 cookies（登录重置）
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "delete_cookies",
			Description: "删除 cookies 文件，重置登录状态。删除后需要重新登录。",
		},
		withPanicRecovery("delete_cookies", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleDeleteCookies(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 4: 发布内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "publish_content",
			Description: "发布小红书图文内容",
		},
		withPanicRecovery("publish_content", func(ctx context.Context, req *mcp.CallToolRequest, args PublishContentArgs) (*mcp.CallToolResult, any, error) {
			// 转换参数格式到现有的 handler
			argsMap := map[string]interface{}{
				"title":   args.Title,
				"content": args.Content,
				"images":  convertStringsToInterfaces(args.Images),
				"tags":    convertStringsToInterfaces(args.Tags),
			}
			result := appServer.handlePublishContent(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 5: 获取Feed列表
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_feeds",
			Description: "获取首页 Feeds 列表",
		},
		withPanicRecovery("list_feeds", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleListFeeds(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 6: 搜索内容
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_feeds",
			Description: "搜索小红书内容（需要已登录）",
		},
		withPanicRecovery("search_feeds", func(ctx context.Context, req *mcp.CallToolRequest, args SearchFeedsArgs) (*mcp.CallToolResult, any, error) {
			result := appServer.handleSearchFeeds(ctx, args)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 7: 获取Feed详情
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_feed_detail",
			Description: "获取小红书笔记详情，返回笔记内容、图片、作者信息、互动数据（点赞/收藏/分享数）及评论列表。默认返回前10条一级评论，如需更多评论请设置load_all_comments=true",
		},
		withPanicRecovery("get_feed_detail", func(ctx context.Context, req *mcp.CallToolRequest, args FeedDetailArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"feed_id":           args.FeedID,
				"xsec_token":        args.XsecToken,
				"load_all_comments": args.LoadAllComments,
			}

			// 只有当 load_all_comments=true 时，才处理其他参数
			if args.LoadAllComments {
				argsMap["click_more_replies"] = args.ClickMoreReplies

				// 设置评论数量限制，默认20
				limit := args.Limit
				if limit <= 0 {
					limit = 20
				}
				argsMap["max_comment_items"] = limit

				// 设置回复数量阈值，默认10
				replyLimit := args.ReplyLimit
				if replyLimit <= 0 {
					replyLimit = 10
				}
				argsMap["max_replies_threshold"] = replyLimit

				if args.ScrollSpeed != "" {
					argsMap["scroll_speed"] = args.ScrollSpeed
				}
			}

			result := appServer.handleGetFeedDetail(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 8: 获取用户主页
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "user_profile",
			Description: "获取指定的小红书用户主页，返回用户基本信息，关注、粉丝、获赞量及其笔记内容",
		},
		withPanicRecovery("user_profile", func(ctx context.Context, req *mcp.CallToolRequest, args UserProfileArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"user_id":    args.UserID,
				"xsec_token": args.XsecToken,
			}
			result := appServer.handleUserProfile(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 9: 发表评论
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "post_comment_to_feed",
			Description: "发表评论到小红书笔记",
		},
		withPanicRecovery("post_comment_to_feed", func(ctx context.Context, req *mcp.CallToolRequest, args PostCommentArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"feed_id":    args.FeedID,
				"xsec_token": args.XsecToken,
				"content":    args.Content,
			}
			result := appServer.handlePostComment(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 10: 回复评论
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "reply_comment_in_feed",
			Description: "回复小红书笔记下的指定评论",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, args ReplyCommentArgs) (*mcp.CallToolResult, any, error) {
			if args.CommentID == "" && args.UserID == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 comment_id 或 user_id"}},
				}, nil, nil
			}

			argsMap := map[string]interface{}{
				"feed_id":    args.FeedID,
				"xsec_token": args.XsecToken,
				"comment_id": args.CommentID,
				"user_id":    args.UserID,
				"content":    args.Content,
			}
			result := appServer.handleReplyComment(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		},
	)

	// 工具 11: 发布视频（仅本地文件）
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "publish_with_video",
			Description: "发布小红书视频内容（仅支持本地单个视频文件）",
		},
		withPanicRecovery("publish_with_video", func(ctx context.Context, req *mcp.CallToolRequest, args PublishVideoArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"title":   args.Title,
				"content": args.Content,
				"video":   args.Video,
				"tags":    convertStringsToInterfaces(args.Tags),
			}
			result := appServer.handlePublishVideo(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 12: 点赞笔记
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "like_feed",
			Description: "为指定笔记点赞或取消点赞（如已点赞将跳过点赞，如未点赞将跳过取消点赞）",
		},
		withPanicRecovery("like_feed", func(ctx context.Context, req *mcp.CallToolRequest, args LikeFeedArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"feed_id":    args.FeedID,
				"xsec_token": args.XsecToken,
				"unlike":     args.Unlike,
			}
			result := appServer.handleLikeFeed(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 13: 收藏笔记
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "favorite_feed",
			Description: "收藏指定笔记或取消收藏（如已收藏将跳过收藏，如未收藏将跳过取消收藏）",
		},
		withPanicRecovery("favorite_feed", func(ctx context.Context, req *mcp.CallToolRequest, args FavoriteFeedArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"feed_id":    args.FeedID,
				"xsec_token": args.XsecToken,
				"unfavorite": args.Unfavorite,
			}
			result := appServer.handleFavoriteFeed(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	logrus.Infof("Registered %d MCP tools", 12)
}

// convertToMCPResult 将自定义的 MCPToolResult 转换为官方 SDK 的格式
func convertToMCPResult(result *MCPToolResult) *mcp.CallToolResult {
	var contents []mcp.Content
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			contents = append(contents, &mcp.TextContent{Text: c.Text})
		case "image":
			// 解码 base64 字符串为 []byte
			imageData, err := base64.StdEncoding.DecodeString(c.Data)
			if err != nil {
				logrus.WithError(err).Error("Failed to decode base64 image data")
				// 如果解码失败，添加错误文本
				contents = append(contents, &mcp.TextContent{
					Text: "图片数据解码失败: " + err.Error(),
				})
			} else {
				contents = append(contents, &mcp.ImageContent{
					Data:     imageData,
					MIMEType: c.MimeType,
				})
			}
		}
	}

	return &mcp.CallToolResult{
		Content: contents,
		IsError: result.IsError,
	}
}

// convertStringsToInterfaces 辅助函数：将 []string 转换为 []interface{}
func convertStringsToInterfaces(strs []string) []interface{} {
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
