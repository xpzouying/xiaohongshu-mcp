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
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: `{"account_id":"` + value.AccountID + `","is_logged_in":false}`}, {Type: "image", MimeType: "image/png", Data: value.Image}}}
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
