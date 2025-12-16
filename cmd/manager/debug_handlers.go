package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DebugSummary 调试汇总信息
type DebugSummary struct {
	User    DebugUserInfo   `json:"user"`
	Login   DebugLoginInfo  `json:"login"`
	Cookies DebugCookieInfo `json:"cookies"`
	MCP     DebugMCPInfo    `json:"mcp"`
}

// DebugUserInfo 用户信息
type DebugUserInfo struct {
	ID       string `json:"id"`
	Port     int    `json:"port"`
	Running  bool   `json:"running"`
	HealthOK bool   `json:"health_ok"`
	URL      string `json:"url"`
}

// DebugLoginInfo 登录信息
type DebugLoginInfo struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	Username   string `json:"username,omitempty"`
}

// DebugCookieInfo Cookie信息
type DebugCookieInfo struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	SizeBytes    int64  `json:"size_bytes"`
	Mtime        string `json:"mtime,omitempty"`
	Count        int    `json:"count"`
	MinExpiresAt string `json:"min_expires_at,omitempty"`
	MaxExpiresAt string `json:"max_expires_at,omitempty"`
}

// DebugMCPInfo MCP信息
type DebugMCPInfo struct {
	Reachable bool `json:"reachable"`
}

// MCPToolInfo MCP工具信息
type MCPToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// MCPCallRequest MCP调用请求
type MCPCallRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	TimeoutMs int            `json:"timeout_ms,omitempty"`
}

// MCPCallResponse MCP调用响应
type MCPCallResponse struct {
	Content []MCPContent `json:"content,omitempty"`
	IsError bool         `json:"isError"`
}

// MCPContent MCP内容
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// GetDebugSummary 获取调试汇总
func (a *App) GetDebugSummary(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	dataDir := a.store.ResolveDataDir()
	paths := a.proc.DerivePaths(dataDir, id, user.Port)
	st := a.proc.GetStatus(id)
	healthOK := false
	if st.Running {
		healthOK = a.proc.CheckHealth(user.Port, 800*time.Millisecond)
	}

	summary := DebugSummary{
		User: DebugUserInfo{
			ID:       id,
			Port:     user.Port,
			Running:  st.Running,
			HealthOK: healthOK,
			URL:      fmt.Sprintf("http://127.0.0.1:%d", user.Port),
		},
	}

	// 获取登录状态
	if st.Running && healthOK {
		loginInfo := a.fetchLoginStatus(c.Request.Context(), user.Port)
		summary.Login = loginInfo
	}

	// 获取Cookie状态
	summary.Cookies = a.getCookieStatus(paths.CookiesPath)

	// 检查MCP可达性
	if st.Running && healthOK {
		summary.MCP.Reachable = a.checkMCPReachable(c.Request.Context(), user.Port)
	}

	c.JSON(http.StatusOK, summary)
}

// GetDebugLoginQRCode 获取登录二维码
func (a *App) GetDebugLoginQRCode(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	st := a.proc.GetStatus(id)
	if !st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程未运行"})
		return
	}

	// 健康检查
	if !a.proc.CheckHealth(user.Port, 800*time.Millisecond) {
		c.JSON(http.StatusConflict, gin.H{"error": "用户实例健康检查失败，请稍后重试"})
		return
	}

	// 转发到用户实例
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/login/qrcode", user.Port)
	status, contentType, data, err := a.proxyGet(c.Request.Context(), url, 60*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("转发请求失败: %v", err)})
		return
	}

	c.Data(status, contentType, data)
}

// GetDebugLoginStatus 获取登录状态
func (a *App) GetDebugLoginStatus(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	st := a.proc.GetStatus(id)
	if !st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程未运行"})
		return
	}

	// 健康检查
	if !a.proc.CheckHealth(user.Port, 800*time.Millisecond) {
		c.JSON(http.StatusConflict, gin.H{"error": "用户实例健康检查失败，请稍后重试"})
		return
	}

	// 转发到用户实例
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/login/status", user.Port)
	status, contentType, data, err := a.proxyGet(c.Request.Context(), url, 10*time.Second)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("转发请求失败: %v", err)})
		return
	}

	c.Data(status, contentType, data)
}

// GetDebugCookies 获取Cookie状态
func (a *App) GetDebugCookies(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	dataDir := a.store.ResolveDataDir()
	paths := a.proc.DerivePaths(dataDir, id, user.Port)
	info := a.getCookieStatus(paths.CookiesPath)

	c.JSON(http.StatusOK, info)
}

