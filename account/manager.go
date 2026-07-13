package account

import "context"

type Manager struct {
	registry Registry
	locks    LockManager
	factory  BrowserFactory
}

func NewAccountManager(registry Registry, locks LockManager, factory BrowserFactory) *Manager {
	return &Manager{registry: registry, locks: locks, factory: factory}
}

func (m *Manager) WithAccount(ctx context.Context, requestedID string, kind OperationKind, fn func(context.Context, Account, Browser) error) (ResolvedAccount, error) {
	resolved, err := m.registry.Resolve(ctx, requestedID)
	if err != nil {
		return ResolvedAccount{}, err
	}
	if err := gate(resolved.Account.Status, kind); err != nil {
		return resolved, err
	}
	release, err := m.locks.Acquire(ctx, resolved.Account.ID)
	if err != nil {
		return resolved, err
	}
	defer release()
	account, err := m.registry.Get(ctx, resolved.Account.ID)
	if err != nil {
		return resolved, err
	}
	if err := gate(account.Status, kind); err != nil {
		return resolved, err
	}
	resolved.Account = account
	browser, err := m.factory.New(ctx, account)
	if err != nil {
		return resolved, err
	}
	defer browser.Close()
	return resolved, fn(ctx, account, browser)
}

func gate(status Status, kind OperationKind) error {
	switch status {
	case StatusActive:
		return nil
	case StatusNeedsLogin:
		if kind == OperationLogin || kind == OperationCookieAdmin || kind == OperationDiagnostic {
			return nil
		}
		return newError(CodeAccountLoginRequired, "账号需要登录", false, nil)
	case StatusPaused:
		if kind == OperationRead || kind == OperationDiagnostic {
			return nil
		}
		return newError(CodeAccountPaused, "账号已暂停", false, nil)
	case StatusRiskHold:
		if kind == OperationDiagnostic {
			return nil
		}
		return newError(CodeAccountRiskHold, "账号处于风控冻结状态", false, nil)
	case StatusDisabled:
		return newError(CodeAccountDisabled, "账号已禁用", false, nil)
	default:
		return newError(CodeRegistryCorrupt, "账号状态无效", false, nil)
	}
}
