package main

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/ratelimit"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// getRateLimitInfo 从 gin.Context 获取限速信息（由 checkRateLimit 设置）
func getRateLimitInfo(c *gin.Context) *ratelimit.Info {
	if v, exists := c.Get("rate_limit"); exists {
		if info, ok := v.(*ratelimit.Info); ok {
			return info
		}
	}
	return nil
}

// respondError 返回错误响应
func respondError(c *gin.Context, statusCode int, code, message string, details any) {
	response := ErrorResponse{
		Error:     message,
		Code:      code,
		Details:   details,
		RateLimit: getRateLimitInfo(c),
	}

	logrus.Errorf("%s %s %s %d", c.Request.Method, c.Request.URL.Path,
		c.GetString("account"), statusCode)

	c.JSON(statusCode, response)
}

// respondSuccess 返回成功响应
func respondSuccess(c *gin.Context, data any, message string) {
	response := SuccessResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		RateLimit: getRateLimitInfo(c),
	}

	logrus.Infof("%s %s %s %d", c.Request.Method, c.Request.URL.Path,
		c.GetString("account"), http.StatusOK)

	c.JSON(http.StatusOK, response)
}

// checkRateLimit 检查速率限制，如果超限则返回 429 并阻止执行。
// force 参数从查询参数 ?force_rate_limit=true 读取。
// 注意：必须在 handler 中尽早调用，调用后继续执行需手动 Record。
func (s *AppServer) checkRateLimit(c *gin.Context) (canProceed bool) {
	force := c.Query("force_rate_limit") == "true"
	if s.rateLimiter == nil {
		return true
	}

	info, canProceed, err := s.rateLimiter.Check(force)
	if err != nil {
		logrus.Errorf("rate limiter check error: %v", err)
		c.Set("rate_limit", &info)
		return true
	}

	c.Set("rate_limit", &info)

	if !canProceed {
		// ⚠️ 超限：提醒但不拦截，放行继续执行
		logrus.Warnf("[ratelimit] ⚠️ 操作超限：%s", info.Warning)
		c.Header("X-RateLimit-Warning", info.Warning)
		// force=false 但超限时，放弃冷却等待直接放行
		return true
	}

	// 应用冷却延迟（按当前使用率自动调整）
	if info.Used > 0 {
		wait := s.rateLimiter.WaitDuration(info)
		logrus.Infof("[ratelimit] cooldown %v before execution", wait)
		time.Sleep(wait)
	}

	// 在可执行范围内，按 70% 概率消耗操作额度
	// （保守策略：不是每步都计，有些是页面浏览）
	if info.Used < info.Limit && rand.Float64() < 0.7 {
		s.rateLimiter.Record()
	}

	logrus.Infof("[ratelimit] %s %s - %s", c.Request.Method, c.Request.URL.Path, s.rateLimiter.String())
	return true
}

// checkRateLimitResult 非 HTTP 环境的速率限制结果
type checkRateLimitResult struct {
	CanProceed bool
	Info       ratelimit.Info
}

// checkRateLimitInternal 通用速率限制检查（供 MCP handler 使用）
// 无 gin.Context 时调此方法。如需强制越过，传 force=true。
func (s *AppServer) checkRateLimitInternal(force bool) checkRateLimitResult {
	if s.rateLimiter == nil {
		return checkRateLimitResult{CanProceed: true}
	}

	info, canProceed, err := s.rateLimiter.Check(force)
	if err != nil {
		logrus.Errorf("rate limiter check error: %v", err)
		return checkRateLimitResult{CanProceed: true, Info: info}
	}

	if !canProceed {
		// ⚠️ 超限：提醒但放行，由上层决定是否通知用户
		logrus.Warnf("[ratelimit] ⚠️ 操作超限：%s - 仍放行", info.Warning)
		return checkRateLimitResult{CanProceed: true, Info: info}
	}

	// 应用冷却延迟
	if info.Used > 0 {
		wait := s.rateLimiter.WaitDuration(info)
		logrus.Infof("[ratelimit] MCP cooldown %v", wait)
		time.Sleep(wait)
	}

	// 按 70% 概率消耗额度
	if info.Used < info.Limit && rand.Float64() < 0.7 {
		s.rateLimiter.Record()
	}

	logrus.Infof("[ratelimit] MCP - %s", s.rateLimiter.String())
	return checkRateLimitResult{CanProceed: true, Info: info}
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

// getLoginQrcodeHandler 处理 [GET /api/v1/login/qrcode] 请求。
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

// publishHandler 发布内容
func (s *AppServer) publishHandler(c *gin.Context) {
	if !s.checkRateLimit(c) {
		return
	}
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 执行发布
	result, err := s.xiaohongshuService.PublishContent(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_FAILED",
			"发布失败", err.Error())
		return
	}

	respondSuccess(c, result, "发布成功")
}

// publishVideoHandler 发布视频内容
func (s *AppServer) publishVideoHandler(c *gin.Context) {
	if !s.checkRateLimit(c) {
		return
	}
	var req PublishVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	// 执行视频发布
	result, err := s.xiaohongshuService.PublishVideo(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_VIDEO_FAILED",
			"视频发布失败", err.Error())
		return
	}

	respondSuccess(c, result, "视频发布成功")
}

// listFeedsHandler 获取Feeds列表
func (s *AppServer) listFeedsHandler(c *gin.Context) {
	if !s.checkRateLimit(c) {
		return
	}
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
	if !s.checkRateLimit(c) {
		return
	}
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
	if !s.checkRateLimit(c) {
		return
	}
	var req FeedDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	var result *FeedDetailResponse
	var err error

	if req.CommentConfig != nil {
		// 使用配置参数
		config := xiaohongshu.CommentLoadConfig{
			ClickMoreReplies:    req.CommentConfig.ClickMoreReplies,
			MaxRepliesThreshold: req.CommentConfig.MaxRepliesThreshold,
			MaxCommentItems:     req.CommentConfig.MaxCommentItems,
			ScrollSpeed:         req.CommentConfig.ScrollSpeed,
		}
		result, err = s.xiaohongshuService.GetFeedDetailWithConfig(c.Request.Context(), req.FeedID, req.XsecToken, req.LoadAllComments, config)
	} else {
		// 使用默认配置
		result, err = s.xiaohongshuService.GetFeedDetail(c.Request.Context(), req.FeedID, req.XsecToken, req.LoadAllComments)
	}

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
	if !s.checkRateLimit(c) {
		return
	}
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
	if !s.checkRateLimit(c) {
		return
	}
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

// replyCommentHandler 回复指定评论
func (s *AppServer) replyCommentHandler(c *gin.Context) {
	if !s.checkRateLimit(c) {
		return
	}
	var req ReplyCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	result, err := s.xiaohongshuService.ReplyCommentToFeed(c.Request.Context(), req.FeedID, req.XsecToken, req.CommentID, req.UserID, req.Content)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "REPLY_COMMENT_FAILED",
			"回复评论失败", err.Error())
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
