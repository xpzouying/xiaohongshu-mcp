package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// ImportDebugCookies 导入Cookies（用于从老版本迁移）
func (a *App) ImportDebugCookies(c *gin.Context) {
	const maxBodyBytes = 5 << 20 // 5MB

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

	// 读取请求体，限制大小
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}
	if len(raw) > maxBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "文件过大（最大 5MB）"})
		return
	}

	// 兼容带BOM的JSON
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不能为空（需要 JSON 数组）"})
		return
	}

	// 验证JSON格式：必须是数组，元素必须是对象
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 JSON：需要 cookies 数组"})
		return
	}
	for i, ck := range arr {
		if ck == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("第 %d 条 cookie 不是对象", i+1)})
			return
		}
		// 严格类型校验：name字段必须是非空字符串
		name, ok := ck["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("第 %d 条 cookie 缺少有效的 name 字段", i+1)})
			return
		}
	}

	// 获取cookie文件路径
	dataDir := a.store.ResolveDataDir()
	paths := a.proc.DerivePaths(dataDir, id, user.Port)

	// 确保目录存在
	if dir := filepath.Dir(paths.CookiesPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建目录失败: %v", err)})
			return
		}
	}

	// 序列化并保存（原子写入：先写临时文件再重命名）
	normalized, err := json.Marshal(arr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JSON 序列化失败"})
		return
	}
	tmpPath := paths.CookiesPath + ".tmp"
	if err := os.WriteFile(tmpPath, normalized, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存 cookies 失败: %v", err)})
		return
	}
	if err := os.Rename(tmpPath, paths.CookiesPath); err != nil {
		_ = os.Remove(tmpPath) // 清理临时文件
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存 cookies 失败: %v", err)})
		return
	}

	st := a.proc.GetStatus(id)
	needRestart := st.Running
	message := "Cookies 已导入并保存"
	if needRestart {
		message = "Cookies 已导入并保存，当前用户实例正在运行，需重启后生效"
	}

	c.JSON(http.StatusOK, gin.H{
		"imported":     true,
		"count":        len(arr),
		"cookie_path":  paths.CookiesPath,
		"running":      st.Running,
		"need_restart": needRestart,
		"message":      message,
	})
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
	// 登录状态检查需要启动浏览器并导航页面，增加超时到30秒
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	status, _, data, err := a.proxyGet(ctx, url, 30*time.Second)
	if err != nil {
		fmt.Printf("[DEBUG] fetchLoginStatus 请求失败: port=%d err=%v\n", port, err)
		return DebugLoginInfo{}
	}
	if status >= 400 {
		fmt.Printf("[DEBUG] fetchLoginStatus 状态码异常: port=%d status=%d body=%s\n", port, status, string(data))
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
		fmt.Printf("[DEBUG] fetchLoginStatus JSON解析失败: port=%d err=%v body=%s\n", port, err, string(data))
		return DebugLoginInfo{}
	}
	if !resp.Success {
		fmt.Printf("[DEBUG] fetchLoginStatus success=false: port=%d body=%s\n", port, string(data))
		return DebugLoginInfo{}
	}

	fmt.Printf("[DEBUG] fetchLoginStatus 成功: port=%d is_logged_in=%v username=%s\n", port, resp.Data.IsLoggedIn, resp.Data.Username)
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

// LogsResponse 日志响应
type LogsResponse struct {
	LogFile    string `json:"log_file"`
	Exists     bool   `json:"exists"`
	SizeBytes  int64  `json:"size_bytes"`
	Lines      int    `json:"lines"`
	Content    string `json:"content"`
	Truncated  bool   `json:"truncated"`
	TotalLines int    `json:"total_lines"`
}

// GetDebugLogs 获取用户实例日志
func (a *App) GetDebugLogs(c *gin.Context) {
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

	resp := LogsResponse{
		LogFile: paths.LogFile,
	}

	stat, err := os.Stat(paths.LogFile)
	if os.IsNotExist(err) {
		c.JSON(http.StatusOK, resp)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取日志文件状态失败: %v", err)})
		return
	}

	resp.Exists = true
	resp.SizeBytes = stat.Size()

	// 读取最后N行，默认200行，最大1000行
	lines := 200
	if linesParam := c.Query("lines"); linesParam != "" {
		if n, err := strconv.Atoi(linesParam); err == nil && n > 0 {
			if n > 1000 {
				n = 1000
			}
			lines = n
		}
	}

	content, totalLines, err := readLastLines(paths.LogFile, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取日志失败: %v", err)})
		return
	}

	resp.Content = content
	resp.TotalLines = totalLines

	// 处理大文件模式（totalLines=-1表示未知总行数）
	if totalLines < 0 {
		// 计算实际返回的行数
		actualLines := strings.Count(content, "\n")
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			actualLines++
		}
		resp.Lines = actualLines
		resp.Truncated = true // 大文件模式始终标记为截断
	} else {
		resp.Lines = min(lines, totalLines)
		resp.Truncated = totalLines > lines
	}

	c.JSON(http.StatusOK, resp)
}

// readLastLines 读取文件最后N行
// 对小文件直接读取，对大文件只读取末尾部分以避免OOM
func readLastLines(filePath string, n int) (string, int, error) {
	const smallFileThreshold = 2 * 1024 * 1024 // 2MB
	const tailReadSize = 512 * 1024            // 大文件只读末尾512KB

	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	fileSize := stat.Size()

	var data []byte
	var totalLines int
	var isPartial bool

	if fileSize <= smallFileThreshold {
		// 小文件：直接读取全部
		data, err = io.ReadAll(file)
		if err != nil {
			return "", 0, err
		}
	} else {
		// 大文件：只读取末尾部分
		isPartial = true
		readSize := int64(tailReadSize)
		if readSize > fileSize {
			readSize = fileSize
		}
		_, err = file.Seek(-readSize, io.SeekEnd)
		if err != nil {
			return "", 0, err
		}
		data, err = io.ReadAll(file)
		if err != nil {
			return "", 0, err
		}
		// 跳过第一个不完整的行
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx < len(data)-1 {
			data = data[idx+1:]
		}
	}

	allLines := strings.Split(string(data), "\n")

	// 移除末尾空行
	for len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	totalLines = len(allLines)
	if isPartial {
		// 大文件无法准确统计总行数，返回-1表示未知
		totalLines = -1
	}

	if len(allLines) <= n {
		return strings.Join(allLines, "\n"), totalLines, nil
	}

	return strings.Join(allLines[len(allLines)-n:], "\n"), totalLines, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
