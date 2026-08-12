# Static Token Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 HTTP API 和 MCP Streamable HTTP 端点增加由 `-token` 或 `AUTH_TOKEN` 控制的可选静态 Bearer Token 鉴权，同时保持 `/health` 公开。

**Architecture:** 在 Gin 中实现单一 `authMiddleware(token string)`，将 `/api/v1/*` 与 `/mcp*` 注册到同一个受保护路由组，公开路由和全局 CORS 中间件留在组外。启动入口以 `AUTH_TOKEN` 作为 `-token` 默认值并把最终配置注入 `AppServer`；Docker 仅透传运行时环境变量，不写入真实密钥。

**Tech Stack:** Go 1.24、Gin 1.10、MCP Go SDK 1.4、Testify、Docker/Docker Compose

## Global Constraints

- 环境变量名称必须是 `AUTH_TOKEN`，启动参数名称必须是 `-token`，显式参数优先。
- 未配置或配置为空时必须关闭鉴权，维持向后兼容。
- 受保护请求只接受精确格式 `Authorization: Bearer <token>`。
- `/health` 和 CORS `OPTIONS` 预检必须免鉴权。
- Token 不得出现在日志、错误响应、镜像层或已提交的 Compose 配置中。
- 不引入新的第三方依赖，不扩展 OAuth、Token 轮换或多 Token 管理。
- Go 注释使用简洁中文，修改后的 Go 文件必须执行 `gofmt`。
- 所有提交保留在 `feat/static-token-auth` 本地分支，未经用户同意不得推送。

---

## File Structure

- `middleware.go`：实现可选静态 Bearer Token 鉴权中间件。
- `middleware_test.go`：独立验证中间件的关闭、放行和拒绝行为。
- `app_server.go`：在 `AppServer` 中保存最终 Token 配置。
- `main.go`：解析 `AUTH_TOKEN` 与 `-token`，把最终值注入应用服务器。
- `routes.go`：建立受保护路由组，公开 `/health`，保持 CORS 在鉴权之前执行。
- `routes_test.go`：验证 HTTP API、MCP、健康检查和预检请求的路由级鉴权契约。
- `Dockerfile`：声明空的 `AUTH_TOKEN` 运行时默认值。
- `docker/docker-compose.yml`：从宿主环境安全透传 `AUTH_TOKEN`。
- `README.md`、`README_EN.md`、`docker/README.md`：记录本机、HTTP、MCP 和容器使用方法。

---

### Task 1: 可选 Bearer Token Gin 中间件

**Files:**
- Create: `middleware_test.go`
- Modify: `middleware.go:3-34`

**Interfaces:**
- Consumes: `respondError(c *gin.Context, statusCode int, code, message string, details any)` from `handlers_api.go`.
- Produces: `authMiddleware(token string) gin.HandlerFunc`，供 `routes.go` 的受保护路由组调用。

- [ ] **Step 1: 写出关闭鉴权和正确 Token 放行的失败测试**

在 `middleware_test.go` 中创建最小测试路由：

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newAuthTestRouter(token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(authMiddleware(token))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func TestAuthMiddlewareAllowsRequestWhenDisabled(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)

	newAuthTestRouter("").ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthMiddlewareAllowsValidBearerToken(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer secret-token")

	newAuthTestRouter("secret-token").ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}
```

- [ ] **Step 2: 运行测试并确认因中间件不存在而失败**

Run: `go test ./... -run 'TestAuthMiddlewareAllows(RequestWhenDisabled|ValidBearerToken)$'`

Expected: FAIL，编译错误包含 `undefined: authMiddleware`。

- [ ] **Step 3: 写出拒绝无效凭据的失败测试**

继续添加：

```go
func TestAuthMiddlewareRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name          string
	authorization string
	}{
		{name: "missing header"},
		{name: "wrong scheme", authorization: "Basic secret-token"},
		{name: "missing token", authorization: "Bearer "},
		{name: "wrong token", authorization: "Bearer wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}

			newAuthTestRouter("secret-token").ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.Equal(t, "Bearer", recorder.Header().Get("WWW-Authenticate"))
			assert.JSONEq(t, `{"error":"未授权","code":"UNAUTHORIZED"}`, recorder.Body.String())
		})
	}
}
```

- [ ] **Step 4: 实现最小鉴权中间件**

在 `middleware.go` 增加 `crypto/subtle` 导入和以下函数。直接比较完整请求头可同时固定 Bearer 格式，并避免解析产生多种等价形式：

```go
// authMiddleware 静态 Bearer Token 鉴权中间件，Token 为空时关闭鉴权。
func authMiddleware(token string) gin.HandlerFunc {
	expectedAuthorization := []byte("Bearer " + token)

	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}

		authorization := []byte(c.GetHeader("Authorization"))
		if subtle.ConstantTimeCompare(authorization, expectedAuthorization) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "未授权", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
