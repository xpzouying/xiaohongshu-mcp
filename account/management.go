package account

import (
	"context"
	"errors"
)

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
		if ErrorCode(err) != CodeAccountNotFound {
			return err
		}
		removal, stageErr := m.cookies.StageRemove(ctx, id)
		if stageErr != nil {
			return errors.Join(err, stageErr)
		}
		if completeErr := removal.Complete(); completeErr != nil {
			return errors.Join(err, completeErr)
		}
		return err
	}
	removal, err := m.cookies.StageRemove(ctx, id)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		return errors.Join(cause, removal.Rollback(context.WithoutCancel(ctx)))
	}
	if err := removal.Commit(ctx); err != nil {
		return rollback(err)
	}
	if err := m.registry.Remove(ctx, id); err != nil {
		return rollback(err)
	}
	return removal.Complete()
}
