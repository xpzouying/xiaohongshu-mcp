# 修复 MCP HTTP 会话管理问题

## 问题描述

原来的实现中，MCP Server 在处理 HTTP 请求时没有正确维护会话状态。每个 HTTP POST 请求都被当作一个全新的会话，导致 MCP 协议的状态机无法正常工作。

具体表现为：
- 客户端发送 `initialize` 请求成功
- 客户端发送 `notifications/initialized` 通知
- 客户端调用 `tools/call` 时报错：`method "tools/call" is invalid during session initialization`

## 根本原因

MCP 协议基于 JSON-RPC，使用状态机来管理会话生命周期：

```
[未初始化] --initialize--> [初始化中] --initialized--> [已就绪] --tools/call--> [执行工具]
```

但原实现中，`StreamableHTTPHandler` 对每个 HTTP 请求都返回同一个全局的 `mcp.Server` 实例，而该实例在处理完一个请求后状态不会保留到下一个请求。

## 解决方案

实现了一个 `SessionManager` 来跨 HTTP 请求维护会话状态：

1. **新增 session_manager.go**：会话管理器
   - 为每个唯一会话ID维护独立的 `mcp.Server` 实例
   - 使用线程安全的 map 存储会话
   - 支持会话的创建和清理

2. **更新 app_server.go**：集成会话管理器
   - 在 `AppServer` 中添加 `sessionManager` 字段
   - 在初始化时创建会话管理器实例

3. **更新 routes.go**：使用会话ID
   - 从 HTTP Header 中读取 `X-Session-Id`
   - 如果未提供，使用 `RemoteAddr` 作为默认会话标识
   - 调用 `sessionManager.GetOrCreateSession()` 获取对应的 server 实例

## 客户端使用方法

HTTP 客户端需要在所有请求中携带相同的 `X-Session-Id` header：

```python
import uuid

# 在脚本开始时生成唯一的会话ID
SESSION_ID = str(uuid.uuid4())

# 在每个请求中添加该 header
headers = {
    'Content-Type': 'application/json',
    'Accept': 'application/json, text/event-stream',
    'X-Session-Id': SESSION_ID
}
```

## 测试步骤

1. 构建并启动服务器：
```bash
go build -o xiaohongshu-mcp
./xiaohongshu-mcp
```

2. 使用修改后的 Python 客户端测试：
```bash
python3 publish_mcp.py
```

预期行为：
- ✅ initialize 成功
- ✅ notifications/initialized 成功
- ✅ tools/list 返回工具列表
- ✅ tools/call 可以成功调用工具（例如 publish_content）

## 后续优化建议

1. **会话超时清理**：实现定时器自动清理长时间不活跃的会话
2. **会话限制**：添加最大会话数限制，防止内存泄漏
3. **持久化**：考虑将会话状态持久化到 Redis 等存储
4. **监控**：添加会话数量、创建/销毁速率等监控指标

## 兼容性

此修复向后兼容：
- 如果客户端不提供 `X-Session-Id`，将使用 `RemoteAddr` 作为会话标识
- 对于长连接客户端（如 SSE），行为不变
- 对于短连接客户端（如 HTTP POST），现在可以通过会话ID维护状态
