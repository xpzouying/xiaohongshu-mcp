package account

import "context"

type ManagementManager struct {
	registry Registry
	locks    TryLockManager
	cookies  CookieStore
}

func NewManagementManager(registry Registry, locks TryLockManager, cookies CookieStore) *ManagementManager {
	return &ManagementManager{registry: registry, locks: locks, cookies: cookies}
}

func (m *ManagementManager) Remove(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	release, acquired, err := m.locks.TryAcquire(id)
	if err != nil {
		return err
	}
	if !acquired {
		return newError(CodeAccountBusy, "账号正在执行其他操作", true, nil)
	}
	defer release()
	if _, err := m.registry.Get(ctx, id); err != nil {
		return err
	}
	if err := m.cookies.Delete(ctx, id); err != nil {
		return err
	}
	return m.registry.Remove(ctx, id)
}
