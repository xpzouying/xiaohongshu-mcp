package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/account"
)

func TestAccountRESTRoutes(t *testing.T) {
	registry := &accountHandlerRegistry{resolved: account.ResolvedAccount{
		Account:         account.Account{ID: "acct_one"},
		SelectionSource: account.SelectionDefault,
	}}
	login := &accountHandlerLogin{qrCode: "png-data"}
	tools := &AccountTools{registry: registry, login: login}
	app := &AppServer{accountTools: tools}
	router := setupRoutes(app)

	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(`{"id":"acct_one","display_name":"主账号"}`)))
	if create.Code != http.StatusOK || registry.created.ID != "acct_one" {
		t.Fatalf("create: status=%d body=%s input=%+v", create.Code, create.Body.String(), registry.created)
	}

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"accounts"`) || !strings.Contains(list.Body.String(), `"default_account_id":"acct_one"`) {
		t.Fatalf("list: status=%d body=%s", list.Code, list.Body.String())
	}

	qr := httptest.NewRecorder()
	router.ServeHTTP(qr, httptest.NewRequest(http.MethodPost, "/api/v1/accounts/acct_one/login/qrcode", nil))
	if qr.Code != http.StatusOK || !strings.Contains(qr.Body.String(), `"image":"png-data"`) {
		t.Fatalf("qrcode: status=%d body=%s", qr.Code, qr.Body.String())
	}
}

func TestAccountRESTErrorsUseAccountCode(t *testing.T) {
	app := &AppServer{accountTools: &AccountTools{registry: &accountHandlerRegistry{err: &account.Error{Code: account.CodeAccountNotFound, Message: "账号不存在"}}}}
	router := setupRoutes(app)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Code != string(account.CodeAccountNotFound) {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}
}

func TestAccountRESTListWithoutDefaultStillSucceeds(t *testing.T) {
	registry := &accountHandlerRegistry{resolveErr: &account.Error{Code: account.CodeAccountRequired, Message: "必须明确指定账号"}}
	router := setupRoutes(&AppServer{accountTools: &AccountTools{registry: registry}})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"default_account_id":null`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAccountRESTBusyUsesTooManyRequests(t *testing.T) {
	app := &AppServer{accountTools: &AccountTools{registry: &accountHandlerRegistry{err: &account.Error{Code: account.CodeAccountBusy, Message: "账号忙"}}}}
	router := setupRoutes(app)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCreateAccountRESTRejectsOversizedBody(t *testing.T) {
	router := setupRoutes(&AppServer{accountTools: &AccountTools{registry: &accountHandlerRegistry{}}})
	response := httptest.NewRecorder()
	body := `{"id":"acct_one","display_name":"` + strings.Repeat("x", maxRESTRequestBody) + `"}`
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body)))

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Code != "REQUEST_TOO_LARGE" {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}
}

type accountHandlerRegistry struct {
	created    account.CreateAccountInput
	err        error
	resolved   account.ResolvedAccount
	resolveErr error
}

func (r *accountHandlerRegistry) List(_ context.Context) ([]account.Account, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []account.Account{{ID: "acct_one", DisplayName: "主账号", Status: account.StatusNeedsLogin}}, nil
}
func (r *accountHandlerRegistry) Get(_ context.Context, id string) (account.Account, error) {
	if r.err != nil {
		return account.Account{}, r.err
	}
	return account.Account{ID: id, DisplayName: "主账号", Status: account.StatusNeedsLogin}, nil
}
func (r *accountHandlerRegistry) Resolve(context.Context, string) (account.ResolvedAccount, error) {
	return r.resolved, r.resolveErr
}
func (r *accountHandlerRegistry) Create(_ context.Context, input account.CreateAccountInput) (account.Account, error) {
	r.created = input
	if r.err != nil {
		return account.Account{}, r.err
	}
	return account.Account{ID: input.ID, DisplayName: input.DisplayName, Status: account.StatusNeedsLogin}, nil
}
func (r *accountHandlerRegistry) Remove(context.Context, string) error     { return r.err }
func (r *accountHandlerRegistry) SetDefault(context.Context, string) error { return r.err }
func (r *accountHandlerRegistry) UpdateStatus(context.Context, string, account.Status, string) error {
	return r.err
}

type accountHandlerLogin struct {
	qrCode string
}

func (*accountHandlerLogin) Status(context.Context, string) (bool, string, error) {
	return false, "", nil
}
func (l *accountHandlerLogin) QRCode(context.Context, string) (string, bool, error) {
	return l.qrCode, false, nil
}
func (*accountHandlerLogin) Cancel(string) {}
