package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

func TestDockerImageUsesInitToReapChromeChildren(t *testing.T) {
	data, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := string(data)
	if !strings.Contains(dockerfile, "tini") || !strings.Contains(dockerfile, `ENTRYPOINT ["/usr/bin/tini", "--"]`) {
		t.Fatal("Docker image must run the app under tini so exited Chrome children are reaped")
	}
}

func TestAccountHandlerQRCodeAlreadyLoggedInHasNoImage(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, &fakeAccountLogin{loggedIn: true})
	s := &AppServer{accountTools: tools}
	_, _ = tools.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})

	result := s.handleAccountLoginQRCode(context.Background(), "acct_a")
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("result=%+v", result)
	}
	var status AccountQRCode
	if err := json.Unmarshal([]byte(result.Content[0].Text), &status); err != nil {
		t.Fatal(err)
	}
	if !status.IsLoggedIn || status.AccountID != "acct_a" {
		t.Fatalf("status=%+v", status)
	}
}

func TestAccountHandlerQRCodeRejectsEmptyImage(t *testing.T) {
	root := t.TempDir()
	registry, _ := account.NewFileRegistry(root)
	store, _ := account.NewFileCookieStore(root)
	locks, _ := account.NewLockManager(1)
	tools := NewAccountTools(registry, account.NewManagementManager(registry, locks, store), store, &fakeAccountLogin{})
	s := &AppServer{accountTools: tools}
	_, _ = tools.Create(context.Background(), account.CreateAccountInput{ID: "acct_a", DisplayName: "A"})

	result := s.handleAccountLoginQRCode(context.Background(), "acct_a")
	if !result.IsError || len(result.Content) != 1 {
		t.Fatalf("result=%+v", result)
	}
}

func TestRESTAccountRoutingRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := &routingRegistry{}
	locks, _ := account.NewLockManager(1)
	manager := account.NewAccountManager(registry, locks, routingFactory{browser: &routingBrowser{}})
	router := gin.New()
	router.POST("/business", withRESTAccountRouting(manager, account.OperationRead, func(c *gin.Context) {
		t.Fatal("handler was called")
	}))
	body := `{"account_id":"acct_one","padding":"` + strings.Repeat("x", maxRESTRequestBody) + `"}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/business", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
