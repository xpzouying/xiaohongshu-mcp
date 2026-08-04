# xiaohongshu-mcp

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->

[![All Contributors](https://img.shields.io/badge/all_contributors-27-orange.svg?style=flat-square)](#contributors-)

<!-- ALL-CONTRIBUTORS-BADGE:END -->

[![Philanthropy](https://img.shields.io/badge/Philanthropy-CNY%201810.00-brightgreen?style=flat-square)](./DONATIONS.md)
[![Gratitude](https://img.shields.io/badge/Gratitude-CNY%201524.64-blue?style=flat-square)](./DONATIONS.md)

MCP for RedNote (Xiaohongshu) / xiaohongshu.com. Give your AI assistant direct access to RedNote data.

> This workspace is the read-only trimmed edition: it keeps login initialization, Feed reads, and user-profile reads (7 MCP tools). Publishing, comment/reply writes, like/favorite operations, notifications, and cookie deletion have been removed. See [docs/API.md](./docs/API.md) for the current API.

### 🚀 Quick Start: Pick the Version That Fits You

> [!IMPORTANT]
> #### 🔥 Option A: Deep Openclaw Integration (recommended for developers)
> - **Openclaw is on fire 🔥🔥🔥 — Openclaw support has been added in two flavors, pick whichever suits you:**
> - [xiaohongshu-mcp-skills](https://github.com/autoclaw-cc/xiaohongshu-mcp-skills) (for users who already have this project deployed)
> - [xiaohongshu-skills](https://github.com/autoclaw-cc/xiaohongshu-skills) (ready to use out of the box)

> [!TIP]
> #### ✨ Option B: x-mcp Browser Extension (recommended for non-technical users / anyone who wants the simplest setup)
> - **Don't want to deal with local deployment? Try [xpzouying/x-mcp](https://github.com/xpzouying/x-mcp).**
> - **Zero configuration**: install the extension and it just works — no code, no proxy, no complicated environment setup.
> - **Safe and stable**: runs directly in your everyday browser (Chrome/Edge) on your local network, with no server IP risk, and it solves 90% of deployment errors.

### 📖 Related Resources

- **My blog article**: [haha.ai/xiaohongshu-mcp](https://www.haha.ai/xiaohongshu-mcp)
- **Contributing Guide**: [Contributing Guide](./CONTRIBUTING.md)

## Star History

<!-- Chart generated weekly onto the star-history data branch by .github/workflows/star-history.yml (star-history.com hosted chart broke due to GitHub API restrictions) -->
<a href="https://www.star-history.com/#xpzouying/xiaohongshu-mcp&Timeline">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/xpzouying/xiaohongshu-mcp/star-history/assets/star-history-dark.svg" />
    <img alt="Star History Chart" src="https://raw.githubusercontent.com/xpzouying/xiaohongshu-mcp/star-history/assets/star-history.svg" />
  </picture>
</a>

## Appreciation and Support

All donations received for this project will be used for charitable giving. For all charitable donation records, please refer to [DONATIONS.md](./DONATIONS.md).

**When donating, please note "MCP" and your name.**
If you need to correct/withdraw your name attribution, please open an Issue or contact via email.

**Alipay (QR code not displayed):**

Donate via Alipay to **xpzouying@gmail.com**.

**WeChat:**

<img src="donate/wechat@2x.png" alt="WeChat Pay QR" width="260" />

## Project Overview

**Main Features**

> 💡 **Tip:** Click on the feature titles below to expand and view video demonstrations

<details>
<summary><b>1. Login and Check Login Status</b></summary>

The first step is required - RedNote needs to be logged in. You can check current login status.

**Login Demo:**

https://github.com/user-attachments/assets/8b05eb42-d437-41b7-9235-e2143f19e8b7

**Check Login Status Demo:**

https://github.com/user-attachments/assets/bd9a9a4a-58cb-4421-b8f3-015f703ce1f9

</details>

<details>
<summary><b>2. Get Login QR Code</b></summary>

When not logged in, obtain a QR code and scan it with the RedNote app. After a successful scan, the service stores the login Cookie for subsequent reads. This is session initialization only; no content is published or modified.

</details>

<details>
<summary><b>3. Search Content</b></summary>

Search RedNote content by keywords.

**Search Posts Demo:**

https://github.com/user-attachments/assets/03c5077d-6160-4b18-b629-2e40933a1fd3

</details>

<details>
<summary><b>4. Get Recommendation List</b></summary>

Get RedNote homepage recommendation content list.

**Get Recommendation List Demo:**

https://github.com/user-attachments/assets/110fc15d-46f2-4cca-bdad-9de5b5b8cc28

</details>

<details>
<summary><b>5. Get Post Details (Including Interaction Data and Comments)</b></summary>

Get complete details of RedNote posts, including:

- Post content (title, description, images, etc.)
- User information
- Interaction data (likes, favorites, shares, comment count)
- Comment list and sub-comments

**⚠️ Important Note:**

- Both post ID and xsec_token are required (both parameters are essential)
- These two parameters can be obtained from Feed list or search results
- Must login first to use this feature

**Get Post Details Demo:**

https://github.com/user-attachments/assets/76a26130-a216-4371-a6b3-937b8fda092a

</details>

<details>
<summary><b>6. Get User Profile</b></summary>

Get RedNote user's personal profile information, including basic user information and note content.

**Feature Description:**

- Get user basic information (nickname, bio, avatar, etc.)
- Get follower count, following count, likes count statistics
- Get user's published note content list
- Supports HTTP API and MCP tool calls

**⚠️ Important Note:**

- Must login first to use this feature
- Need to provide user ID and xsec_token
- These parameters can be obtained from Feed list or search results

**Returned Information Includes:**

- User basic info: nickname, bio, avatar, verification status
- Statistics: following count, follower count, likes count, note count
- Note list: all public notes published by the user

</details>

<details>
<summary><b>7. Current Capability Boundary</b></summary>

This trimmed edition only reads the homepage Feed, search results, note details, comments, user profiles, and the current user's profile. Publishing, comment/reply writes, like/favorite operations, notification reads, and cookie deletion are not provided.

</details>

**Usage Boundary**

- This edition is read-only and does not provide publishing, comment/reply, like, or favorite writes.
- **(Very Important) RedNote does not allow the same account to login on multiple web platforms**. If you login to the current xiaohongshu-mcp, don't login to that account on other web platforms, otherwise it will "kick out" the current MCP account login. You can use the mobile app to check current account information.

**Risk Explanation**

1. This project is open-sourced based on another project of mine. The original project has been running stably for over a year without any account bans, only occasional cookie expiration requiring re-login.
2. I used Claude Code CLI integration and verified stable read access for several weeks before open-sourcing.
3. If an account has not completed real-name verification, especially a new account, it will usually trigger a **real-name verification** prompt (see the screenshot below). ⚠️ This is not an account ban — you would be asked to verify even without using the MCP. Once verified, the account works normally. It is recommended to complete verification before using this project.
   <img width="508" height="306" alt="image" src="https://github.com/user-attachments/assets/34383e1b-f666-409f-9870-002655507dc1" />

This project is for learning purposes only. All illegal activities are prohibited.

## 1. Usage Tutorial

### 1.1. Quick Start (Recommended)

**Method 1: Download Pre-compiled Binaries**

Download pre-compiled binaries for your platform directly from [GitHub Releases](https://github.com/xpzouying/xiaohongshu-mcp/releases):

**Main Program (MCP Service):**

- **macOS Apple Silicon**: `xiaohongshu-mcp-darwin-arm64`
- **Windows x64**: `xiaohongshu-mcp-windows-amd64.exe`
- **Linux x64**: `xiaohongshu-mcp-linux-amd64`

**Login Tool:**

- **macOS Apple Silicon**: `xiaohongshu-login-darwin-arm64`
- **Windows x64**: `xiaohongshu-login-windows-amd64.exe`
- **Linux x64**: `xiaohongshu-login-linux-amd64`

> Only the three platforms above are supported. macOS Intel and Linux ARM64 are not supported.

Usage Steps:

```bash
# 1. First run the login tool
chmod +x xiaohongshu-login-darwin-arm64
./xiaohongshu-login-darwin-arm64

# 2. Then start the MCP service
chmod +x xiaohongshu-mcp-darwin-arm64
./xiaohongshu-mcp-darwin-arm64
```

**⚠️ Important Note**: This project does not automatically download a browser. Install Google Chrome/Chromium locally before running, or set `XHS_BROWSER_BIN` to the browser executable.

**Method 2: Build from Source**

<details>
<summary>Build from Source Details</summary>

Requires Golang environment. For installation instructions, please refer to [Golang Official Documentation](https://go.dev/doc/install).

Set Go domestic proxy source:

```bash
# Configure GOPROXY environment variable, choose one of the following three

# 1. Qiniu Go module mirror
go env -w  GOPROXY=https://goproxy.cn,direct

# 2. Alibaba Cloud
go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

# 3. Official
go env -w  GOPROXY=https://goproxy.io,direct
```

</details>

For Windows issues, check here first: [Windows Installation Guide](./docs/windows_guide.md)

### 1.2. Login

First time requires manual login to save RedNote login status.

**Using Binary Files:**

```bash
# Run the login tool for your platform
./xiaohongshu-login-darwin-arm64
```

**Using Source Code:**

```bash
go run cmd/login/main.go
```

### 1.3. Start MCP Service

Start xiaohongshu-mcp service.

**Using Binary Files:**

```bash
# Default: Headless mode, no browser interface
./xiaohongshu-mcp-darwin-arm64

# Non-headless mode, with browser interface
./xiaohongshu-mcp-darwin-arm64 -headless=false
```

**Using Source Code:**

```bash
# Default: Headless mode, no browser interface
go run .

# Non-headless mode, with browser interface
go run . -headless=false
```

**Configure a proxy (optional)**:

If you need to go through a proxy, set the `XHS_PROXY` environment variable:

```bash
# Start with a proxy configured
XHS_PROXY=http://user:pass@proxy:port ./xiaohongshu-mcp-darwin-arm64

# Or from source
XHS_PROXY=http://proxy:port go run .
```

HTTP/HTTPS/SOCKS5 proxies are supported, and proxy credentials are automatically masked in the logs.

## 1.4. Verify MCP

```bash
npx @modelcontextprotocol/inspector
```

![Run Inspector](./assets/run_inspect.png)

After running, open the red-marked link, configure MCP inspector, enter `http://localhost:18060/mcp`, and click the `Connect` button.

<img width="915" height="659" alt="bf9532dd0b7ba423491accf511a467de" src="https://github.com/user-attachments/assets/08bc3cef-73e7-42d2-b923-7ba9e6c8af30" />

**Note:** Check if the options in the left sidebar are correct.

After configuring MCP inspector as above, click the `List Tools` button to view all Tools.

## 1.5. Use MCP for Reading

### Check Login Status

![Check Login Status](./assets/check_login.gif)

### Search Content

Use search functionality to search RedNote content by keywords:

![Search Content](./assets/search_result.png)

## 2. MCP Client Integration

This service supports the standard Model Context Protocol (MCP) and can integrate with various AI clients that support MCP.

### 2.1. Quick Start

#### Start MCP Service

```bash
# Start service (default headless mode)
go run .

# Or with interface mode
go run . -headless=false
```

Service will run at: `http://localhost:18060/mcp`

#### Verify Service Status

```bash
# Test MCP connection
curl -X POST http://localhost:18060/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}'
```

#### Claude Code CLI Integration

```bash
# Add HTTP MCP server
claude mcp add --transport http xiaohongshu-mcp http://localhost:18060/mcp

# Check if MCP was added successfully (ensure MCP is already started before running this command)
claude mcp list
```

### 2.2. Supported Clients

<details>
<summary><b>Claude Code CLI</b></summary>

Official command line tool, already shown in the quick start section above:

```bash
# Add HTTP MCP server
claude mcp add --transport http xiaohongshu-mcp http://localhost:18060/mcp

# Check if MCP was added successfully (ensure MCP is already started before running this command)
claude mcp list
```

</details>

<details>
<summary><b>Open Code CLI</b></summary>

Add the MCP server with the interactive command:

```bash
opencode mcp add
```

Using `xiaohongshu-mcp` as an example:

```
┌  Add MCP server
│
◇  Enter MCP server name
│  xiaohongshu-mcp
│
◇  Select MCP server type
│  Remote
│
◇  Enter MCP server URL
│  http://localhost:18060/mcp
│
◇  Does this server require OAuth authentication?
│  No
│
◆  MCP server "xiaohongshu-mcp" added to C:\Users\admin\.config\opencode\opencode.json
│
└  MCP server added successfully
```

Verify that it was added successfully (make sure the MCP service is running):

```bash
opencode mcp list
```

```
┌  MCP Servers
│
●  ✓ xiaohongshu-mcp connected
```

</details>

<details>
<summary><b>Cursor</b></summary>

#### Configuration File Method

Create or edit MCP configuration file:

**Project-level configuration** (recommended):
Create `.cursor/mcp.json` in project root directory:

```json
{
  "mcpServers": {
    "xiaohongshu-mcp": {
      "url": "http://localhost:18060/mcp",
      "description": "RedNote read-only data service - MCP Streamable HTTP"
    }
  }
}
```

**Global configuration**:
Create `~/.cursor/mcp.json` in user directory (same content).

#### Usage Steps

1. Ensure RedNote MCP service is running
2. Save configuration file and restart Cursor
3. In Cursor chat, tools should be automatically available
4. You can view connected MCP tools through "Available Tools" in the chat interface

**Demo**

Plugin MCP integration:

![cursor_mcp_settings](./assets/cursor_mcp_settings.png)

Call MCP tools: (using check login status as example)

![cursor_mcp_check_login](./assets/cursor_mcp_check_login.png)

</details>

<details>
<summary><b>VSCode</b></summary>

#### Method 1: Configure using Command Palette

1. Press `Ctrl/Cmd + Shift + P` to open command palette
2. Run `MCP: Add Server` command
3. Select `HTTP` method.
4. Enter address: `http://localhost:18060/mcp`, or modify to corresponding Server address.
5. Enter MCP name: `xiaohongshu-mcp`.

#### Method 2: Direct Configuration File Edit

**Workspace configuration** (recommended):
Create `.vscode/mcp.json` in project root directory:

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

**View Configuration**:

![vscode_config](./assets/vscode_mcp_config.png)

1. Confirm running status.
2. Check if `tools` are correctly detected.

**Demo**

Using search post content as example:

![vscode_mcp_search](./assets/vscode_search_demo.png)

</details>

<details>
<summary><b>Google Gemini CLI</b></summary>

Configure in `~/.gemini/settings.json` or project directory `.gemini/settings.json`:

```json
{
  "mcpServers": {
    "xiaohongshu": {
      "httpUrl": "http://localhost:18060/mcp",
      "timeout": 30000
    }
  }
}
```

For more information, please refer to [Gemini CLI MCP Documentation](https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html)

</details>

<details>
<summary><b>MCP Inspector</b></summary>

Debug tool for testing MCP connections:

```bash
# Start MCP Inspector
npx @modelcontextprotocol/inspector

# Connect in browser to: http://localhost:18060/mcp
```

Usage steps:

- Use MCP Inspector to test connection
- Test Ping Server functionality to verify connection
- Check if List Tools returns 13 tools

</details>

<details>
<summary><b>Cline</b></summary>

Cline is a powerful AI programming assistant that supports MCP protocol integration.

#### Configuration Method

Add the following configuration to Cline's MCP settings:

```json
{
  "xiaohongshu-mcp": {
    "url": "http://localhost:18060/mcp",
    "type": "streamableHttp",
    "autoApprove": [],
    "disabled": false
  }
}
```

#### Usage Steps

1. Ensure RedNote MCP service is running (`http://localhost:18060/mcp`)
2. Open MCP settings in Cline
3. Add the above configuration to the MCP server list
4. Save configuration and restart Cline
5. You can directly use RedNote-related features in conversations

#### Configuration Explanation

- `url`: MCP service address
- `type`: Use `streamableHttp` type for better performance
- `autoApprove`: Configurable auto-approve tool list (empty means manual approval)
- `disabled`: Set to `false` to enable this MCP service

#### Usage Examples

After configuration, you can use natural language to operate RedNote directly in Cline:

```
Help me check RedNote login status
```

```
Help me search RedNote for spring-related content and read one note's details
```

```
Search for content about "food" on RedNote
```

</details>
<details>
<summary><b>OpenClaw (via MCPorter)</b></summary>

> Make sure xiaohongshu-mcp is already deployed locally before you start. Handing the GitHub link to OpenClaw and letting it deploy the project for you is **not recommended**.

Since OpenClaw does not natively support MCP yet, the officially recommended way to call MCP services is through **MCPorter**.

> 💡 **Tip:** MCPorter is not the ideal way to call MCP — you may run into compatibility issues along the way, so please be aware.

#### Installation and Setup

Just hand the following three commands to OpenClaw in one go (via the Control UI, Telegram, Feishu, etc.), and OpenClaw will set up MCPorter for you.

```
npm i -g mcporter
npx mcporter config add xiaohongshu-mcp http://localhost:18060/mcp
npx mcporter list xiaohongshu-mcp
```

Once that is done, you can use every xiaohongshu-mcp feature from OpenClaw through natural language.

</details>
<details>
<summary><b>Other HTTP MCP Supporting Clients</b></summary>

Any client supporting HTTP MCP protocol can connect to: `http://localhost:18060/mcp`

Basic configuration template:

```json
{
  "name": "xiaohongshu-mcp",
  "url": "http://localhost:18060/mcp",
  "type": "http"
}
```

</details>

### 2.3. Available MCP Tools

After successful connection, you can use the following MCP tools:

- `check_login_status` - Check RedNote login status (no parameters)
- `get_login_qrcode` - Get login QR code, returns Base64 image and timeout (no parameters)
- `list_feeds` - Get RedNote homepage recommendation list (no parameters)
- `search_feeds` - Search RedNote content (required: keyword)
  - `filters`: Filter options (optional). Values must be passed exactly as the Chinese strings below — they match the labels on the RedNote filter panel.
    - `sort_by`: Sort by - `综合` / comprehensive (default) | `最新` / latest | `最多点赞` / most liked | `最多评论` / most comments | `最多收藏` / most saved
    - `note_type`: Note type - `不限` / any (default) | `视频` / video | `图文` / image-text
    - `publish_time`: Publish time - `不限` / any (default) | `一天内` / last day | `一周内` / last week | `半年内` / last 6 months
    - `search_scope`: Search scope - `不限` / any (default) | `已看过` / viewed | `未看过` / not viewed | `已关注` / followed
    - `location`: Location - `不限` / any (default) | `同城` / same city | `附近` / nearby
- `get_feed_detail` - Get post details including interaction data and comments (required: feed_id, xsec_token)
  - `load_all_comments`: Whether to load all comments (optional), default false returns only first 10 top-level comments
  - `limit`: Limit number of top-level comments to load (optional), only effective when load_all_comments=true, default 20
  - `click_more_replies`: Whether to expand nested replies (optional), only effective when load_all_comments=true, default false
  - `reply_limit`: Skip comments with too many replies (optional), only effective when click_more_replies=true, default 10
  - `scroll_speed`: Scroll speed (optional), `slow` | `normal` | `fast`, only effective when load_all_comments=true
- `user_profile` - Get user profile information (required: user_id, xsec_token)
- `get_my_profile` - Get the current user's profile (optional: tab)

### 2.4. Usage Examples

Using Claude Code to read RedNote content:

```
Use xiaohongshu-mcp to search for “travel”, return the matching notes,
then read one note's full text and comments.
```

The trimmed MCP only initializes the login session and reads data; it does not publish or modify RedNote content.

### 2.5. 💬 MCP FAQ

---

> ⚠️ The following are known risks when using OpenClaw + MCPorter. Please read them carefully before you start:

- OpenClaw's automated AI deployment behavior is outside the scope of this project's maintenance, and its results cannot be guaranteed
- As an intermediate layer, MCPorter may introduce additional compatibility issues that have nothing to do with xiaohongshu-mcp itself
- If you hit connection failures or abnormal tool calls, please check MCPorter's own configuration first instead of filing an Issue
- Before asking in the community or the groups, please confirm whether the problem also reproduces **without OpenClaw**

If you do not specifically need OpenClaw, we strongly recommend switching to a client with native HTTP MCP support such as [Claude Code CLI](#claude-code-cli), [Cursor](#cursor) or [Cline](#cline) — the experience is much more stable.

---

**Q:** Why does the check login username display `xiaghgngshu-mcp`?
**A:** The username is hardcoded.

---

**Q:** Why does reading a Feed fail?
**A:** Confirm that the session is logged in, and that `feed_id` and `xsec_token` came from a current Feed list or search result. Video links and some page data may also expire when the platform's signature expires.

---

**Q:** The MCP program crashes on my device, how to resolve?
**A:**

1. It is recommended to **build from source**.
2. If local deployment is inconvenient, use the [X-MCP project](https://github.com/xpzouying/x-mcp/).

---

**Q:** When verifying MCP with `http://localhost:18060/mcp`, it shows connection error?
**A:**

- From a local client, connect to `http://127.0.0.1:18060/mcp`.

---

## 3. 🌟 Community Showcases

> 💡 **Highly Recommended**: These are real-world use cases from community contributors, featuring detailed configuration steps and practical experiences!

### 📚 Complete Tutorial List

1. **[Cherry Studio Complete Configuration Tutorial](./examples/cherrystudio/README.md)** - Perfect AI client integration
2. **[Claude Code + Kimi K2 Integration Tutorial](./examples/claude-code/claude-code-kimi-k2.md)** - If Claude Code's barrier is too high, then integrate with Kimi domestic LLM!
3. **[AnythingLLM Complete Guide](./examples/anythingLLM/readme.md)** - AnythingLLM is an all-in-one multimodal AI client supporting multiple LLMs and plugin extensions.

> 🎯 **Tip**: Click the links above to view detailed step-by-step tutorials for quick setup of various integration solutions!
>
> 📢 **Contributions Welcome**: If you have new integration cases, feel free to submit a PR to share with the community!

## 4. RedNote MCP Community Group

**Important: Before asking questions in the group, please make sure to read the README documentation thoroughly and check Issues first.**

### WeChat Group

> These are Chinese-language community groups — discussion in the groups is in Chinese.

|                                                 WeChat Group 24                                     |                                                 WeChat Group 25                                      |
| :------------------------------------------------------------------------------------------------: | :------------------------------------------------------------------------------------------------: |
| <img src="https://github.com/user-attachments/assets/918a48d5-1d3c-40ce-b225-4af0d77078db" alt="WechatIMG119" width="300"> | <img src="https://github.com/user-attachments/assets/c49ad483-0f27-46f3-a6a7-31b3ba31540f" alt="WechatIMG119" width="300">|

### Feishu (Lark) Groups

|                                                         Feishu Group 2                                                    |                                                         Feishu Group 3                                                    |                                                         Feishu Group 4                                                    |                                                         Feishu Group 5                                                    |
| :-----------------------------------------------------------------------------------------------------------------------: | :-----------------------------------------------------------------------------------------------------------------------: | :-----------------------------------------------------------------------------------------------------------------------: | :-----------------------------------------------------------------------------------------------------------------------: |
| <img src="https://github.com/user-attachments/assets/4983ea42-ce5b-4e26-a8c0-33889093b579" alt="qr-feishu02" width="260"> | <img src="https://github.com/user-attachments/assets/c77b45da-6028-4d3a-b421-ccc6c7210695" alt="qr-feishu03" width="260"> | <img src="https://github.com/user-attachments/assets/c42f5595-71cd-4d9b-b7f8-0c333bd25e2b" alt="qr-feishu04" width="260"> | <img src="https://github.com/user-attachments/assets/c032801c-bf02-4e8e-81ad-fb8471b3d765" alt="qr-feishu05" width="260"> |

> **Note:**
>
> 1. WeChat group QR codes have a time limit. Sometimes I forget to update them — please wait for an update or submit an Issue to remind me.
> 2. If a Feishu group is full, try scanning another group's QR code — there's always a spot somewhere.

## 🙏 Thanks to Contributors ✨

Thanks to all friends who have contributed to this project! (In no particular order)

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://haha.ai"><img src="https://avatars.githubusercontent.com/u/3946563?v=4?s=100" width="100px;" alt="zy"/><br /><sub><b>zy</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=xpzouying" title="Code">💻</a> <a href="#ideas-xpzouying" title="Ideas, Planning, & Feedback">🤔</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=xpzouying" title="Documentation">📖</a> <a href="#design-xpzouying" title="Design">🎨</a> <a href="#maintenance-xpzouying" title="Maintenance">🚧</a> <a href="#infra-xpzouying" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/pulls?q=is%3Apr+reviewed-by%3Axpzouying" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://www.hwbuluo.com"><img src="https://avatars.githubusercontent.com/u/1271815?v=4?s=100" width="100px;" alt="clearwater"/><br /><sub><b>clearwater</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=esperyong" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/laryzhong"><img src="https://avatars.githubusercontent.com/u/47939471?v=4?s=100" width="100px;" alt="Zhongpeng"/><br /><sub><b>Zhongpeng</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=laryzhong" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/DTDucas"><img src="https://avatars.githubusercontent.com/u/105262836?v=4?s=100" width="100px;" alt="Duong Tran"/><br /><sub><b>Duong Tran</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=DTDucas" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Angiin"><img src="https://avatars.githubusercontent.com/u/17389304?v=4?s=100" width="100px;" alt="Angiin"/><br /><sub><b>Angiin</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=Angiin" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/muhenan"><img src="https://avatars.githubusercontent.com/u/43441941?v=4?s=100" width="100px;" alt="Henan Mu"/><br /><sub><b>Henan Mu</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=muhenan" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/chengazhen"><img src="https://avatars.githubusercontent.com/u/52627267?v=4?s=100" width="100px;" alt="Journey"/><br /><sub><b>Journey</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=chengazhen" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/eveyuyi"><img src="https://avatars.githubusercontent.com/u/69026872?v=4?s=100" width="100px;" alt="Eve Yu"/><br /><sub><b>Eve Yu</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=eveyuyi" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/CooperGuo"><img src="https://avatars.githubusercontent.com/u/183056602?v=4?s=100" width="100px;" alt="CooperGuo"/><br /><sub><b>CooperGuo</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=CooperGuo" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://biboyqg.github.io/"><img src="https://avatars.githubusercontent.com/u/125724218?v=4?s=100" width="100px;" alt="Banghao Chi"/><br /><sub><b>Banghao Chi</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=BiboyQG" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/varz1"><img src="https://avatars.githubusercontent.com/u/60377372?v=4?s=100" width="100px;" alt="varz1"/><br /><sub><b>varz1</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=varz1" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://google.meloguan.site"><img src="https://avatars.githubusercontent.com/u/62586556?v=4?s=100" width="100px;" alt="Melo Y Guan"/><br /><sub><b>Melo Y Guan</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=Meloyg" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/lmxdawn"><img src="https://avatars.githubusercontent.com/u/21293193?v=4?s=100" width="100px;" alt="lmxdawn"/><br /><sub><b>lmxdawn</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=lmxdawn" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/haikow"><img src="https://avatars.githubusercontent.com/u/22428382?v=4?s=100" width="100px;" alt="haikow"/><br /><sub><b>haikow</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=haikow" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://carlo-blog.aiju.fun/"><img src="https://avatars.githubusercontent.com/u/18513362?v=4?s=100" width="100px;" alt="Carlo"/><br /><sub><b>Carlo</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=a67793581" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/hrz394943230"><img src="https://avatars.githubusercontent.com/u/28583005?v=4?s=100" width="100px;" alt="hrz"/><br /><sub><b>hrz</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=hrz394943230" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ctrlz526"><img src="https://avatars.githubusercontent.com/u/143257420?v=4?s=100" width="100px;" alt="Ctrlz"/><br /><sub><b>Ctrlz</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=ctrlz526" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/flippancy"><img src="https://avatars.githubusercontent.com/u/6467703?v=4?s=100" width="100px;" alt="flippancy"/><br /><sub><b>flippancy</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=flippancy" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Infinityay"><img src="https://avatars.githubusercontent.com/u/103165980?v=4?s=100" width="100px;" alt="Yuhang Lu"/><br /><sub><b>Yuhang Lu</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=Infinityay" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://triepod.ai"><img src="https://avatars.githubusercontent.com/u/199543909?v=4?s=100" width="100px;" alt="Bryan Thompson"/><br /><sub><b>Bryan Thompson</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=triepod-ai" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="http://www.megvii.com"><img src="https://avatars.githubusercontent.com/u/7806992?v=4?s=100" width="100px;" alt="tan jun"/><br /><sub><b>tan jun</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=tanxxjun321" title="Code">💻</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/coldmountein"><img src="https://avatars.githubusercontent.com/u/95873096?v=4?s=100" width="100px;" alt="coldmountain"/><br /><sub><b>coldmountain</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=coldmountein" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://blog.litpp.com/"><img src="https://avatars.githubusercontent.com/u/44826388?v=4?s=100" width="100px;" alt="mamage"/><br /><sub><b>mamage</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=yqdaddy" title="Code">💻</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=yqdaddy" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://runyang.vercel.app/"><img src="https://avatars.githubusercontent.com/u/54588936?v=4?s=100" width="100px;" alt="Runyang YOU"/><br /><sub><b>Runyang YOU</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=YRYangang" title="Code">💻</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=YRYangang" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://www.hnfnu.edu.cn/"><img src="https://avatars.githubusercontent.com/u/134906805?v=4?s=100" width="100px;" alt="e0_7"/><br /><sub><b>e0_7</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=Daily-AC" title="Code">💻</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=Daily-AC" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/prehisle"><img src="https://avatars.githubusercontent.com/u/2081344?v=4?s=100" width="100px;" alt="prehisle"/><br /><sub><b>prehisle</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=prehisle" title="Code">💻</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=prehisle" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/blablabiu"><img src="https://avatars.githubusercontent.com/u/123888078?v=4?s=100" width="100px;" alt="Xinhao Chen"/><br /><sub><b>Xinhao Chen</b></sub></a><br /><a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=blablabiu" title="Code">💻</a> <a href="https://github.com/xpzouying/xiaohongshu-mcp/commits?author=blablabiu" title="Documentation">📖</a></td>
    </tr>
  </tbody>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

### ✨ Special Thanks

<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="20%"><a href="https://github.com/wanpengxie"><img src="https://avatars.githubusercontent.com/wanpengxie" width="130px;" alt="wanpengxie"/><br /><sub><b>@wanpengxie</b></sub></a></td>
      <td align="center" valign="top" width="20%"><a href="https://github.com/tanxxjun321"><img src="https://avatars.githubusercontent.com/u/7806992?v=4" width="130px;" alt="tanxxjun321"/><br /><sub><b>@tanxxjun321</b></sub></a></td>
      <td align="center" valign="top" width="20%"><a href="https://github.com/Angiin"><img src="https://avatars.githubusercontent.com/u/17389304?v=4" width="130px;" alt="Angiin"/><br /><sub><b>@Angiin</b></sub></a></td>
    </tr>
  </tbody>
</table>

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!

## 📄 License

This project is open source under the [Apache License 2.0](LICENSE).

You are free to use, modify and distribute this project, including for commercial purposes, as long as you keep the original copyright notice and license file. The [LICENSE](LICENSE) file is the authoritative source for the full terms.

Contributions submitted to this project are licensed under the same license by default.
