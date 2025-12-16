package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const envCookiesPath = "COOKIES_PATH"

// DerivedPaths 派生路径
type DerivedPaths struct {
	CookiesPath string
	UserDataDir string
	LogFile     string
	HealthURL   string
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	Running   bool
	PID       int
	StartedAt string
	LastError string
}

// StartUserParams 启动参数
type StartUserParams struct {
	User     UserConfig
	BinPath  string
	Headless bool
	DataDir  string
}

type runningProc struct {
	cmd       *exec.Cmd
	logFile   *os.File
	startedAt time.Time
	lastError string
	done      chan error
}

// ProcessManager 进程管理器
type ProcessManager struct {
	mu    sync.RWMutex
	procs map[string]*runningProc
}

// NewProcessManager 创建进程管理器
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		procs: map[string]*runningProc{},
	}
}

// DerivePaths 派生路径
func (pm *ProcessManager) DerivePaths(dataDir, userID string, port int) DerivedPaths {
	return DerivedPaths{
		CookiesPath: filepath.Join(dataDir, "cookies", userID+".json"),
		UserDataDir: filepath.Join(dataDir, "profiles", userID),
		LogFile:     filepath.Join(dataDir, "logs", userID+".log"),
		HealthURL:   fmt.Sprintf("http://127.0.0.1:%d/health", port),
	}
}

// GetStatus 获取进程状态
func (pm *ProcessManager) GetStatus(userID string) ProcessStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.procs[userID]
	if !ok || p == nil || p.cmd == nil || p.cmd.Process == nil {
		return ProcessStatus{Running: false}
	}
	return ProcessStatus{
		Running:   true,
		PID:       p.cmd.Process.Pid,
		StartedAt: p.startedAt.Format(time.RFC3339),
		LastError: p.lastError,
	}
}

// StartUser 启动用户进程
func (pm *ProcessManager) StartUser(ctx context.Context, params StartUserParams) error {
	if params.User.ID == "" {
		return fmt.Errorf("id 不能为空")
	}
	if params.User.Port <= 0 || params.User.Port > 65535 {
		return fmt.Errorf("port 非法: %d", params.User.Port)
	}
	if params.BinPath == "" {
		return fmt.Errorf("bin 不能为空")
	}
	if params.DataDir == "" {
		return fmt.Errorf("data_dir 不能为空")
	}

	pm.mu.Lock()
	if _, ok := pm.procs[params.User.ID]; ok {
		pm.mu.Unlock()
		return fmt.Errorf("用户进程已在运行")
	}
	pm.mu.Unlock()

	paths := pm.DerivePaths(params.DataDir, params.User.ID, params.User.Port)
	if err := ensureDirs(paths); err != nil {
		return err
	}

	logFile, err := os.OpenFile(paths.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	args := []string{
		"-headless=" + strconv.FormatBool(params.Headless),
		"-port=:" + strconv.Itoa(params.User.Port),
		"-user-data-dir=" + paths.UserDataDir,
	}
	if params.User.Proxy != "" {
		args = append(args, "-proxy="+params.User.Proxy)
	}

	cmd := exec.Command(params.BinPath, args...)
	cmd.Env = append(os.Environ(), envCookiesPath+"="+paths.CookiesPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("启动子进程失败: %w", err)
	}

	rp := &runningProc{
		cmd:       cmd,
		logFile:   logFile,
		startedAt: time.Now(),
		done:      make(chan error, 1),
	}

	pm.mu.Lock()
	pm.procs[params.User.ID] = rp
	pm.mu.Unlock()

	go func(userID string, p *runningProc) {
		err := cmd.Wait()
		_ = logFile.Close()
		pm.mu.Lock()
		if err != nil {
			p.lastError = err.Error()
		}
		delete(pm.procs, userID)
		pm.mu.Unlock()
		p.done <- err
	}(params.User.ID, rp)

	// 启动后健康检查
	if err := pm.waitHealthy(ctx, params.User.Port, 30*time.Second, 500*time.Millisecond); err != nil {
		_ = pm.StopUser(context.Background(), params.User.ID, 10*time.Second)
		return err
	}
	return nil
}

// StopUser 停止用户进程
func (pm *ProcessManager) StopUser(ctx context.Context, userID string, timeout time.Duration) error {
	pm.mu.RLock()
	p, ok := pm.procs[userID]
	pm.mu.RUnlock()
	if !ok || p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	_ = p.cmd.Process.Signal(os.Interrupt)

	select {
	case <-ctx.Done():
		_ = p.cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		return nil
	case <-p.done:
		return nil
	}
}

// StopAll 停止所有进程
func (pm *ProcessManager) StopAll(ctx context.Context, stopTimeout time.Duration) error {
	pm.mu.RLock()
	ids := make([]string, 0, len(pm.procs))
	for id := range pm.procs {
		ids = append(ids, id)
	}
	pm.mu.RUnlock()

	for _, id := range ids {
		_ = pm.StopUser(ctx, id, stopTimeout)
	}
	return nil
}

// CheckHealth 检查健康状态
func (pm *ProcessManager) CheckHealth(port int, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	u := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (pm *ProcessManager) waitHealthy(ctx context.Context, port int, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if pm.CheckHealth(port, 2*time.Second) {
			return nil
		}
		lastErr = fmt.Errorf("health 失败: http://127.0.0.1:%d/health", port)
		time.Sleep(interval)
	}
	return fmt.Errorf("启动超时(%s): %v", timeout, lastErr)
}

func ensureDirs(p DerivedPaths) error {
	if err := os.MkdirAll(filepath.Dir(p.CookiesPath), 0755); err != nil {
		return fmt.Errorf("创建 cookies 目录失败: %w", err)
	}
	if err := os.MkdirAll(p.UserDataDir, 0755); err != nil {
		return fmt.Errorf("创建 user-data-dir 目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p.LogFile), 0755); err != nil {
		return fmt.Errorf("创建 logs 目录失败: %w", err)
	}
	return nil
}
