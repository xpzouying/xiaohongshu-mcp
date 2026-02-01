package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// respondError 返回错误响应
func respondError(c *gin.Context, statusCode int, code, message string, details any) {
	response := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}

	logrus.Errorf("%s %s %s %d", c.Request.Method, c.Request.URL.Path,
		c.GetString("account"), statusCode)

	c.JSON(statusCode, response)
}

// respondSuccess 返回成功响应
func respondSuccess(c *gin.Context, data any, message string) {
	response := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}

	// 记录完整的响应内容，确保message字段存在
	if respBytes, err := json.Marshal(response); err == nil {
		logrus.Infof("发送成功响应: %s", string(respBytes))
	}

	logrus.Infof("%s %s %s %d", c.Request.Method, c.Request.URL.Path,
		c.GetString("account"), http.StatusOK)

	c.JSON(http.StatusOK, response)
}

// checkLoginStatusHandler 检查登录状态
func (s *AppServer) checkLoginStatusHandler(c *gin.Context) {
	status, err := s.xiaohongshuService.CheckLoginStatus(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED",
			"检查登录状态失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, status, "检查登录状态成功")
}

// getLoginQrcodeHandler 处理 [GET /api/login/qrcode] 请求。
// 用于生成并返回登录二维码（Base64 图片 + 超时时间），供前端展示给用户扫码登录。
func (s *AppServer) getLoginQrcodeHandler(c *gin.Context) {
	result, err := s.xiaohongshuService.GetLoginQrcode(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED",
			"获取登录二维码失败", err.Error())
		return
	}

	respondSuccess(c, result, "获取登录二维码成功")
}

// deleteCookiesHandler 删除 cookies，重置登录状态
func (s *AppServer) deleteCookiesHandler(c *gin.Context) {
	err := s.xiaohongshuService.DeleteCookies(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_COOKIES_FAILED",
			"删除 cookies 失败", err.Error())
		return
	}

	cookiePath := cookies.GetCookiesFilePath()
	respondSuccess(c, map[string]interface{}{
		"cookie_path": cookiePath,
		"message":     "Cookies 已成功删除，登录状态已重置。下次操作时需要重新登录。",
	}, "删除 cookies 成功")
}

// publishHandler 发布内容（异步模式）
//
// 流程：
//  1. 立即返回 202 Accepted，告知客户端请求已接受
//  2. 后台异步执行发布操作
//  3. 发布完成后通过 webhook 通知结果
//
// 注意：
//   - 必须提供 webhook 参数，否则无法获知发布结果
//   - 客户端不会等待发布完成，避免超时问题
func (s *AppServer) publishHandler(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 验证 webhook 参数
	if req.Webhook == "" {
		respondError(c, http.StatusBadRequest, "WEBHOOK_REQUIRED",
			"异步发布模式需要提供 webhook 参数", "请在请求中添加 webhook URL 以接收发布结果")
		return
	}

	// 立即返回 202 Accepted
	c.JSON(http.StatusAccepted, SuccessResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":  "accepted",
			"message": "发布请求已接受，正在后台处理",
			"webhook": req.Webhook,
		},
		Message: "请求已接受，发布结果将通过 webhook 通知",
	})

	// 使用 channel 确保 goroutine 真正启动
	started := make(chan struct{})

	// 异步执行发布
	go func() {
		// 创建独立的 context，30 分钟超时（足够完成发布）
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// 通知主 goroutine：异步任务已启动
		close(started)

		logrus.Infof("开始异步发布内容，webhook: %s", req.Webhook)

		// 执行发布
		_, err := s.xiaohongshuService.PublishContent(ctx, &req)
		if err != nil {
			logrus.Errorf("异步发布失败: %v", err)
			// 发送失败通知到 webhook
			s.sendPublishErrorWebhook(req.Webhook, err.Error(), "publish_content")
			return
		}

		logrus.Infof("异步发布成功，准备发送 webhook")
		// webhook 发送已在 service 层处理
	}()

	// 等待异步任务真正启动（最多等待 100ms）
	select {
	case <-started:
		// 任务已启动
	case <-time.After(100 * time.Millisecond):
		// 超时保护
		logrus.Warn("等待异步任务启动超时")
	}
}

