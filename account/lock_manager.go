package account

import (
	"context"
	"sync"
)

type accountLock struct {
	semaphore chan struct{}
	refs      int
}

type InMemoryLockManager struct {
	global chan struct{}
	mu     sync.Mutex
	locks  map[string]*accountLock
}

func NewLockManager(maxConcurrency int) (*InMemoryLockManager, error) {
	if maxConcurrency < 1 || maxConcurrency > 8 {
		return nil, newError(CodeInternalError, "账号并发数必须在 1 到 8 之间", false, nil)
	}
	return &InMemoryLockManager{global: make(chan struct{}, maxConcurrency), locks: make(map[string]*accountLock)}, nil
}

func (m *InMemoryLockManager) Acquire(ctx context.Context, accountID string) (func(), error) {
	if err := ValidateAccountID(accountID); err != nil {
		return nil, err
	}
	m.mu.Lock()
	entry := m.locks[accountID]
	if entry == nil {
		entry = &accountLock{semaphore: make(chan struct{}, 1)}
		m.locks[accountID] = entry
	}
	entry.refs++
	m.mu.Unlock()

	select {
	case entry.semaphore <- struct{}{}:
	case <-ctx.Done():
		m.releaseReference(accountID, entry)
		return nil, lockContextError(ctx.Err())
	}

	select {
	case m.global <- struct{}{}:
	case <-ctx.Done():
		<-entry.semaphore
		m.releaseReference(accountID, entry)
		return nil, lockContextError(ctx.Err())
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			<-entry.semaphore
			m.releaseReference(accountID, entry)
			<-m.global
		})
	}, nil
}

func (m *InMemoryLockManager) TryAcquire(accountID string) (func(), bool, error) {
	if err := ValidateAccountID(accountID); err != nil {
		return nil, false, err
	}
	m.mu.Lock()
	entry := m.locks[accountID]
	if entry == nil {
		entry = &accountLock{semaphore: make(chan struct{}, 1)}
		m.locks[accountID] = entry
	}
	entry.refs++
	m.mu.Unlock()
	select {
	case entry.semaphore <- struct{}{}:
	default:
		m.releaseReference(accountID, entry)
		return nil, false, nil
	}
	select {
	case m.global <- struct{}{}:
	default:
		<-entry.semaphore
		m.releaseReference(accountID, entry)
		return nil, false, nil
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			<-entry.semaphore
			m.releaseReference(accountID, entry)
			<-m.global
		})
	}, true, nil
}

func (m *InMemoryLockManager) releaseReference(accountID string, entry *accountLock) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(m.locks, accountID)
	}
}

func lockContextError(err error) error {
	if err == context.DeadlineExceeded {
		return newError(CodeAccountBusy, "账号正在执行其他操作", true, err)
	}
	return canceledError(err)
}
