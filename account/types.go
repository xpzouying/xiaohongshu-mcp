package account

import (
	"context"
	"regexp"
	"time"
)

type Status string

const (
	StatusActive     Status = "active"
	StatusNeedsLogin Status = "needs_login"
	StatusPaused     Status = "paused"
	StatusRiskHold   Status = "risk_hold"
	StatusDisabled   Status = "disabled"
)

type Account struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Owner       string    `json:"owner,omitempty"`
	Purpose     string    `json:"purpose,omitempty"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateAccountInput struct {
	ID          string
	DisplayName string
	Owner       string
	Purpose     string
}

type ResolvedAccount struct {
	Account         Account
	SelectionSource string
}

const (
	SelectionExplicit        = "explicit"
	SelectionDefault         = "default"
	SelectionSingleAvailable = "single_available"
)

type Registry interface {
	List(context.Context) ([]Account, error)
	Get(context.Context, string) (Account, error)
	Resolve(context.Context, string) (ResolvedAccount, error)
	Create(context.Context, CreateAccountInput) (Account, error)
	Remove(context.Context, string) error
	SetDefault(context.Context, string) error
	UpdateStatus(context.Context, string, Status, string) error
}

type CookieStore interface {
	Load(context.Context, string) ([]byte, error)
	Save(context.Context, string, []byte) error
	Delete(context.Context, string) error
	StageRemove(context.Context, string) (CookieRemoval, error)
	Path(string) (string, error)
}

type CookieRemoval interface {
	Commit(context.Context) error
	Rollback(context.Context) error
	Complete() error
}

type Browser interface{ Close() }

type BrowserFactory interface {
	New(context.Context, Account) (Browser, error)
}

type LockManager interface {
	Acquire(context.Context, string) (func(), error)
}

type TryLockManager interface {
	LockManager
	TryAcquire(string) (func(), bool, error)
}

type OperationKind string

const (
	OperationRead        OperationKind = "read"
	OperationWrite       OperationKind = "write"
	OperationLogin       OperationKind = "login"
	OperationCookieAdmin OperationKind = "cookie_admin"
	OperationDiagnostic  OperationKind = "diagnostic"
)

var accountIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
var reservedIDs = map[string]struct{}{"accounts": {}, "system": {}, "root": {}, "null": {}, "unknown": {}}

func ValidateAccountID(id string) error {
	if !accountIDPattern.MatchString(id) {
		return newError(CodeInvalidAccountID, "账号 ID 格式无效", false, nil)
	}
	if _, reserved := reservedIDs[id]; reserved {
		return newError(CodeInvalidAccountID, "账号 ID 为保留值", false, nil)
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusActive, StatusNeedsLogin, StatusPaused, StatusRiskHold, StatusDisabled:
		return true
	default:
		return false
	}
}
