package account

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const registrySchemaVersion = 1

type registryDocument struct {
	SchemaVersion    int       `json:"schema_version"`
	DefaultAccountID *string   `json:"default_account_id"`
	Accounts         []Account `json:"accounts"`
}

type RegistryOption func(*FileRegistry)

func WithClock(clock func() time.Time) RegistryOption {
	return func(r *FileRegistry) { r.clock = clock }
}

type FileRegistry struct {
	mu    sync.RWMutex
	path  string
	doc   registryDocument
	clock func() time.Time
}

func NewFileRegistry(root string, options ...RegistryOption) (*FileRegistry, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, newError(CodeRegistryCorrupt, "注册表目录无效", false, err)
	}
	r := &FileRegistry{path: filepath.Join(abs, "accounts.json"), clock: func() time.Time { return time.Now().UTC() }}
	for _, option := range options {
		option(r)
	}
	data, err := readFileNoFollow(abs, r.path, true)
	if errors.Is(err, os.ErrNotExist) {
		r.doc = registryDocument{SchemaVersion: registrySchemaVersion, Accounts: []Account{}}
		return r, nil
	}
	if err != nil {
		return nil, newError(CodeRegistryCorrupt, "读取注册表失败", false, err)
	}
	if err := strictDecode(data, &r.doc); err != nil {
		return nil, newError(CodeRegistryCorrupt, "注册表 JSON 无效", false, err)
	}
	if r.doc.SchemaVersion != registrySchemaVersion {
		return nil, newError(CodeRegistryVersionUnsupported, "不支持的注册表版本", false, nil)
	}
	if err := validateDocument(r.doc); err != nil {
		return nil, err
	}
	return r, nil
}

func validateDocument(doc registryDocument) error {
	seen := make(map[string]struct{}, len(doc.Accounts))
	for _, account := range doc.Accounts {
		if err := ValidateAccountID(account.ID); err != nil {
			return newError(CodeRegistryCorrupt, "注册表含非法账号 ID", false, err)
		}
		if _, exists := seen[account.ID]; exists {
			return newError(CodeRegistryCorrupt, "注册表含重复账号", false, nil)
		}
		seen[account.ID] = struct{}{}
		if account.DisplayName == "" || !validStatus(account.Status) || account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() {
			return newError(CodeRegistryCorrupt, "注册表账号字段无效", false, nil)
		}
	}
	if doc.DefaultAccountID != nil {
		found := false
		for _, account := range doc.Accounts {
			if account.ID == *doc.DefaultAccountID && account.Status != StatusDisabled {
				found = true
			}
		}
		if !found {
			return newError(CodeRegistryCorrupt, "默认账号无效", false, nil)
		}
	}
	return nil
}

func (r *FileRegistry) saveLocked() error {
	sort.Slice(r.doc.Accounts, func(i, j int) bool { return r.doc.Accounts[i].ID < r.doc.Accounts[j].ID })
	data, err := json.MarshalIndent(r.doc, "", "  ")
	if err != nil {
		return newError(CodePersistenceFailed, "编码注册表失败", true, err)
	}
	data = append(data, '\n')
	if err := atomicWrite(filepath.Dir(r.path), r.path, data); err != nil {
		return newError(CodePersistenceFailed, "保存注册表失败", true, err)
	}
	return nil
}

func cloneAccounts(accounts []Account) []Account {
	result := make([]Account, len(accounts))
	copy(result, accounts)
	return result
}

func (r *FileRegistry) List(ctx context.Context) ([]Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, canceledError(err)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneAccounts(r.doc.Accounts), nil
}

