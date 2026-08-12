package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPStatelessSinglePost 固定「/mcp 接受不带 initialize 握手的单次 POST」这一契约。
//
// 契约由 routes.go 里一行 Stateless 支撑，丢掉它编译和其他单测都不会报错。
func TestMCPStatelessSinglePost(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), ""))
	server := httptest.NewServer(router)
	defer server.Close()

	post := func(t *testing.T, body string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	// 只用 tools/list：它不碰浏览器，而契约丢失时它恰好就是失败点——
	// 有状态模式下无 session 调用 tools/list 会被回以「握手前不允许调用」。
	resp := post(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	require.Nil(t, result.Error, "无握手的 tools/list 不应报错")
	assert.NotEmpty(t, result.Result.Tools, "应返回已注册的工具")
}

// TestNotificationToolsRegistered 固定通知相关工具已注册到 MCP。
//
// 三个工具的注册各是 registerTools 里一段独立代码，漏掉任何一个编译都不会报错，
// 只有真正调用时才会发现工具不存在。
func TestNotificationToolsRegistered(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), ""))
	server := httptest.NewServer(router)
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	names := make(map[string]bool, len(result.Result.Tools))
	for _, tool := range result.Result.Tools {
		names[tool.Name] = true
	}

	for _, want := range []string{"get_unread_count", "list_notifications", "reply_notification", "like_notification"} {
		assert.True(t, names[want], "工具 %s 应已注册", want)
	}
}

// TestNotificationRoutesRegistered 固定通知的 HTTP 路由存在。
//
// 读路由表而不是发请求：这些 handler 会真的起浏览器访问小红书，
// 单测里不能碰。
func TestNotificationRoutesRegistered(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), ""))

	registered := make(map[string]bool)
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = true
	}

	// 列表接口两个参数都可选，GET 也要能进
	for _, want := range []string{
		"GET /api/v1/notifications/unread",
		"GET /api/v1/notifications/list",
		"POST /api/v1/notifications/list",
		"POST /api/v1/notifications/reply",
		"POST /api/v1/notifications/like",
	} {
		assert.True(t, registered[want], "路由 %s 应已注册", want)
	}
}

func TestProtectedRoutesRequireBearerToken(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), "secret-token"))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "health remains public", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "MCP requires token", method: http.MethodPost, path: "/mcp", wantStatus: http.StatusUnauthorized},
		{name: "HTTP API requires token", method: http.MethodGet, path: "/api/v1/notifications/unread", wantStatus: http.StatusUnauthorized},
		{name: "CORS preflight remains public", method: http.MethodOptions, path: "/mcp", wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			assert.Equal(t, tt.wantStatus, recorder.Code)
		})
	}
}

func TestMCPAcceptsConfiguredBearerToken(t *testing.T) {
	router := setupRoutes(NewAppServer(NewXiaohongshuService(), "secret-token"))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	request.Header.Set("Authorization", "Bearer secret-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}
