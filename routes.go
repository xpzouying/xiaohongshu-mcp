package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

const maxRESTRequestBody = 1 << 20

func withRESTAccountRouting(manager *account.Manager, kind account.OperationKind, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if manager == nil {
			respondError(c, http.StatusInternalServerError, string(account.CodeInternalError), "账号管理器未初始化", nil)
			return
		}
		requestedID, err := restAccountID(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
			return
		}
		_, err = manager.WithAccount(c.Request.Context(), requestedID, kind, func(ctx context.Context, selected account.Account, browser account.Browser) error {
			ctx = context.WithValue(ctx, accountContextKey{}, selected.ID)
			ctx = context.WithValue(ctx, accountBrowserContextKey{}, browser)
			c.Request = c.Request.WithContext(ctx)
			c.Set("account", selected.ID)
			handler(c)
			return nil
		})
		if err != nil {
			respondRESTAccountError(c, err)
		}
	}
}

func restAccountID(c *gin.Context) (string, error) {
	if id := c.Query("account_id"); id != "" {
		return id, nil
	}
	if c.Request.Body == nil || c.Request.Method == http.MethodGet {
		return "", nil
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRESTRequestBody)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	var payload struct {
		AccountID string `json:"account_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	return payload.AccountID, nil
}

func respondRESTAccountError(c *gin.Context, err error) {
	code := account.ErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case account.CodeInvalidAccountID, account.CodeAccountRequired:
		status = http.StatusBadRequest
	case account.CodeAccountNotFound:
		status = http.StatusNotFound
	case account.CodeAccountLoginRequired:
		status = http.StatusUnauthorized
	case account.CodeAccountBusy:
		status = http.StatusTooManyRequests
	case account.CodeOperationCanceled:
		status = http.StatusRequestTimeout
	case account.CodeAccountPaused, account.CodeAccountRiskHold, account.CodeAccountDisabled:
		status = http.StatusConflict
	}
	if code == "" {
		code = account.CodeInternalError
	}
	respondError(c, status, string(code), "账号执行失败", err.Error())
}

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
		api.POST("/publish", withRESTAccountRouting(appServer.accountManager, account.OperationWrite, appServer.publishHandler))
		api.POST("/publish_video", withRESTAccountRouting(appServer.accountManager, account.OperationWrite, appServer.publishVideoHandler))
		api.GET("/feeds/list", withRESTAccountRouting(appServer.accountManager, account.OperationRead, appServer.listFeedsHandler))
		api.GET("/feeds/search", withRESTAccountRouting(appServer.accountManager, account.OperationRead, appServer.searchFeedsHandler))
		api.POST("/feeds/search", withRESTAccountRouting(appServer.accountManager, account.OperationRead, appServer.searchFeedsHandler))
		api.POST("/feeds/detail", withRESTAccountRouting(appServer.accountManager, account.OperationRead, appServer.getFeedDetailHandler))
		api.POST("/user/profile", withRESTAccountRouting(appServer.accountManager, account.OperationRead, appServer.userProfileHandler))
		api.POST("/feeds/comment", withRESTAccountRouting(appServer.accountManager, account.OperationWrite, appServer.postCommentHandler))
		api.POST("/feeds/comment/reply", withRESTAccountRouting(appServer.accountManager, account.OperationWrite, appServer.replyCommentHandler))
		api.POST("/feeds/like", withRESTAccountRouting(appServer.accountManager, account.OperationWrite, appServer.likeFeedHandler))
		api.POST("/feeds/favorite", withRESTAccountRouting(appServer.accountManager, account.OperationWrite, appServer.favoriteFeedHandler))
		api.GET("/user/me", withRESTAccountRouting(appServer.accountManager, account.OperationRead, appServer.myProfileHandler))
	}

	return router
}