func (r *FileRegistry) Get(ctx context.Context, id string) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, canceledError(err)
	}
	if err := ValidateAccountID(id); err != nil {
		return Account{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getLocked(id)
}

func (r *FileRegistry) getLocked(id string) (Account, error) {
	for _, account := range r.doc.Accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return Account{}, newError(CodeAccountNotFound, "账号不存在", false, nil)
}

func (r *FileRegistry) Resolve(ctx context.Context, requestedID string) (ResolvedAccount, error) {
	if err := ctx.Err(); err != nil {
		return ResolvedAccount{}, canceledError(err)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if requestedID != "" {
		if err := ValidateAccountID(requestedID); err != nil {
			return ResolvedAccount{}, err
		}
		account, err := r.getLocked(requestedID)
		if err != nil {
			return ResolvedAccount{}, err
		}
		return ResolvedAccount{Account: account, SelectionSource: SelectionExplicit}, nil
	}
	if r.doc.DefaultAccountID != nil {
		account, err := r.getLocked(*r.doc.DefaultAccountID)
		if err == nil && account.Status != StatusDisabled {
			return ResolvedAccount{Account: account, SelectionSource: SelectionDefault}, nil
		}
	}
	var available []Account
	for _, account := range r.doc.Accounts {
		if account.Status != StatusDisabled {
			available = append(available, account)
		}
	}
	if len(available) == 1 {
		return ResolvedAccount{Account: available[0], SelectionSource: SelectionSingleAvailable}, nil
	}
	return ResolvedAccount{}, newError(CodeAccountRequired, "必须明确指定账号", false, nil)
}

func (r *FileRegistry) Create(ctx context.Context, in CreateAccountInput) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, canceledError(err)
	}
	if err := ValidateAccountID(in.ID); err != nil {
		return Account{}, err
	}
	if in.DisplayName == "" {
		return Account{}, newError(CodeRegistryCorrupt, "展示名称不能为空", false, nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.getLocked(in.ID); err == nil {
		return Account{}, newError(CodeRegistryCorrupt, "账号已存在", false, nil)
	}
	now := r.clock().UTC()
	account := Account{ID: in.ID, DisplayName: in.DisplayName, Owner: in.Owner, Purpose: in.Purpose, Status: StatusNeedsLogin, CreatedAt: now, UpdatedAt: now}
	old := cloneAccounts(r.doc.Accounts)
	r.doc.Accounts = append(r.doc.Accounts, account)
	if err := r.saveLocked(); err != nil {
		r.doc.Accounts = old
		return Account{}, err
	}
	return account, nil
}

func (r *FileRegistry) Remove(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	if err := ValidateAccountID(id); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	index := -1
	for i := range r.doc.Accounts {
		if r.doc.Accounts[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return newError(CodeAccountNotFound, "账号不存在", false, nil)
	}
	old := registryDocument{SchemaVersion: r.doc.SchemaVersion, DefaultAccountID: r.doc.DefaultAccountID, Accounts: cloneAccounts(r.doc.Accounts)}
	r.doc.Accounts = append(r.doc.Accounts[:index], r.doc.Accounts[index+1:]...)
	if r.doc.DefaultAccountID != nil && *r.doc.DefaultAccountID == id {
		r.doc.DefaultAccountID = nil
	}
	if err := r.saveLocked(); err != nil {
		r.doc = old
		return err
	}
	return nil
}

func (r *FileRegistry) SetDefault(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	if err := ValidateAccountID(id); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	account, err := r.getLocked(id)
	if err != nil {
		return err
	}
	if account.Status == StatusDisabled {
		return newError(CodeAccountDisabled, "禁用账号不能设为默认", false, nil)
	}
	old := r.doc.DefaultAccountID
	value := id
	r.doc.DefaultAccountID = &value
	if err := r.saveLocked(); err != nil {
		r.doc.DefaultAccountID = old
		return err
	}
	return nil
}

func (r *FileRegistry) UpdateStatus(ctx context.Context, id string, status Status, reason string) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	if err := ValidateAccountID(id); err != nil {
		return err
	}
	if !validStatus(status) {
		return newError(CodeRegistryCorrupt, "账号状态无效", false, nil)
	}
	if reason == "" {
		return newError(CodeRegistryCorrupt, "状态变更原因不能为空", false, nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	index := -1
	for i := range r.doc.Accounts {
		if r.doc.Accounts[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return newError(CodeAccountNotFound, "账号不存在", false, nil)
	}
	oldDoc := registryDocument{SchemaVersion: r.doc.SchemaVersion, DefaultAccountID: r.doc.DefaultAccountID, Accounts: cloneAccounts(r.doc.Accounts)}
	r.doc.Accounts[index].Status = status
	r.doc.Accounts[index].UpdatedAt = r.clock().UTC()
	if status == StatusDisabled && r.doc.DefaultAccountID != nil && *r.doc.DefaultAccountID == id {
		r.doc.DefaultAccountID = nil
	}
	if err := r.saveLocked(); err != nil {
		r.doc = oldDoc
		return err
	}
	return nil
}
