package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
)

// 用户 ID 只允许字母、数字、下划线、连字符
var validIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// UserConfig 用户配置
type UserConfig struct {
	ID    string `json:"id"`
	Port  int    `json:"port"`
	Proxy string `json:"proxy,omitempty"`
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	Bin      string       `json:"bin"`
	Headless bool         `json:"headless"`
	DataDir  string       `json:"data_dir"`
	Users    []UserConfig `json:"users"`
}

// Store JSON 存储
type Store struct {
	mu   sync.RWMutex
	path string
	cwd  string // 当前工作目录，用于解析相对路径
	cfg  ManagerConfig
}

// LoadStore 加载存储
func LoadStore(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("store 路径不能为空")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析 store 绝对路径失败: %w", err)
	}

	// 使用当前工作目录作为相对路径基准
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
	}

	s := &Store{
		path: absPath,
		cwd:  cwd,
		cfg: ManagerConfig{
			Bin:      "./xiaohongshu-mcp",
			Headless: true,
			DataDir:  "./data",
			Users:    []UserConfig{},
		},
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return nil, fmt.Errorf("创建 store 目录失败: %w", err)
		}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return s, nil
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取 store 失败: %w", err)
	}
	if len(raw) == 0 {
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return s, nil
	}

	var cfg ManagerConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	// 默认值兜底
	if cfg.Bin == "" {
		cfg.Bin = "./xiaohongshu-mcp"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	s.cfg = cfg

	if err := validateConfig(&s.cfg); err != nil {
		return nil, err
	}
	s.sortUsersLocked()
	return s, nil
}

// GetConfig 获取配置
func (s *Store) GetConfig() ManagerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// ResolveBinPath 解析可执行文件路径（相对于当前工作目录）
func (s *Store) ResolveBinPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return resolvePath(s.cwd, s.cfg.Bin)
}

// ResolveDataDir 解析数据目录（相对于当前工作目录）
func (s *Store) ResolveDataDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return resolvePath(s.cwd, s.cfg.DataDir)
}

// ListUsers 获取用户列表
func (s *Store) ListUsers() []UserConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserConfig, 0, len(s.cfg.Users))
	out = append(out, s.cfg.Users...)
	return out
}

// GetUser 获取用户
func (s *Store) GetUser(id string) (UserConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.cfg.Users {
		if u.ID == id {
			return u, true
		}
	}
	return UserConfig{}, false
}

// CreateUser 创建用户
func (s *Store) CreateUser(u UserConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateUser(u); err != nil {
		return err
	}
	for _, ex := range s.cfg.Users {
		if ex.ID == u.ID {
			return fmt.Errorf("用户已存在: %s", u.ID)
		}
		if ex.Port == u.Port {
			return fmt.Errorf("端口已被占用: %d", u.Port)
		}
	}
	s.cfg.Users = append(s.cfg.Users, u)
	s.sortUsersLocked()
	return s.saveLocked()
}

// UpdateUser 更新用户
func (s *Store) UpdateUser(id string, patch UserConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("id 不能为空")
	}
	if patch.ID != "" && patch.ID != id {
		return fmt.Errorf("不允许修改 id")
	}
	if patch.Port != 0 && (patch.Port <= 0 || patch.Port > 65535) {
		return fmt.Errorf("port 非法: %d", patch.Port)
	}

	found := false
	for i := range s.cfg.Users {
		if s.cfg.Users[i].ID != id {
			continue
		}
		found = true
		if patch.Port != 0 {
			for _, ex := range s.cfg.Users {
				if ex.ID != id && ex.Port == patch.Port {
					return fmt.Errorf("端口已被占用: %d", patch.Port)
				}
			}
			s.cfg.Users[i].Port = patch.Port
		}
		// 允许清空 proxy
		s.cfg.Users[i].Proxy = patch.Proxy
		break
	}
	if !found {
		return fmt.Errorf("用户不存在: %s", id)
	}
	s.sortUsersLocked()
	return s.saveLocked()
}

// DeleteUser 删除用户
func (s *Store) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("id 不能为空")
	}
	idx := -1
	for i, u := range s.cfg.Users {
		if u.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("用户不存在: %s", id)
	}
	s.cfg.Users = append(s.cfg.Users[:idx], s.cfg.Users[idx+1:]...)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := validateConfig(&s.cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("创建 store 目录失败: %w", err)
	}

	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	// 直接写入（Windows 兼容，简单化）
	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

func (s *Store) sortUsersLocked() {
	sort.Slice(s.cfg.Users, func(i, j int) bool {
		return s.cfg.Users[i].ID < s.cfg.Users[j].ID
	})
}

func resolvePath(baseDir, p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(baseDir, p))
}

func validateConfig(cfg *ManagerConfig) error {
	if cfg.Bin == "" {
		return fmt.Errorf("bin 不能为空")
	}
	if cfg.DataDir == "" {
		return fmt.Errorf("data_dir 不能为空")
	}

	seenID := map[string]struct{}{}
	seenPort := map[int]struct{}{}
	for i, u := range cfg.Users {
		if err := validateUser(u); err != nil {
			return fmt.Errorf("users[%d]: %w", i, err)
		}
		if _, ok := seenID[u.ID]; ok {
			return fmt.Errorf("id 重复: %s", u.ID)
		}
		seenID[u.ID] = struct{}{}
		if _, ok := seenPort[u.Port]; ok {
			return fmt.Errorf("port 重复: %d", u.Port)
		}
		seenPort[u.Port] = struct{}{}
	}
	return nil
}

func validateUser(u UserConfig) error {
	if u.ID == "" {
		return fmt.Errorf("id 不能为空")
	}
	if !validIDRegex.MatchString(u.ID) {
		return fmt.Errorf("id 只能包含字母、数字、下划线、连字符")
	}
	if u.Port <= 0 || u.Port > 65535 {
		return fmt.Errorf("port 非法: %d", u.Port)
	}
	return nil
}
