# xiaohongshu-mcp

MCP for 小红书/xiaohongshu.com。

- 我的博客文章：[haha.ai/xiaohongshu-mcp](https://www.haha.ai/xiaohongshu-mcp)


## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=xpzouying/xiaohongshu-mcp&type=Timeline)](https://www.star-history.com/#xpzouying/xiaohongshu-mcp&Timeline)


**主要功能**

> 💡 **提示：** 点击下方功能标题可展开查看视频演示

<details>
<summary><b>1. 登录和检查登录状态</b></summary>

第一步必须，小红书需要进行登录。可以检查当前登录状态。

**登录演示：**

https://github.com/user-attachments/assets/8b05eb42-d437-41b7-9235-e2143f19e8b7

**检查登录状态演示：**

https://github.com/user-attachments/assets/bd9a9a4a-58cb-4421-b8f3-015f703ce1f9

</details>

<details>
<summary><b>2. 发布图文内容</b></summary>

支持发布图文内容到小红书，包括标题、内容描述和图片。后续支持更多的发布功能。

**发布图文帖子演示：**

https://github.com/user-attachments/assets/8aee0814-eb96-40af-b871-e66e6bbb6b06

</details>

<details>
<summary><b>3. 搜索内容</b></summary>

根据关键词搜索小红书内容。

**搜索帖子演示：**

https://github.com/user-attachments/assets/03c5077d-6160-4b18-b629-2e40933a1fd3

</details>

<details>
<summary><b>4. 获取推荐列表</b></summary>

获取小红书首页推荐内容列表。

**获取推荐列表演示：**

https://github.com/user-attachments/assets/110fc15d-46f2-4cca-bdad-9de5b5b8cc28

</details>

<details>
<summary><b>5. 获取帖子详情（包括互动数据和评论）</b></summary>

获取小红书帖子的完整详情，包括：
- 帖子内容（标题、描述、图片等）
- 用户信息
- 互动数据（点赞、收藏、分享、评论数）
- 评论列表及子评论

**⚠️ 重要提示：**
- 需要提供帖子ID和xsec_token（两个参数缺一不可）
- 这两个参数可以从Feed列表或搜索结果中获取
- 必须先登录才能使用此功能

**获取帖子详情演示：**

https://github.com/user-attachments/assets/76a26130-a216-4371-a6b3-937b8fda092a

</details>

<details>
<summary><b>6. 发表评论到帖子</b></summary>

支持自动发表评论到小红书帖子。

**功能说明：**
- 自动定位评论输入框
- 输入评论内容并发布
- 支持 HTTP API 和 MCP 工具调用

**⚠️ 重要提示：**
- 需要先登录才能使用此功能
- 需要提供帖子ID、xsec_token和评论内容
- 这些参数可以从Feed列表或搜索结果中获取

**发表评论演示：**

https://github.com/user-attachments/assets/cc385b6c-422c-489b-a5fc-63e92c695b80

</details>

**小红书基础运营知识**

- **标题：（非常重要）小红书要求标题不超过 20个字**
- 当前只支持图文发送：从推荐的角度看，图文的流量会比纯文字的更好。
- （低优先级）可以考虑视频和纯文字的支持。1. 个人感觉这两种会大大增加运营的复杂度；2. 这两类在我的使用场景的价值较低。
- Tags：马上支持。
- 根据本人实操，小红书每天的发帖量应该是 **50 篇**。
- **（非常重要）小红书的同一个账号不允许在多个网页端登录**，如果你登录了当前 xiaohongshu-mcp 后，就不要再在其他的网页端登录该账号，否则就会把当前 MCP 的账号“踢出登录”。你可以使用移动 App 端进行查看当前账号信息。

**风险说明**

1. 该项目是在自己的另外一个项目的基础上开源出来的，原来的项目稳定运行一年多，没有出现过封号的情况，只有出现过 Cookies 过期需要重新登录。
2. 我是使用 Claude Code 接入，稳定自动化运营数周后，验证没有问题后开源。

该项目是基于学习的目的，禁止一切违法行为。

**实操结果**

第一天点赞/收藏数达到了 999+，

<img width="386" height="278" alt="CleanShot 2025-09-05 at 01 31 55@2x" src="https://github.com/user-attachments/assets/4b5a283b-bd38-45b8-b608-8f818997366c" />

<img width="350" height="280" alt="CleanShot 2025-09-05 at 01 32 49@2x" src="https://github.com/user-attachments/assets/4481e1e7-3ef6-4bbd-8483-dcee8f77a8f2" />

一周左右的成果

<img width="1840" height="582" alt="CleanShot 2025-09-05 at 01 33 13@2x" src="https://github.com/user-attachments/assets/fb367944-dc48-4bbd-8ece-934caa86323e" />


## 1. 使用教程

### 1.1. 登录

第一次需要手动登录，需要保存小红书的登录状态。

运行

```bash
go run cmd/login/main.go
```

### 1.2. 启动 MCP 服务

启动 xiaohongshu-mcp 服务。

```bash

# 默认：无头模式，没有浏览器界面
go run .

# 非无头模式，有浏览器界面
go run . -headless=false
```

## 1.3. 验证 MCP

```bash
npx @modelcontextprotocol/inspector
```

![运行 Inspector](./assets/run_inspect.png)

运行后，打开红色标记的链接，配置 MCP inspector，输入 `http://localhost:18060/mcp` ，点击 `Connect` 按钮。

![配置 MCP inspector](./assets/inspect_mcp.png)

按照上面配置 MCP inspector 后，点击 `List Tools` 按钮，查看所有的 Tools。

## 1.4. 使用 MCP 发布

### 检查登录状态

![检查登录状态](./assets/check_login.gif)

### 发布图文

示例中是从 https://unsplash.com/ 中随机找了个图片做测试。

![发布图文](./assets/inspect_mcp_publish.gif)

### 搜索内容

使用搜索功能，根据关键词搜索小红书内容：

![搜索内容](./assets/search_result.png)

## 2. MCP 客户端接入

本服务支持标准的 Model Context Protocol (MCP)，可以接入各种支持 MCP 的 AI 客户端。


### 2.1. 快速开始

#### 启动 MCP 服务

```bash
# 启动服务（默认无头模式）
go run .

# 或者有界面模式
go run . -headless=false
```

服务将运行在：`http://localhost:18060/mcp`

#### 验证服务状态

```bash
# 测试 MCP 连接
curl -X POST http://localhost:18060/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}'
```

#### Claude Code CLI 接入

```bash
# 添加 HTTP MCP 服务器
claude mcp add --transport http xiaohongshu-mcp http://localhost:18060/mcp
```

### 2.2. 支持的客户端

<details>
<summary><b>Claude Code CLI</b></summary>

官方命令行工具，已在上面快速开始部分展示：

```bash
# 添加 HTTP MCP 服务器
claude mcp add --transport http xiaohongshu-mcp http://localhost:18060/mcp
```

</details>

<details>
<summary><b>Cursor</b></summary>

#### 配置文件的方式

创建或编辑 MCP 配置文件：

**项目级配置**（推荐）：
在项目根目录创建 `.cursor/mcp.json`：

```json
{
  "mcpServers": {
    "xiaohongshu-mcp": {
      "url": "http://localhost:18060/mcp",
      "description": "小红书内容发布服务 - MCP Streamable HTTP"
    }
  }
}
```

**全局配置**：
在用户目录创建 `~/.cursor/mcp.json` (同样内容)。

#### 使用步骤

1. 确保小红书 MCP 服务正在运行
2. 保存配置文件后，重启 Cursor
3. 在 Cursor 聊天中，工具应该自动可用
4. 可以通过聊天界面的 "Available Tools" 查看已连接的 MCP 工具

**Demo**

插件 MCP 接入：

![cursor_mcp_settings](./assets/cursor_mcp_settings.png)

调用 MCP 工具：（以检查登录状态为例）

![cursor_mcp_check_login](./assets/cursor_mcp_check_login.png)

</details>

<details>
<summary><b>VSCode</b></summary>

#### 方法一：使用命令面板配置

1. 按 `Ctrl/Cmd + Shift + P` 打开命令面板
2. 运行 `MCP: Add Server` 命令
3. 选择 `HTTP` 方式。
4. 输入地址： `http://localhost:18060/mcp`，或者修改成对应的 Server 地址。
5. 输入 MCP 名字： `xiaohongshu-mcp`。

#### 方法二：直接编辑配置文件

**工作区配置**（推荐）：
在项目根目录创建 `.vscode/mcp.json`：

```json
{
  "servers": {
    "xiaohongshu-mcp": {
      "url": "http://localhost:18060/mcp",
      "type": "http"
    }
  },
  "inputs": []
}
```

**查看配置**：

![vscode_config](./assets/vscode_mcp_config.png)

1. 确认运行状态。
2. 查看 `tools` 是否正确检测。

**Demo**

以搜索帖子内容为例：

![vscode_mcp_search](./assets/vscode_search_demo.png)

</details>

<details>
<summary><b>MCP Inspector</b></summary>

调试工具，用于测试 MCP 连接：

```bash
# 启动 MCP Inspector
npx @modelcontextprotocol/inspector

# 在浏览器中连接到：http://localhost:18060/mcp
```

使用步骤：
- 使用 MCP Inspector 测试连接
- 测试 Ping Server 功能验证连接
- 检查 List Tools 是否返回 6 个工具

</details>

<details>
<summary><b>其他支持 HTTP MCP 的客户端</b></summary>

任何支持 HTTP MCP 协议的客户端都可以连接到：`http://localhost:18060/mcp`

基本配置模板：
```json
{
  "name": "xiaohongshu-mcp",
  "url": "http://localhost:18060/mcp",
  "type": "http"
}
```

</details>

### 2.3. 可用 MCP 工具

连接成功后，可使用以下 MCP 工具：

- `check_login_status` - 检查小红书登录状态（无参数）
- `publish_content` - 发布图文内容到小红书（需要：title, content, 可选：images, video）
- `list_feeds` - 获取小红书首页推荐列表（无参数）
- `search_feeds` - 搜索小红书内容（需要：keyword）
- `get_feed_detail` - 获取帖子详情（需要：feed_id, xsec_token）
- `post_comment_to_feed` - 发表评论到小红书帖子（需要：feed_id, xsec_token, content）

### 2.4. 使用示例

使用 Claude Code 发布内容到小红书：

```
帮我写一篇帖子发布到小红书上，
配图为：https://cn.bing.com/th?id=OHR.MaoriRock_EN-US6499689741_UHD.jpg&w=3840
图片是："纽西兰陶波湖的Ngātoroirangi矿湾毛利岩雕（© Joppi/Getty Images）"

使用 xiaohongshu-mcp 进行发布。
```

![claude-cli 进行发布](./assets/claude_push.gif)

**发布结果：**

<img src="./assets/publish_result.jpeg" alt="xiaohongshu-mcp 发布结果" width="400">
