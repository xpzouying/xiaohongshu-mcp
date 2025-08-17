# MCP 服务接入指南

本文档介绍如何在各种 AI 客户端中接入小红书 MCP 服务。

## 🚀 快速开始

### 1. 启动 MCP 服务

```bash
# 启动服务（默认无头模式）
go run .

# 或者有界面模式
go run . -headless=false
```

服务将运行在：`http://localhost:18060/mcp`

### 2. 验证服务状态

```bash
# 测试 MCP 连接
curl -X POST http://localhost:18060/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}'
```

## 📱 客户端接入

### Claude Desktop

在 `~/.config/claude-desktop/claude_desktop_config.json` 中添加：

```json
{
  "mcpServers": {
    "xiaohongshu": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/client-stdio", "http://localhost:18060/mcp"],
      "env": {}
    }
  }
}
```

### Claude Code CLI

```bash
# 添加 HTTP MCP 服务器
claude mcp add --transport http xiaohongshu-mcp http://localhost:18060/mcp
```

### Cursor

在 Cursor 设置中添加 MCP 配置：

```json
{
  "mcp.servers": {
    "xiaohongshu": {
      "command": "node",
      "args": ["-e", "/* HTTP proxy script */"],
      "description": "小红书内容发布服务"
    }
  }
}
```

### VSCode

安装 MCP 扩展并配置：

1. 安装 [MCP for VSCode](https://marketplace.visualstudio.com/search?term=mcp&target=VSCode) 扩展
2. 在 VSCode 设置中添加配置（`Ctrl/Cmd + ,` → 搜索 "mcp"）：

```json
{
  "mcp.servers": {
    "xiaohongshu-mcp": {
      "command": "curl",
      "args": [
        "-X", "POST",
        "http://localhost:18060/mcp",
        "-H", "Content-Type: application/json",
        "-d", "@-"
      ],
      "description": "小红书内容发布和管理服务"
    }
  }
}
```

或者在工作区的 `.vscode/settings.json` 中添加：

```json
{
  "mcp.servers": {
    "xiaohongshu-mcp": {
      "transport": "http",
      "endpoint": "http://localhost:18060/mcp",
      "description": "小红书 MCP 服务"
    }
  }
}

### 通用 MCP Inspector（调试用）

```bash
# 启动 MCP Inspector
npx @modelcontextprotocol/inspector

# 在浏览器中连接到：http://localhost:18060/mcp
```

## 🛠️ 可用工具

连接成功后，可使用以下 MCP 工具：

- `check_login_status` - 检查小红书登录状态
- `publish_content` - 发布图文内容到小红书
- `list_feeds` - 获取小红书首页推荐列表

## 📝 使用示例

### 检查登录状态

```json
{
  "name": "check_login_status",
  "arguments": {}
}
```

### 发布内容

```json
{
  "name": "publish_content",
  "arguments": {
    "title": "标题",
    "content": "内容描述",
    "images": ["图片URL或本地路径"]
  }
}
```

## ⚠️ 注意事项

1. **首次使用需要登录**：运行 `go run cmd/login/main.go` 完成登录
2. **网络要求**：确保客户端能访问 `localhost:18060`
3. **权限验证**：某些操作需要有效的登录状态

## 🔧 故障排除

### 连接失败
- 检查服务是否运行：`curl http://localhost:18060/health`
- 确认端口未被占用
- 检查防火墙设置

### 工具调用失败
- 确认已完成小红书登录
- 检查图片URL或路径是否有效
- 查看服务日志获取详细错误信息
