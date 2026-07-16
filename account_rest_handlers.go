package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

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
	accounts.GET("", appServer.listAccountsRESTHandler)
	accounts.POST("", appServer.createAccountRESTHandler)
	accounts.DELETE("/:id", appServer.removeAccountRESTHandler)
	accounts.PUT("/:id/default", appServer.setDefaultAccountRESTHandler)
	accounts.POST("/:id/login/qrcode", appServer.accountLoginQRCodeRESTHandler)
	accounts.POST("/:id/login/status", appServer.accountLoginStatusRESTHandler)
	accounts.DELETE("/:id/login", appServer.resetAccountLoginRESTHandler)
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