// DeleteDebugCookies 删除Cookie
func (a *App) DeleteDebugCookies(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	dataDir := a.store.ResolveDataDir()
	paths := a.proc.DerivePaths(dataDir, id, user.Port)
	st := a.proc.GetStatus(id)

	var mode string
	var message string

	// 如果用户实例运行中，优先调用用户实例API
	if st.Running && a.proc.CheckHealth(user.Port, 800*time.Millisecond) {
		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/login/cookies", user.Port)
		err := a.proxyDelete(c.Request.Context(), url, 10*time.Second)
		if err != nil {
			// 回退到直接删除文件
			if err := os.Remove(paths.CookiesPath); err != nil && !os.IsNotExist(err) {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除Cookie失败: %v", err)})
				return
			}
			mode = "file"
			message = "用户实例API调用失败，已直接删除Cookie文件"
		} else {
			mode = "upstream"
			message = "已通过用户实例API删除Cookie"
		}
	} else {
		// 直接删除文件
		if err := os.Remove(paths.CookiesPath); err != nil && !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除Cookie失败: %v", err)})
			return
		}
		mode = "file"
		message = "已直接删除Cookie文件"
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted":     true,
		"mode":        mode,
		"cookie_path": paths.CookiesPath,
		"message":     message,
	})
}

// GetDebugMCPTools 获取MCP工具列表
func (a *App) GetDebugMCPTools(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	st := a.proc.GetStatus(id)
	if !st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程未运行"})
		return
	}

	// 健康检查
	if !a.proc.CheckHealth(user.Port, 800*time.Millisecond) {
		c.JSON(http.StatusConflict, gin.H{"error": "用户实例健康检查失败，请稍后重试"})
		return
	}

	tools, err := a.fetchMCPTools(c.Request.Context(), user.Port)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("获取MCP工具列表失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

// PostDebugMCPCall 调用MCP工具
func (a *App) PostDebugMCPCall(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}

	user, ok := a.store.GetUser(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	st := a.proc.GetStatus(id)
	if !st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程未运行"})
		return
	}

	// 健康检查
	if !a.proc.CheckHealth(user.Port, 800*time.Millisecond) {
		c.JSON(http.StatusConflict, gin.H{"error": "用户实例健康检查失败，请稍后重试"})
		return
	}

	var req MCPCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 JSON"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工具名称不能为空"})
		return
	}

	// 限制超时时间
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > 2*time.Minute {
		timeout = 30 * time.Second
	}

	result, err := a.callMCPTool(c.Request.Context(), user.Port, req.Name, req.Arguments, timeout)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("调用MCP工具失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, result)
}

// 辅助方法

func (a *App) fetchLoginStatus(ctx context.Context, port int) DebugLoginInfo {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/login/status", port)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	status, _, data, err := a.proxyGet(ctx, url, 5*time.Second)
	if err != nil || status >= 400 {
		return DebugLoginInfo{}
	}

	// 解析响应
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			IsLoggedIn bool   `json:"is_logged_in"`
			Username   string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return DebugLoginInfo{}
	}
	if !resp.Success {
		return DebugLoginInfo{}
	}

	return DebugLoginInfo{
		IsLoggedIn: resp.Data.IsLoggedIn,
		Username:   resp.Data.Username,
	}
}

func (a *App) getCookieStatus(cookiePath string) DebugCookieInfo {
	info := DebugCookieInfo{
		Path: cookiePath,
	}

	stat, err := os.Stat(cookiePath)
	if os.IsNotExist(err) {
		return info
	}
	if err != nil {
		return info
	}

	info.Exists = true
	info.SizeBytes = stat.Size()
	info.Mtime = stat.ModTime().Format(time.RFC3339)

	// 解析Cookie文件获取详细信息
	data, err := os.ReadFile(cookiePath)
	if err != nil {
		return info
	}

	var cookies []map[string]any
	if err := json.Unmarshal(data, &cookies); err != nil {
		return info
	}

	info.Count = len(cookies)

	// 计算过期时间范围
	var minExpires, maxExpires float64
	for _, cookie := range cookies {
		if expires, ok := cookie["expires"].(float64); ok && expires > 0 {
			if minExpires == 0 || expires < minExpires {
				minExpires = expires
			}
			if expires > maxExpires {
				maxExpires = expires
			}
		}
	}

	if minExpires > 0 {
		info.MinExpiresAt = time.Unix(int64(minExpires), 0).Format(time.RFC3339)
	}
	if maxExpires > 0 {
		info.MaxExpiresAt = time.Unix(int64(maxExpires), 0).Format(time.RFC3339)
	}

	return info
}

func (a *App) checkMCPReachable(ctx context.Context, port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodOptions, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

// proxyGet 转发GET请求，返回状态码、Content-Type和响应体
func (a *App) proxyGet(ctx context.Context, url string, timeout time.Duration) (int, string, []byte, error) {
	return a.proxyRequest(ctx, http.MethodGet, url, timeout)
}

// proxyDelete 转发DELETE请求
func (a *App) proxyDelete(ctx context.Context, url string, timeout time.Duration) error {
	status, _, body, err := a.proxyRequest(ctx, http.MethodDelete, url, timeout)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("HTTP %d: %s", status, string(body))
	}
	return nil
}

// proxyRequest 通用HTTP请求转发
func (a *App) proxyRequest(ctx context.Context, method, url string, timeout time.Duration) (int, string, []byte, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return resp.StatusCode, contentType, body, nil
}