```

- [ ] **Step 5: 格式化并运行中间件测试**

Run: `gofmt -w middleware.go middleware_test.go`

Run: `go test ./... -run '^TestAuthMiddleware'`

Expected: PASS，全部中间件测试通过。

- [ ] **Step 6: 提交中间件**

```bash
git add middleware.go middleware_test.go
git commit -m "feat: add optional bearer auth middleware"
```

---

### Task 2: 启动配置和受保护路由

**Files:**
- Modify: `main.go:3-48`
- Modify: `app_server.go:16-38`
- Modify: `routes.go:10-67`
- Modify: `routes_test.go:14-116`

**Interfaces:**
- Consumes: `authMiddleware(token string) gin.HandlerFunc` from Task 1.
- Produces: `NewAppServer(xiaohongshuService *XiaohongshuService, authToken string) *AppServer`；`AppServer.authToken string` 由 `setupRoutes` 使用。

- [ ] **Step 1: 更新现有测试调用并写出路由鉴权失败测试**

先把 `routes_test.go` 中三个现有构造调用统一改为：

```go
NewAppServer(NewXiaohongshuService(), "")
```

然后增加：

```go
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
```

- [ ] **Step 2: 运行测试并确认构造函数签名不匹配**

Run: `go test ./... -run 'Test(ProtectedRoutesRequireBearerToken|MCPAcceptsConfiguredBearerToken)$'`

Expected: FAIL，编译错误指出 `NewAppServer` 参数数量不匹配。

- [ ] **Step 3: 将 Token 注入 AppServer**

在 `app_server.go` 增加字段并修改构造函数：

```go
type AppServer struct {
	xiaohongshuService *XiaohongshuService
	mcpServer          *mcp.Server
	router             *gin.Engine
	httpServer         *http.Server
	authToken          string
}

func NewAppServer(xiaohongshuService *XiaohongshuService, authToken string) *AppServer {
	appServer := &AppServer{
		xiaohongshuService: xiaohongshuService,
		authToken:          authToken,
	}

	// 初始化 MCP Server（需要在创建 appServer 之后，因为工具注册需要访问 appServer）
	appServer.mcpServer = InitMCPServer(appServer)

	return appServer
}
```

- [ ] **Step 4: 建立统一受保护路由组**

在 `routes.go` 保持全局中间件和 `/health` 原位，在它们之后创建路由组，并将 MCP 与 API 注册改到该组：

```go
protected := router.Group("")
protected.Use(authMiddleware(appServer.authToken))

protected.Any("/mcp", gin.WrapH(mcpHandler))
protected.Any("/mcp/*path", gin.WrapH(mcpHandler))

api := protected.Group("/api/v1")
```

不要给 `/health` 添加 `authMiddleware`。保持 `router.Use(corsMiddleware())` 位于路由组之前，使 `OPTIONS` 在鉴权执行前被终止并返回 204。

- [ ] **Step 5: 解析环境变量和启动参数**

在 `main.go` 导入 `os`，声明 `token string`，并用环境变量作为 flag 默认值：

```go
var (
	headless bool
	port     string
	token    string
)
flag.BoolVar(&headless, "headless", true, "是否无头模式")
flag.StringVar(&port, "port", ":18060", "端口")
flag.StringVar(&token, "token", os.Getenv("AUTH_TOKEN"), "鉴权 Token，留空则关闭鉴权")
flag.Parse()
```

把应用服务器构造改为：

```go
appServer := NewAppServer(xiaohongshuService, token)
```

不要记录 `token` 的值。Go flag 会让显式 `-token=...` 覆盖从 `AUTH_TOKEN` 取得的默认值，显式 `-token=` 也可关闭环境变量提供的鉴权。

- [ ] **Step 6: 格式化并运行路由测试**

Run: `gofmt -w main.go app_server.go routes.go routes_test.go`

Run: `go test ./... -run 'Test(MCPStatelessSinglePost|NotificationToolsRegistered|NotificationRoutesRegistered|ProtectedRoutesRequireBearerToken|MCPAcceptsConfiguredBearerToken)$'`

Expected: PASS，旧的无 Token 契约和新的受保护路由契约同时通过。

- [ ] **Step 7: 提交启动配置和路由接入**

```bash
git add main.go app_server.go routes.go routes_test.go
git commit -m "feat: protect HTTP and MCP routes with bearer auth"
```

---

### Task 3: Docker 透传和用户文档

**Files:**
- Modify: `Dockerfile:65-73`
- Modify: `docker/docker-compose.yml:13-17`
- Modify: `README.md:464-560`
- Modify: `README_EN.md:468-564`
- Modify: `docker/README.md:96-127`

**Interfaces:**
- Consumes: `AUTH_TOKEN` and `-token` configuration from Task 2.
- Produces: Docker runtime configuration and copy-pasteable client examples; no Go interfaces.

- [ ] **Step 1: 给 Docker 镜像和 Compose 增加空默认配置**

在 Dockerfile 的运行阶段环境变量区域增加：

```dockerfile
ENV AUTH_TOKEN=""
```

在 `docker/docker-compose.yml` 的 `environment` 中增加：

```yaml
      - AUTH_TOKEN=${AUTH_TOKEN:-}