// saveAndExitHandler 暂存内容并离开（异步模式）
//
// 流程：
//  1. 立即返回 202 Accepted
//  2. 后台异步执行暂存操作
//  3. 完成后通过 webhook 通知结果
func (s *AppServer) saveAndExitHandler(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 验证 webhook 参数
	if req.Webhook == "" {
		respondError(c, http.StatusBadRequest, "WEBHOOK_REQUIRED",
			"异步模式需要提供 webhook 参数", "请在请求中添加 webhook URL 以接收结果")
		return
	}

	// 立即返回 202 Accepted
	c.JSON(http.StatusAccepted, SuccessResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":  "accepted",
			"message": "暂存请求已接受，正在后台处理",
			"webhook": req.Webhook,
		},
		Message: "请求已接受，结果将通过 webhook 通知",
	})

	// 使用 channel 确保 goroutine 真正启动
	started := make(chan struct{})

	// 异步执行暂存
	go func() {
		// 创建独立的 context，30 分钟超时
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// 通知主 goroutine：异步任务已启动
		close(started)

		logrus.Infof("开始异步暂存内容，webhook: %s", req.Webhook)

		// 执行暂存
		_, err := s.xiaohongshuService.SaveAndExit(ctx, &req)
		if err != nil {
			logrus.Errorf("异步暂存失败: %v", err)
			// 发送失败通知到 webhook
			// 发送失败通知到 webhook
			s.sendPublishErrorWebhook(req.Webhook, err.Error(), "save_and_exit")
			return
		}

		logrus.Infof("异步暂存成功，准备发送 webhook")
		// webhook 发送已在 service 层处理
	}()

	// 等待异步任务真正启动
	select {
	case <-started:
		// 任务已启动
	case <-time.After(100 * time.Millisecond):
		logrus.Warn("等待异步任务启动超时")
	}
}

// publishVideoHandler 发布视频内容（异步模式）
//
// 流程：
//  1. 立即返回 202 Accepted，告知客户端请求已接受
//  2. 后台异步执行视频发布操作
//  3. 发布完成后通过 webhook 通知结果
//
// 注意：
//   - 必须提供 webhook 参数，否则无法获知发布结果
//   - 视频上传可能需要较长时间，异步处理避免超时
func (s *AppServer) publishVideoHandler(c *gin.Context) {
	var req PublishVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 验证 webhook 参数
	if req.Webhook == "" {
		respondError(c, http.StatusBadRequest, "WEBHOOK_REQUIRED",
			"异步发布模式需要提供 webhook 参数", "请在请求中添加 webhook URL 以接收发布结果")
		return
	}

	// 立即返回 202 Accepted
	c.JSON(http.StatusAccepted, SuccessResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":  "accepted",
			"message": "视频发布请求已接受，正在后台处理",
			"webhook": req.Webhook,
		},
		Message: "请求已接受，发布结果将通过 webhook 通知",
	})

	// 使用 channel 确保 goroutine 真正启动
	started := make(chan struct{})

	// 异步执行视频发布
	go func() {
		// 创建独立的 context，60 分钟超时（视频上传可能需要更长时间）
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		// 通知主 goroutine：异步任务已启动
		close(started)

		logrus.Infof("开始异步发布视频，webhook: %s", req.Webhook)

		// 执行视频发布
		_, err := s.xiaohongshuService.PublishVideo(ctx, &req)
		if err != nil {
			logrus.Errorf("异步视频发布失败: %v", err)
			// 发送失败通知到 webhook
			s.sendPublishErrorWebhook(req.Webhook, err.Error(), "publish_video")
			return
		}

		logrus.Infof("异步视频发布成功，准备发送 webhook")
		// webhook 发送已在 service 层处理
	}()

	// 等待异步任务真正启动（最多等待 100ms）
	select {
	case <-started:
		// 任务已启动
	case <-time.After(100 * time.Millisecond):
		// 超时保护
		logrus.Warn("等待异步视频任务启动超时")
	}
}

// sendPublishErrorWebhook 发送发布失败的 webhook 通知
func (s *AppServer) sendPublishErrorWebhook(webhookURL string, errorMsg string, eventType string) {
	webhookSender := NewWebhookSender()

	errorPayload := map[string]interface{}{
		"error":     errorMsg,
		"status":    "failed",
		"timestamp": time.Now().Unix(),
		"event":     eventType,
	}

	webhookSender.SendAsync(webhookURL, errorPayload, nil, eventType+"_failed")
}

