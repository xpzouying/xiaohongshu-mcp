//go:build stdio
// +build stdio

package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func main() {
	var (
		headless bool
		binPath  string
		port     string
		useStdio bool
	)
	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.StringVar(&port, "port", ":18060", "HTTP 模式端口")
	flag.BoolVar(&useStdio, "stdio", false, "使用 stdio 模式（默认为 HTTP 模式）")
	flag.Parse()

	if len(binPath) == 0 {
		binPath = os.Getenv("ROD_BROWSER_BIN")
	}
	if binPath != "" {
		logrus.Infof("using browser binary: %s", binPath)
	} else {
		logrus.Infof("browser binary is not configured; rod will auto-detect or download Chromium")
	}

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	if useStdio {
		// stdio 模式
		logrus.Info("启动 stdio 模式")
		runStdioServer(xiaohongshuService)
	} else {
		// HTTP 模式
		logrus.Infof("启动 HTTP 模式，端口: %s", port)
		appServer := NewAppServer(xiaohongshuService)
		if err := appServer.Start(port); err != nil {
			logrus.Fatalf("failed to run server: %v", err)
		}
	}
}

// runStdioServer 运行 stdio 模式的 MCP 服务器
func runStdioServer(service *XiaohongshuService) {
	// 重要：在 stdio 模式下，stdout 只能用于 MCP JSONRPC 消息
	// rod 浏览器启动时会输出进度日志到 stdout，需要重定向到 stderr
	// 保存原始 stdout，用于 MCP 协议
	originalStdout := os.Stdout
	// 将 stdout 重定向到 stderr，这样浏览器进度日志不会干扰 MCP
	os.Stdout = os.Stderr

	// 设置日志输出到文件（同时保留 stderr）
	// 优先使用环境变量 LOG_DIR，否则使用工作目录下的 logs 子目录
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		// 获取当前可执行文件所在目录，确保日志目录在正确位置
		execPath, err := os.Executable()
		if err == nil {
			execDir := filepath.Dir(execPath)
			logDir = filepath.Join(execDir, "logs")
		} else {
			// fallback 到当前工作目录
			logDir = "logs"
		}
	}
	if err := os.MkdirAll(logDir, 0755); err == nil {
		logFilePath := filepath.Join(logDir, "xhs-mcp.log")
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			// 同时输出到文件和 stderr
			multiWriter := io.MultiWriter(os.Stderr, logFile)
			logrus.SetOutput(multiWriter)
			// 同时配置 slog（发布流程使用 slog）
			slog.SetDefault(slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelDebug})))
			logrus.Infof("日志文件路径: %s", logFilePath)
		}
	}
	logrus.SetLevel(logrus.DebugLevel) // 设置调试级别日志

	// 恢复 stdout 用于 MCP 协议
	os.Stdout = originalStdout

	// 创建 MCP Server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "xiaohongshu-mcp",
			Version: "2.0.0",
		},
		nil,
	)

	// 创建 AppServer 用于复用现有的 handler
	appServer := &AppServer{
		xiaohongshuService: service,
		mcpServer:          server,
	}

	// 注册所有工具
	registerToolsForStdio(server, appServer)

	logrus.Info("MCP Server initialized with official SDK (stdio mode)")
	logrus.Info("Registered 13 MCP tools")

	// 运行 stdio 服务器
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logrus.Fatalf("stdio server error: %v", err)
	}
}

// registerToolsForStdio 为 stdio 模式注册工具（复用现有的 handler）
func registerToolsForStdio(server *mcp.Server, appServer *AppServer) {
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

	// 工具 3: 删除 cookies
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
			argsMap := map[string]interface{}{
				"title":       args.Title,
				"content":     args.Content,
				"images":      convertStringsToInterfaces(args.Images),
				"tags":        convertStringsToInterfaces(args.Tags),
				"schedule_at": args.ScheduleAt,
				"is_original": args.IsOriginal,
				"visibility":  args.Visibility,
				"products":    convertStringsToInterfaces(args.Products),
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
			Description: "获取小红书笔记详情，返回笔记内容、图片、作者信息、互动数据（点赞/收藏/分享数）及评论列表",
		},
		withPanicRecovery("get_feed_detail", func(ctx context.Context, req *mcp.CallToolRequest, args FeedDetailArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"feed_id":           args.FeedID,
				"xsec_token":        args.XsecToken,
				"load_all_comments": args.LoadAllComments,
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
		withPanicRecovery("reply_comment_in_feed", func(ctx context.Context, req *mcp.CallToolRequest, args ReplyCommentArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"feed_id":    args.FeedID,
				"xsec_token": args.XsecToken,
				"comment_id": args.CommentID,
				"user_id":    args.UserID,
				"content":    args.Content,
			}
			result := appServer.handleReplyComment(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 11: 发布视频
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "publish_with_video",
			Description: "发布小红书视频内容（仅支持本地单个视频文件）",
		},
		withPanicRecovery("publish_with_video", func(ctx context.Context, req *mcp.CallToolRequest, args PublishVideoArgs) (*mcp.CallToolResult, any, error) {
			argsMap := map[string]interface{}{
				"title":       args.Title,
				"content":     args.Content,
				"video":       args.Video,
				"tags":        convertStringsToInterfaces(args.Tags),
				"schedule_at": args.ScheduleAt,
				"visibility":  args.Visibility,
				"products":    convertStringsToInterfaces(args.Products),
			}
			result := appServer.handlePublishVideo(ctx, argsMap)
			return convertToMCPResult(result), nil, nil
		}),
	)

	// 工具 12: 点赞笔记
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "like_feed",
			Description: "为指定笔记点赞或取消点赞",
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
			Description: "收藏指定笔记或取消收藏",
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
}