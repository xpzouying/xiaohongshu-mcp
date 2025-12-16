package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// App 应用
type App struct {
	store     *Store
	proc      *ProcessManager
	indexHTML string
}

// NewApp 创建应用
func NewApp(store *Store, proc *ProcessManager, indexHTML string) *App {
	return &App{
		store:     store,
		proc:      proc,
		indexHTML: indexHTML,
	}
}

// HandleIndex 首页
func (a *App) HandleIndex(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, a.indexHTML)
}

type userView struct {
	ID    string `json:"id"`
	Port  int    `json:"port"`
	Proxy string `json:"proxy"`

	URL string `json:"url"`

	CookiesPath string `json:"cookies_path"`
	UserDataDir string `json:"user_data_dir"`
	LogFile     string `json:"log_file"`

	Running   bool   `json:"running"`
	PID       int    `json:"pid"`
	HealthOK  bool   `json:"health_ok"`
	StartedAt string `json:"started_at,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

type usersResponse struct {
	Bin      string     `json:"bin"`
	Headless bool       `json:"headless"`
	DataDir  string     `json:"data_dir"`
	Users    []userView `json:"users"`
}

// ListUsers 获取用户列表
func (a *App) ListUsers(c *gin.Context) {
	cfg := a.store.GetConfig()
	binPath := a.store.ResolveBinPath()
	dataDir := a.store.ResolveDataDir()
	users := a.store.ListUsers()

	out := make([]userView, 0, len(users))
	for _, u := range users {
		derived := a.proc.DerivePaths(dataDir, u.ID, u.Port)
		st := a.proc.GetStatus(u.ID)
		healthOK := false
		if st.Running {
			healthOK = a.proc.CheckHealth(u.Port, 800*time.Millisecond)
		}
		out = append(out, userView{
			ID:          u.ID,
			Port:        u.Port,
			Proxy:       u.Proxy,
			URL:         fmt.Sprintf("http://127.0.0.1:%d", u.Port),
			CookiesPath: derived.CookiesPath,
			UserDataDir: derived.UserDataDir,
			LogFile:     derived.LogFile,
			Running:     st.Running,
			PID:         st.PID,
			HealthOK:    healthOK,
			StartedAt:   st.StartedAt,
			LastError:   st.LastError,
		})
	}

	c.JSON(http.StatusOK, usersResponse{
		Bin:      binPath,
		Headless: cfg.Headless,
		DataDir:  dataDir,
		Users:    out,
	})
}

type createUserReq struct {
	ID    string `json:"id"`
	Port  int    `json:"port"`
	Proxy string `json:"proxy"`
}

// CreateUser 创建用户
func (a *App) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 JSON"})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Proxy = strings.TrimSpace(req.Proxy)

	if err := a.store.CreateUser(UserConfig{
		ID:    req.ID,
		Port:  req.Port,
		Proxy: req.Proxy,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusCreated)
}

type updateUserReq struct {
	Port  int    `json:"port"`
	Proxy string `json:"proxy"`
}

// UpdateUser 更新用户
func (a *App) UpdateUser(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}
	if st := a.proc.GetStatus(id); st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程运行中，请先停止再修改"})
		return
	}

	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 JSON"})
		return
	}
	req.Proxy = strings.TrimSpace(req.Proxy)

	if err := a.store.UpdateUser(id, UserConfig{
		ID:    id,
		Port:  req.Port,
		Proxy: req.Proxy,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteUser 删除用户
func (a *App) DeleteUser(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}
	if st := a.proc.GetStatus(id); st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程运行中，请先停止再删除"})
		return
	}
	if err := a.store.DeleteUser(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// StartUser 启动用户
func (a *App) StartUser(c *gin.Context) {
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
	if st := a.proc.GetStatus(id); st.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "用户进程已在运行"})
		return
	}

	cfg := a.store.GetConfig()
	binPath := a.store.ResolveBinPath()
	dataDir := a.store.ResolveDataDir()

	if err := a.proc.StartUser(c.Request.Context(), StartUserParams{
		User:     user,
		BinPath:  binPath,
		Headless: cfg.Headless,
		DataDir:  dataDir,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// StopUser 停止用户
func (a *App) StopUser(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不能为空"})
		return
	}
	if _, ok := a.store.GetUser(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err := a.proc.StopUser(c.Request.Context(), id, 10*time.Second); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
