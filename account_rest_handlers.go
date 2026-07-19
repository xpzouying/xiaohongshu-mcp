package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

type createAccountRESTRequest struct {
	ID          string `json:"id" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Owner       string `json:"owner"`
	Purpose     string `json:"purpose"`
}

type accountListRESTResponse struct {
	Accounts         []account.Account `json:"accounts"`
	DefaultAccountID *string           `json:"default_account_id"`
}

func registerAccountRESTRoutes(api *gin.RouterGroup, appServer *AppServer) {
	accounts := api.Group("/accounts")
	accounts.GET("", requireRESTScope(scopeRead, "accounts.list"), appServer.listAccountsRESTHandler)
	accounts.POST("", requireRESTScope(scopeAdmin, "accounts.create"), appServer.createAccountRESTHandler)
	accounts.POST("/quick_add", requireRESTScope(scopeAdmin, "accounts.quick_add"), appServer.quickAddAccountRESTHandler)
	accounts.DELETE("/:id", requireRESTScope(scopeAdmin, "accounts.remove"), appServer.removeAccountRESTHandler)
	accounts.PUT("/:id/default", requireRESTScope(scopeAdmin, "accounts.default"), appServer.setDefaultAccountRESTHandler)
	accounts.POST("/:id/login/qrcode", requireRESTScope(scopeAdmin, "accounts.login.qrcode"), appServer.accountLoginQRCodeRESTHandler)
	accounts.POST("/:id/login/status", requireRESTScope(scopeRead, "accounts.login.status"), appServer.accountLoginStatusRESTHandler)
	accounts.POST("/:id/sync_profile", requireRESTScope(scopeAdmin, "accounts.sync_profile"), appServer.syncAccountProfileRESTHandler)
	accounts.DELETE("/:id/login", requireRESTScope(scopeAdmin, "accounts.login.reset"), appServer.resetAccountLoginRESTHandler)
}

func (s *AppServer) listAccountsRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	accounts, err := s.accountTools.List(c.Request.Context())
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	defaultAccountID, err := s.accountTools.DefaultAccountID(c.Request.Context())
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, accountListRESTResponse{Accounts: accounts, DefaultAccountID: defaultAccountID}, "获取账号列表成功")
}

func (s *AppServer) createAccountRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	// 先预读全部 body 并校验大小，防止合法 JSON + 尾随空白绕过 MaxBytesReader
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxRESTRequestBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondError(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "请求体超过 1 MiB 限制", nil)
			return
		}
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "读取请求体失败", err.Error())
		return
	}
	var request createAccountRESTRequest
	if err := json.Unmarshal(body, &request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	created, err := s.accountTools.Create(c.Request.Context(), account.CreateAccountInput{
		ID: request.ID, DisplayName: request.DisplayName, Owner: request.Owner, Purpose: request.Purpose,
	})
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, created, "创建账号成功")
}

// quickAddAccountRESTHandler 快速添加账号：自动生成 ID，直接返回登录二维码。
// 用户无需手动填写账号 ID 和名称，扫码登录后系统自动获取昵称。
func (s *AppServer) quickAddAccountRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	// 自动生成唯一账号 ID（acct_ + 纳秒时间戳）
	id := "acct_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	created, err := s.accountTools.Create(c.Request.Context(), account.CreateAccountInput{
		ID:          id,
		DisplayName: "待登录", // 扫码成功后自动更新为真实昵称
	})
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	// 立即获取二维码
	result, err := s.accountTools.GetLoginQRCode(c.Request.Context(), id)
	if err != nil {
		// 二维码获取失败则删除刚创建的空账号
		_ = s.accountTools.Remove(c.Request.Context(), id)
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, gin.H{
		"account": created,
		"qrcode":  result,
	}, "扫码添加账号成功")
}

// syncAccountProfileRESTHandler 登录成功后自动读取当前小红书账号资料并更新展示名称
func (s *AppServer) syncAccountProfileRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	if s.accountManager == nil || s.xiaohongshuService == nil {
		respondError(c, http.StatusServiceUnavailable, string(account.CodeInternalError), "账号资料同步未初始化", nil)
		return
	}
	id := c.Param("id")
	status, err := s.accountTools.CheckLoginStatus(c.Request.Context(), id)
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	if !status.IsLoggedIn {
		respondError(c, http.StatusBadRequest, "NOT_LOGGED_IN", "账号尚未登录", nil)
		return
	}

	var profile *UserProfileResponse
	_, err = s.accountManager.WithAccount(c.Request.Context(), id, account.OperationRead,
		func(ctx context.Context, selected account.Account, browser account.Browser) error {
			ctx = context.WithValue(ctx, accountContextKey{}, selected.ID)
			ctx = context.WithValue(ctx, accountBrowserContextKey{}, browser)
			profile, err = s.xiaohongshuService.GetMyProfile(ctx)
			return err
		})
	if err != nil {
		respondRESTAccountError(c, err)
		return
	}
	nickname := strings.TrimSpace(profile.UserBasicInfo.Nickname)
	if nickname == "" {
		respondError(c, http.StatusBadGateway, "PROFILE_NAME_EMPTY", "未能读取小红书昵称，请稍后重试", nil)
		return
	}
	if err := s.accountTools.UpdateDisplayName(c.Request.Context(), id, nickname); err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, gin.H{
		"account_id":   id,
		"display_name": nickname,
		"red_id":       profile.UserBasicInfo.RedId,
		"avatar":       profile.UserBasicInfo.Imageb,
		"is_logged_in": true,
	}, "同步账号信息成功")
}

func (s *AppServer) removeAccountRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	id := c.Param("id")
	if err := s.accountTools.Remove(c.Request.Context(), id); err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, map[string]string{"account_id": id}, "删除账号成功")
}

func (s *AppServer) setDefaultAccountRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	id := c.Param("id")
	if err := s.accountTools.SetDefault(c.Request.Context(), id); err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, map[string]string{"account_id": id}, "设置默认账号成功")
}

func (s *AppServer) accountLoginQRCodeRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	result, err := s.accountTools.GetLoginQRCode(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, result, "获取登录二维码成功")
}

func (s *AppServer) accountLoginStatusRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	result, err := s.accountTools.CheckLoginStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, result, "检查登录状态成功")
}

func (s *AppServer) resetAccountLoginRESTHandler(c *gin.Context) {
	if !s.requireAccountTools(c) {
		return
	}
	id := c.Param("id")
	if err := s.accountTools.ResetLogin(c.Request.Context(), id); err != nil {
		respondAccountRESTError(c, err)
		return
	}
	respondSuccess(c, map[string]string{"account_id": id}, "重置登录状态成功")
}

func (s *AppServer) requireAccountTools(c *gin.Context) bool {
	if s.accountTools != nil {
		return true
	}
	respondError(c, http.StatusServiceUnavailable, string(account.CodeInternalError), "账号管理未初始化", nil)
	return false
}

func respondAccountRESTError(c *gin.Context, err error) {
	code := account.ErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case account.CodeInvalidAccountID, account.CodeAccountRequired, account.CodeRegistryCorrupt:
		status = http.StatusBadRequest
	case account.CodeAccountNotFound, account.CodeCookieNotFound:
		status = http.StatusNotFound
	case account.CodeAccountBusy:
		status = http.StatusTooManyRequests
	case account.CodeOperationCanceled:
		status = http.StatusRequestTimeout
	case account.CodeAccountDisabled, account.CodeAccountPaused, account.CodeAccountRiskHold:
		status = http.StatusConflict
	}
	if code == "" {
		code = account.CodeInternalError
	}
	respondError(c, status, string(code), "账号操作失败", err.Error())
}
