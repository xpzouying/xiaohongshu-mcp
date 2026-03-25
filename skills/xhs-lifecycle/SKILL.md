---
name: xhs-lifecycle
description: >
  小红书全生命周期管理技能，基于 MCP tools 实现。覆盖：自动启动 MCP server、登录引导、搜索、浏览推荐、
  查看笔记详情、查看用户主页、发布图文/视频、评论、回复、点赞、收藏。
  当用户提到小红书、xiaohongshu、RedNote，或者想要搜索/阅读/发布/评论/点赞小红书内容时触发。
  也可通过 '发小红书'、'搜小红书'、'看小红书' 触发。
---

# 小红书全生命周期管理

基于 xiaohongshu-mcp 的 13 个 MCP tools，覆盖搜索、阅读、发布、互动的完整操作流程。
自动管理 server 启动和登录状态，用户无需手动配置。

## 1. 确保 MCP Server 运行

每次执行小红书相关操作前，先确认服务状态。

### 1a. 检查状态

```bash
bash <skill-path>/scripts/check-status.sh
```

- `SERVER_UP` → 跳到步骤 2
- `SERVER_DOWN` → 执行 1b

### 1b. 启动服务

```bash
bash <skill-path>/scripts/start-server.sh
```

脚本会自动检测本地 Chrome/Chromium 路径（macOS、Linux、Windows 均支持），
通过 `ROD_BROWSER_BIN` 环境变量传递给 server，避免 rod 从 Google 下载 Chromium
（在部分网络环境下极慢或失败）。

如果启动失败，检查日志（repo 根目录下 `.server.log`）并报告给用户。

如果 binary 不存在，提示用户先构建：
```bash
cd <repo-root> && go build -o xiaohongshu-mcp .
```

## 2. 检查登录状态

服务运行后，调用 MCP tool `check_login_status` 确认小红书账号已登录。

- **已登录** → 跳到步骤 3
- **未登录** → 告诉用户需要扫码登录，然后运行登录工具：
  ```bash
  cd <repo-root> && go run cmd/login/main.go
  ```
  如果已构建 login binary：
  ```bash
  cd <repo-root> && ./xiaohongshu-login
  ```
  登录工具会弹出浏览器窗口显示二维码，用户用小红书 App 扫码。
  登录成功后 cookies 持久化，后续重启服务无需重新登录。

## 3. 执行用户任务

登录确认后，根据用户意图调用对应的 MCP tool。

### 常见场景映射

| 用户意图 | MCP Tool | 注意事项 |
|---------|----------|---------|
| 搜索内容 | `search_feeds` | 传入关键词 |
| 浏览推荐 | `list_feeds` | 返回首页 feed |
| 查看帖子详情 | `get_feed_detail` | 需要 feed_id + xsec_token |
| 查看用户主页 | `user_profile` | 需要 user_id |
| 发布图文 | `publish_content` | 标题 ≤20 字，正文 ≤1000 字 |
| 发布视频 | `publish_with_video` | 传入本地视频文件路径 |
| 评论 | `post_comment_to_feed` | 需要 feed_id + xsec_token |
| 回复评论 | `reply_comment_in_feed` | 需要 comment_id |
| 点赞/取消 | `like_feed` | 智能切换状态 |
| 收藏/取消 | `favorite_feed` | 智能切换状态 |

### 发布内容的额外要求

发布前**必须**向用户确认以下内容，因为发布操作不可撤回：
- 标题和正文的最终版本
- 要附带的图片（路径列表）
- 如果用户没提供图片，提醒他们小红书帖子通常需要图片

### 已知限制

- 每个账号每天约 50 条发帖上限
- 小红书只允许单设备 web session：如果用户在别处登录了网页版，MCP 的 cookies 会失效，需要重新扫码
- xsec_token 从 `get_feed_detail` 的返回结果中获取，评论时需要它
