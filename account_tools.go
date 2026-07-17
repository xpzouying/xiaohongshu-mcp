package main

import (
	"context"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

type AccountLogin interface {
	Status(context.Context, string) (bool, string, error)
	QRCode(context.Context, string) (string, bool, error)
	Cancel(string)
}

type AccountLoginStatus struct {
	AccountID  string           `json:"account_id"`
	IsLoggedIn bool             `json:"is_logged_in"`
	Identity   *AccountIdentity `json:"identity,omitempty"`
}

type AccountIdentity struct {
	Nickname string `json:"nickname"`
}

type AccountQRCode struct {
	AccountID  string `json:"account_id"`
	Image      string `json:"image,omitempty"`
	IsLoggedIn bool   `json:"is_logged_in"`
}

type AccountTools struct {
	registry   account.Registry
	management *account.ManagementManager
	cookies    account.CookieStore
	login      AccountLogin
}

func NewAccountTools(registry account.Registry, management *account.ManagementManager, cookies account.CookieStore, login AccountLogin) *AccountTools {
	return &AccountTools{registry: registry, management: management, cookies: cookies, login: login}
}

func (t *AccountTools) List(ctx context.Context) ([]account.Account, error) {
	return t.registry.List(ctx)
}

func (t *AccountTools) DefaultAccountID(ctx context.Context) (*string, error) {
	resolved, err := t.registry.Resolve(ctx, "")
	if err != nil {
		if account.ErrorCode(err) == account.CodeAccountRequired {
			return nil, nil
		}
		return nil, err
	}
	if resolved.SelectionSource != account.SelectionDefault {
		return nil, nil
	}
	id := resolved.Account.ID
	return &id, nil
}

func (t *AccountTools) Create(ctx context.Context, input account.CreateAccountInput) (account.Account, error) {
	return t.registry.Create(ctx, input)
}

func (t *AccountTools) Remove(ctx context.Context, id string) error {
	t.login.Cancel(id)
	return t.management.Remove(ctx, id)
}

func (t *AccountTools) SetDefault(ctx context.Context, id string) error {
	return t.registry.SetDefault(ctx, id)
}

func (t *AccountTools) CheckLoginStatus(ctx context.Context, id string) (AccountLoginStatus, error) {
	if _, err := t.registry.Get(ctx, id); err != nil {
		return AccountLoginStatus{}, err
	}
	loggedIn, identity, err := t.login.Status(ctx, id)
	if err != nil {
		return AccountLoginStatus{}, err
	}
	status := AccountLoginStatus{AccountID: id, IsLoggedIn: loggedIn}
	if identity != "" {
		status.Identity = &AccountIdentity{Nickname: identity}
	}
	return status, nil
}

func (t *AccountTools) GetLoginQRCode(ctx context.Context, id string) (AccountQRCode, error) {
	if _, err := t.registry.Get(ctx, id); err != nil {
		return AccountQRCode{}, err
	}
	image, loggedIn, err := t.login.QRCode(ctx, id)
	if err != nil {
		return AccountQRCode{}, err
	}
	return AccountQRCode{AccountID: id, Image: image, IsLoggedIn: loggedIn}, nil
}

func (t *AccountTools) ResetLogin(ctx context.Context, id string) error {
	release, acquired, err := t.management.TryLock(id)
	if err != nil {
		return err
	}
	if !acquired {
		return &account.Error{Code: account.CodeAccountBusy, Message: "账号正在执行其他操作", Retryable: true}
	}
	defer release()
	t.login.Cancel(id)
	if _, err := t.registry.Get(ctx, id); err != nil {
		return err
	}
	if err := t.cookies.Delete(ctx, id); err != nil {
		return err
	}
	return t.registry.UpdateStatus(ctx, id, account.StatusNeedsLogin, "登录状态已重置")
}

// UpdateDisplayName 更新账号展示名称（用于扫码登录后自动同步昵称）
func (t *AccountTools) UpdateDisplayName(ctx context.Context, id, displayName string) error {
	if _, err := t.registry.Get(ctx, id); err != nil {
		return err
	}
	return t.registry.UpdateDisplayName(ctx, id, displayName)
}
