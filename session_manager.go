package main

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SessionManager 管理MCP会话状态
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*mcp.Server
	appServer *AppServer
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager(appServer *AppServer) *SessionManager {
	return &SessionManager{
		sessions:  make(map[string]*mcp.Server),
		appServer: appServer,
	}
}

// GetOrCreateSession 获取或创建会话
func (sm *SessionManager) GetOrCreateSession(sessionID string) *mcp.Server {
	sm.mu.RLock()
	server, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if exists {
		return server
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 再次检查，避免竞态条件
	server, exists = sm.sessions[sessionID]
	if exists {
		return server
	}

	// 为每个会话创建新的MCP Server实例
	server = InitMCPServer(sm.appServer)
	sm.sessions[sessionID] = server

	return server
}

// RemoveSession 删除会话
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}
