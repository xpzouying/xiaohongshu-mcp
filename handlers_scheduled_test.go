package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScheduledNotesToolAndRoute(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), ""))
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/list status: %d", w.Code)
	}
	var response struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Annotations struct {
					ReadOnly    bool `json:"readOnlyHint"`
					Destructive bool `json:"destructiveHint"`
				} `json:"annotations"`
				Schema struct {
					Required []string `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range response.Result.Tools {
		if tool.Name != "list_scheduled_notes" {
			continue
		}
		found = true
		if !tool.Annotations.ReadOnly || tool.Annotations.Destructive || len(tool.Schema.Required) != 0 {
			t.Fatal("预约查询必须是无必填参数的只读工具")
		}
	}
	if !found {
		t.Fatal("未注册 list_scheduled_notes")
	}
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/notes/scheduled" {
			return
		}
	}
	t.Fatal("未注册预约查询 REST 路由")
}

func TestScheduledNotesRouteRequiresAuth(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), "test-token"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/notes/scheduled", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("预约查询路由应受鉴权保护，实际状态 %d", w.Code)
	}
}
