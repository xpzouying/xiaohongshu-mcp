# xiaohongshu-mcp

MCP for xiaohongshu.com

## 使用教程

### 登录

第一次需要手动登录，需要保存小红书的登录状态。

运行

```bash
go run cmd/login/main.go
```

### 启动 MCP 服务

启动 xiaohongshu-mcp 服务。

```bash
go run . -headless=false
```

## 验证 MCP

```bash
npx @modelcontextprotocol/inspector
```

![运行 Inspector](./assets/run_inspect.png)

运行后，打开红色标记的链接，配置 MCP inspector，输入 `http://localhost:18060/mcp` ，点击 `Connect` 按钮。

![配置 MCP inspector](./assets/inspect_mcp.png)

按照上面配置 MCP inspector 后，点击 `List Tools` 按钮，查看所有的 Tools。

## 使用 MCP 发布

### 检查登录状态

![检查登录状态](./assets/check_login.gif)

### 发布图文

示例中是从 https://unsplash.com/ 中随机找了个图片做测试。

![发布图文](./assets/inspect_mcp_publish.gif)