// listFeedsHandler 获取Feeds列表
func (s *AppServer) listFeedsHandler(c *gin.Context) {
	// 获取 Feeds 列表
	result, err := s.xiaohongshuService.ListFeeds(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_FEEDS_FAILED",
			"获取Feeds列表失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, result, "获取Feeds列表成功")
}

// searchFeedsHandler 搜索Feeds
func (s *AppServer) searchFeedsHandler(c *gin.Context) {
	var keyword string
	var filters xiaohongshu.FilterOption

	switch c.Request.Method {
	case http.MethodPost:
		// 对于POST请求，从JSON中获取keyword
		var searchReq SearchFeedsRequest
		if err := c.ShouldBindJSON(&searchReq); err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
				"请求参数错误", err.Error())
			return
		}
		keyword = searchReq.Keyword
		filters = searchReq.Filters
	default:
		keyword = c.Query("keyword")
	}

	if keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD",
			"缺少关键词参数", "keyword parameter is required")
		return
	}

	// 搜索 Feeds
	result, err := s.xiaohongshuService.SearchFeeds(c.Request.Context(), keyword, filters)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_FEEDS_FAILED",
			"搜索Feeds失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, result, "搜索Feeds成功")
}

// getFeedDetailHandler 获取Feed详情
func (s *AppServer) getFeedDetailHandler(c *gin.Context) {
	var req FeedDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 获取 Feed 详情
	result, err := s.xiaohongshuService.GetFeedDetail(c.Request.Context(), req.FeedID, req.XsecToken)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FEED_DETAIL_FAILED",
			"获取Feed详情失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, result, "获取Feed详情成功")
}

// userProfileHandler 用户主页
func (s *AppServer) userProfileHandler(c *gin.Context) {
	var req UserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 获取用户信息
	result, err := s.xiaohongshuService.UserProfile(c.Request.Context(), req.UserID, req.XsecToken)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_USER_PROFILE_FAILED",
			"获取用户主页失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, map[string]any{"data": result}, "result.Message")
}

// postCommentHandler 发表评论到Feed
func (s *AppServer) postCommentHandler(c *gin.Context) {
	var req PostCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 发表评论
	result, err := s.xiaohongshuService.PostCommentToFeed(c.Request.Context(), req.FeedID, req.XsecToken, req.Content)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "POST_COMMENT_FAILED",
			"发表评论失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, result, result.Message)
}

// healthHandler 健康检查
func healthHandler(c *gin.Context) {
	respondSuccess(c, map[string]any{
		"status":    "healthy",
		"service":   "xiaohongshu-mcp",
		"account":   "ai-report",
		"timestamp": "now",
	}, "服务正常")
}

// myProfileHandler 我的信息
func (s *AppServer) myProfileHandler(c *gin.Context) {
	// 获取当前登录用户信息
	result, err := s.xiaohongshuService.GetMyProfile(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MY_PROFILE_FAILED",
			"获取我的主页失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, map[string]any{"data": result}, "获取我的主页成功")
}

// myNoteListHandler 获取我的小红书笔记内容
//
// HTTP API 端点：POST /api/v1/user/my-note-list
//
// 请求参数：
//
//	{
//	  "user_id": "小红书用户ID"
//	}
//
// 返回数据：
//
//	{
//	  "success": true,
//	  "data": {
//	    "feeds": [...] // 笔记内容数组
//	  },
//	  "message": "获取我的小红书笔记内容成功"
//	}
func (s *AppServer) myNoteListHandler(c *gin.Context) {
	// 定义请求参数结构
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 调用 MyNoteList 服务方法
	feeds, err := s.xiaohongshuService.MyNoteList(c.Request.Context(), req.UserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MY_NOTE_LIST_FAILED",
			"获取我的小红书笔记内容失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, map[string]any{"feeds": feeds}, "获取我的小红书笔记内容成功")
}

// ownProfileHandler 获取我的小红书主页信息
//
// HTTP API 端点：POST /api/v1/user/own-profile
//
// 请求参数：
//
//	{
//	  "user_id": "小红书用户ID"
//	}
//
// 返回数据：
//
//	{
//	  "success": true,
//	  "data": {
//	    "userBasicInfo": {...},
//	    "interactions": [...],
//	    "feeds": []  // 空数组
//	  },
//	  "message": "获取我的小红书主页信息成功"
//	}
func (s *AppServer) ownProfileHandler(c *gin.Context) {
	// 定义请求参数结构
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 调用 OwnProfile 服务方法
	result, err := s.xiaohongshuService.OwnProfile(c.Request.Context(), req.UserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_OWN_PROFILE_FAILED",
			"获取我的小红书主页信息失败", err.Error())
		return
	}

	c.Set("account", "ai-report")
	respondSuccess(c, result, "获取我的小红书主页信息成功")
}
