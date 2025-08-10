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

![运行 Inspector](./assets/mcp-inspector.png)

运行后，打开红色标记的链接，配置 MCP inspector，输入 `http://localhost:18060/mcp` ，点击 `Connect` 按钮。

## 使用 MCP 发布

### 检查登录状态

<video width="600" controls>
  <source src="./assets/check_login.mp4" type="video/mp4">
  Your browser does not support the video tag.
</video>

### 发布图文

示例中是从 https://unsplash.com/ 中随机找了个图片做测试。

<video width="600" controls>
  <source src="./assets/inspect_mcp_publish.mp4" type="video/mp4">
  Your browser does not support the video tag.
</video>
