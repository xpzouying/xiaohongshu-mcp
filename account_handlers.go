package main

import (
	"context"
	"encoding/json"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

func accountToolResult(value any, err error) *MCPToolResult {
	if err != nil {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: err.Error()}}, IsError: true}
	}
	data, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "响应序列化失败"}}, IsError: true}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleListAccounts(ctx context.Context) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	items, err := s.accountTools.List(ctx)
	return accountToolResult(items, err)
}

func (s *AppServer) handleCreateAccount(ctx context.Context, input account.CreateAccountInput) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	value, err := s.accountTools.Create(ctx, input)
	return accountToolResult(value, err)
}

func (s *AppServer) handleRemoveAccount(ctx context.Context, id string) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	return accountToolResult(map[string]string{"account_id": id}, s.accountTools.Remove(ctx, id))
}

func (s *AppServer) handleSetDefaultAccount(ctx context.Context, id string) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	return accountToolResult(map[string]string{"account_id": id}, s.accountTools.SetDefault(ctx, id))
}

func (s *AppServer) handleAccountLoginStatus(ctx context.Context, id string) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	value, err := s.accountTools.CheckLoginStatus(ctx, id)
	return accountToolResult(value, err)
}

func (s *AppServer) handleAccountLoginQRCode(ctx context.Context, id string) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	value, err := s.accountTools.GetLoginQRCode(ctx, id)
	if err != nil {
		return accountToolResult(nil, err)
	}
	status := AccountQRCode{AccountID: value.AccountID, IsLoggedIn: value.IsLoggedIn}
	if value.IsLoggedIn {
		return accountToolResult(status, nil)
	}
	if value.Image == "" {
		return accountToolResult(nil, &account.Error{Code: account.CodeInternalError, Message: "登录二维码为空"})
	}
	data, marshalErr := json.Marshal(status)
	if marshalErr != nil {
		return accountToolResult(nil, marshalErr)
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}, {Type: "image", MimeType: "image/png", Data: value.Image}}}
}

func (s *AppServer) handleResetAccountLogin(ctx context.Context, id string) *MCPToolResult {
	if s.accountTools == nil {
		return accountToolResult(nil, accountUnavailableError())
	}
	return accountToolResult(map[string]string{"account_id": id}, s.accountTools.ResetLogin(ctx, id))
}

type unavailableAccountToolsError struct{}

func (unavailableAccountToolsError) Error() string { return "账号管理未初始化" }
func accountUnavailableError() error               { return unavailableAccountToolsError{} }
