package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setupRoutes 设置路由配置
func setupRoutes(appServer *AppServer) *gin.Engine {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 添加中间件
	router.Use(errorHandlingMiddleware())
	router.Use(corsMiddleware())

	// 健康检查
	router.GET("/health", healthHandler)

	// OAuth discovery endpoints — return JSON 404 instead of plain text 404.
	//
	// Background: Claude Code's MCP HTTP client probes a number of OAuth
	// .well-known endpoints (per the MCP authorization spec) on every
	// reconnect. When these paths return Gin's default plain-text
	// "404 page not found", the client tries to parse the body as JSON,
	// fails, and ends up stuck in a "needs authentication" state with no
	// OAuth metadata to act on. The MCP endpoint still works, but the
	// auth state machine on the client side never recovers without a
	// process restart.
	//
	// Returning a JSON 404 here lets the client cleanly conclude "no auth
	// metadata, no auth required" and fall back to anonymous mode.
	noAuthRequired := func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":             "not_found",
			"error_description": "This MCP server does not require authentication",
		})
	}
	router.GET("/.well-known/oauth-protected-resource", noAuthRequired)
	router.GET("/.well-known/oauth-protected-resource/*path", noAuthRequired)
	router.GET("/.well-known/oauth-authorization-server", noAuthRequired)
	router.GET("/.well-known/oauth-authorization-server/*path", noAuthRequired)
	router.GET("/.well-known/openid-configuration", noAuthRequired)
	router.GET("/.well-known/openid-configuration/*path", noAuthRequired)
	router.GET("/mcp/.well-known/*path", noAuthRequired)
	router.POST("/register", noAuthRequired)

	// MCP 端点 - 使用官方 SDK 的 Streamable HTTP Handler
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return appServer.mcpServer
		},
		&mcp.StreamableHTTPOptions{
			JSONResponse: true, // 支持 JSON 响应
		},
	)
	router.Any("/mcp", gin.WrapH(mcpHandler))
	router.Any("/mcp/*path", gin.WrapH(mcpHandler))

	// API 路由组
	api := router.Group("/api/v1")
	{
		api.GET("/login/status", appServer.checkLoginStatusHandler)
		api.GET("/login/qrcode", appServer.getLoginQrcodeHandler)
		api.DELETE("/login/cookies", appServer.deleteCookiesHandler)
		api.POST("/publish", appServer.publishHandler)
		api.POST("/publish_video", appServer.publishVideoHandler)
		api.GET("/feeds/list", appServer.listFeedsHandler)
		api.GET("/feeds/search", appServer.searchFeedsHandler)
		api.POST("/feeds/search", appServer.searchFeedsHandler)
		api.POST("/feeds/detail", appServer.getFeedDetailHandler)
		api.POST("/user/profile", appServer.userProfileHandler)
		api.POST("/feeds/comment", appServer.postCommentHandler)
		api.POST("/feeds/comment/reply", appServer.replyCommentHandler)
		api.GET("/user/me", appServer.myProfileHandler)
	}

	return router
}