```

不得使用 `ARG`、`RUN` 或字面量真实 Token，以免密钥进入镜像层或仓库。

- [ ] **Step 2: 验证 Compose 默认关闭并能透传配置**

Run: `docker compose -f docker/docker-compose.yml config`

Expected: 命令成功，渲染结果包含空的 `AUTH_TOKEN`，且不包含示例密钥。

Run: `AUTH_TOKEN=compose-test-token docker compose -f docker/docker-compose.yml config`

Expected: 命令成功，渲染结果的 `AUTH_TOKEN` 为 `compose-test-token`。

- [ ] **Step 3: 更新中文 README**

在 `README.md` 的代理配置之后新增“访问鉴权（可选）”，明确默认关闭、生产推荐环境变量、启动参数优先，并加入：

```bash
# 环境变量
AUTH_TOKEN=your-secret-token ./xiaohongshu-mcp-darwin-arm64
AUTH_TOKEN=your-secret-token go run .

# 启动参数（优先于环境变量）
./xiaohongshu-mcp-darwin-arm64 -token=your-secret-token
go run . -token=your-secret-token
```

在 HTTP/MCP 验证 curl 中增加：

```bash
-H "Authorization: Bearer your-secret-token"
```

并说明启用鉴权后，所有 MCP 客户端都必须配置自定义请求头 `Authorization: Bearer <token>`；命令行参数可能被进程列表看到，部署环境优先使用 `AUTH_TOKEN`。

- [ ] **Step 4: 同步英文 README**

在 `README_EN.md` 对应位置加入同样的命令和英文说明，术语固定为 `optional authentication`、`AUTH_TOKEN`、`Authorization: Bearer <token>`，curl 示例与中文文档一致。

- [ ] **Step 5: 更新 Docker 使用说明**

在 `docker/README.md` 增加独立“配置访问鉴权（可选）”章节，包含：

```bash
docker run -e AUTH_TOKEN=your-secret-token -p 18060:18060 xpzouying/xiaohongshu-mcp

AUTH_TOKEN=your-secret-token docker compose up -d
```

说明不设置或设为空时关闭鉴权，Compose 通过 `${AUTH_TOKEN:-}` 读取宿主环境变量，并给出客户端请求头格式。

- [ ] **Step 6: 检查文档和配置一致性**

Run: `rg -n 'XHS_TOKEN|AUTH_TOKEN|Authorization: Bearer|token=' Dockerfile docker/docker-compose.yml docker/README.md README.md README_EN.md`

Expected: 不出现 `XHS_TOKEN`；所有新增配置统一使用 `AUTH_TOKEN`，三个文档都包含 Bearer 请求头示例。

Run: `git diff --check`

Expected: PASS，无尾随空格或 Markdown 空白错误。

- [ ] **Step 7: 提交 Docker 和文档**

```bash
git add Dockerfile docker/docker-compose.yml docker/README.md README.md README_EN.md
git commit -m "docs: document static token authentication"
```

---

### Task 4: 全量验证和本地 Review

**Files:**
- Review: all files changed since `main`

**Interfaces:**
- Consumes: Tasks 1-3 的全部实现。
- Produces: 可供用户本地 review 的已验证分支；不创建新接口。

- [ ] **Step 1: 检查工作区和改动范围**

Run: `git status --short`

Expected: 无未提交文件。

Run: `git diff --stat main...HEAD && git diff --check main...HEAD`

Expected: 仅包含设计/计划文档、鉴权实现与测试、Docker 配置和相关 README；`git diff --check` 成功。

- [ ] **Step 2: 运行全部 Go 测试**

Run: `go test ./...`

Expected: PASS，所有包测试通过。

- [ ] **Step 3: 运行静态检查和构建**

Run: `go vet ./...`

Expected: PASS，无诊断。

Run: `go build ./...`

Expected: PASS，所有 Go 包构建成功；删除命令意外产生且不需要提交的构建中间文件。

- [ ] **Step 4: 检查敏感信息与鉴权边界**

Run: `rg -n 'secret-token|compose-test-token|your-secret-token' --glob '!docs/superpowers/**' .`

Expected: `secret-token` 只出现在测试中，`your-secret-token` 只出现在用户文档中，`compose-test-token` 不出现在任何已提交文件；不存在真实密钥。

人工检查 `git diff main...HEAD`，确认 `/health` 在受保护组之外，CORS 位于鉴权之前，HTTP API 和两个 MCP 路由都挂到同一受保护组，任何日志语句都不打印 Token。

- [ ] **Step 5: 准备本地 Review 摘要**

向用户报告：变更文件、配置优先级、受保护/公开路由、测试命令及结果、当前本地分支与提交列表。不要推送远程；远程 PR review 等用户后续明确授权创建或推送 MR/PR 后再执行。
