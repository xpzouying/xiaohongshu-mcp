package account

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeInvalidAccountID           Code = "INVALID_ACCOUNT_ID"
	CodeAccountRequired            Code = "ACCOUNT_REQUIRED"
	CodeAccountNotFound            Code = "ACCOUNT_NOT_FOUND"
	CodeAccountDisabled            Code = "ACCOUNT_DISABLED"
	CodeAccountPaused              Code = "ACCOUNT_PAUSED"
	CodeAccountRiskHold            Code = "ACCOUNT_RISK_HOLD"
	CodeAccountLoginRequired       Code = "ACCOUNT_LOGIN_REQUIRED"
	CodeAccountBusy                Code = "ACCOUNT_BUSY"
	CodeCookieNotFound             Code = "COOKIE_NOT_FOUND"
	CodeRegistryCorrupt            Code = "REGISTRY_CORRUPT"
	CodeRegistryVersionUnsupported Code = "REGISTRY_VERSION_UNSUPPORTED"
	CodePersistenceFailed          Code = "PERSISTENCE_FAILED"
	CodeLegacyCookieAmbiguous      Code = "LEGACY_COOKIE_AMBIGUOUS"
	CodeOperationCanceled          Code = "OPERATION_CANCELED"
	CodeInternalError              Code = "INTERNAL_ERROR"
)

type Error struct {
	Code      Code
	Message   string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func newError(code Code, message string, retryable bool, cause error) error {
	return &Error{Code: code, Message: message, Retryable: retryable, Cause: cause}
}

func ErrorCode(err error) Code {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}
