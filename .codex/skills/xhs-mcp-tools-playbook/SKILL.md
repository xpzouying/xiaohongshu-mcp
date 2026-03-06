---
name: xhs-mcp-tools-playbook
description: Use when orchestrating xiaohongshu-mcp tool calls for login, search, feed detail, interaction, publish, and feed video transcription with MCP session and retry safety.
---

# XHS MCP Tools Playbook

## Overview

基于当前仓库 `mcp_server.go` 与 `mcp_handlers.go` 的能力，整理 14 个 MCP 工具的稳定调用手册。  
目标是减少“参数缺失、会话握手错误、登录态错位、详情抓取偶发失败”导致的反复重试。

登录策略（强制）：默认使用浏览器登录流程，不使用 `get_login_qrcode` 作为主登录路径。

## Tool Inventory（14 个）

| 工具名 | 必填参数 | 主要用途 | 常见前置依赖 |
|---|---|---|---|
| `check_login_status` | 无 | 检查是否登录 | 无 |
| `get_login_qrcode` | 无 | 获取二维码（仅保留兼容，不作为默认登录方案） | 无 |
| `delete_cookies` | 无 | 删除 cookies，重置登录 | 登录异常时使用 |
| `publish_content` | `title` `content` `images` | 发布图文 | 已登录 |
| `publish_with_video` | `title` `content` `video` | 发布视频（本地文件） | 已登录 |
| `list_feeds` | 无 | 首页推荐列表 | 已登录更稳定 |
| `search_feeds` | `keyword` | 关键词搜索 | 已登录 |
| `get_feed_detail` | `feed_id` `xsec_token` | 获取帖子详情/评论 | `feed_id` 与 `xsec_token` 来自 feed 列表 |
| `transcribe_feed_video` | `feed_id` `xsec_token` | 转写视频帖子，输出 TXT/SRT | 先能稳定 `get_feed_detail` + 配置视频理解 API Key（DashScope/GLM） |
| `user_profile` | `user_id` `xsec_token` | 获取用户主页 | `user_id` 与 `xsec_token` 来自 feed 结果 |
| `post_comment_to_feed` | `feed_id` `xsec_token` `content` | 发表评论 | 先拿到 feed 标识 |
| `reply_comment_in_feed` | `feed_id` `xsec_token` `content` + (`comment_id` 或 `user_id`) | 回复评论 | 先拿评论信息 |
| `like_feed` | `feed_id` `xsec_token` | 点赞/取消点赞 | 先拿到 feed 标识 |
| `favorite_feed` | `feed_id` `xsec_token` | 收藏/取消收藏 | 先拿到 feed 标识 |

## 为什么前面会失败很多次（根因）

1. **MCP 会话未完成握手**：只 `initialize` 不带 `Mcp-Session-Id` 继续调 `tools/list`，会报 *invalid during session initialization*。
2. **登录态错位**：登录和测试请求不在同一实例/同一路径 cookies 下，导致“刚登录又显示未登录”。
3. **详情抓取偶发空窗**：`get_feed_detail` 首次可能返回 `feed ... not found in noteDetailMap`，通常重试 1~3 次可恢复。
4. **转写配置缺失**：`transcribe_feed_video` 未配置 `DASHSCOPE_API_KEY` 或 `ZHIPUAI_API_KEY`/`BIGMODEL_API_KEY` 时会直接失败。
5. **跨阶段参数漂移**：`search` 得到的 `feed_id/xsec_token` 必须尽快使用，且最好在同一会话链路中验证。

## Browser Login Standard（默认登录流程）

### Rule

- 禁止默认调用 `get_login_qrcode`。
- 必须通过浏览器登录程序完成登录态写入，再用 `check_login_status` 验收。

### Local Source Mode

1. 激活包含新 Go 版本的环境（示例：`conda activate nanobot`）。
2. 执行：`go run cmd/login/main.go`（非无头，自动拉起浏览器）。
3. 登录完成后调用 `check_login_status` 验证。

### Docker MCP Mode

1. 先确认 MCP 容器使用的 cookies 路径（通常为 `/app/data/cookies.json`）。
2. 浏览器登录时将 `COOKIES_PATH` 指向该容器映射路径（例如本地 `docker/data/cookies.json`）。
3. 若宿主目录权限受限，先写入临时 cookies，再 `docker cp` 到容器内目标路径并重启容器。
4. 最后调用 `check_login_status` 验证结果。

## MCP Session Standard（必做）

任何 MCP 调用都必须按以下顺序：

1. `initialize`（记录响应头 `Mcp-Session-Id`）
2. `notifications/initialized`
3. 后续所有 `tools/list` / `tools/call` 请求都带 `Mcp-Session-Id`

最小模板：

```bash
INIT_HEADERS=$(mktemp)
curl -sS -D "$INIT_HEADERS" -o /tmp/mcp_init.json \
  -X POST http://localhost:18060/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}'

SESSION_ID=$(awk 'BEGIN{IGNORECASE=1} /^Mcp-Session-Id:/ {print $2}' "$INIT_HEADERS" | tr -d '\r')

curl -sS -X POST http://localhost:18060/mcp \
  -H 'Content-Type: application/json' \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
```

## Key Argument Rules

- `xsec_token` 不是可猜字段，必须从列表/搜索结果中提取。
- `reply_comment_in_feed` 必须至少提供 `comment_id` 或 `user_id` 之一。
- `publish_content.images` 至少 1 张，支持 HTTP 链接或本地绝对路径。
- `publish_with_video.video` 只支持本地单个视频绝对路径。
- `get_feed_detail` 默认只返回前 10 条一级评论；如需更多评论显式设置 `load_all_comments=true`。
- `transcribe_feed_video.provider` 支持 `dashscope`（默认）或 `glm`。
- `transcribe_feed_video.api_key` 可选；传入时优先于环境变量（适合在 Inspector 表单内临时填入）。
- `transcribe_feed_video.model` 可选，默认：
  - `provider=dashscope` -> `qwen3.5-flash`
  - `provider=glm` -> `glm-4.6v-flash`
- `transcribe_feed_video.keep_artifacts` 为兼容参数；当前链路不再保留本地中间视频文件。

## High-Value Optional Parameters

### `search_feeds.filters`

- `sort_by`: `综合` / `最新` / `最多点赞` / `最多评论` / `最多收藏`
- `note_type`: `不限` / `视频` / `图文`
- `publish_time`: `不限` / `一天内` / `一周内` / `半年内`
- `search_scope`: `不限` / `已看过` / `未看过` / `已关注`
- `location`: `不限` / `同城` / `附近`

### `get_feed_detail` 评论拉取控制

- `load_all_comments`: `true` 时滚动加载更多评论
- `limit`: 一级评论上限（默认 20）
- `click_more_replies`: 是否展开二级回复（默认 `false`）
- `reply_limit`: 跳过回复过多评论阈值（默认 10）
- `scroll_speed`: `slow` / `normal` / `fast`

### 发布类附加参数

- `schedule_at`: ISO8601 定时发布时间（1 小时到 14 天窗口）
- `visibility`: `公开可见` / `仅自己可见` / `仅互关好友可见`
- `products`: 带货商品关键词或商品 ID 列表
- `is_original`（仅图文发布）：是否声明原创

### `transcribe_feed_video` 参数

- `language`: 默认自动识别；中文建议显式传 `zh`
- `output_dir`: 推荐传固定目录便于定位产物
- `keep_artifacts`: 默认 `false`，只保留 `txt/srt`

## Minimal Playbooks

### A. 登录并搜索

1. 调 `check_login_status`
2. 若未登录，执行浏览器登录流程（`go run cmd/login/main.go`）
3. 调 `search_feeds`，至少提供 `keyword`

### B. 从搜索结果到互动

1. 从 `search_feeds` 结果取 `feed_id`、`xsec_token`
2. 调 `get_feed_detail`
3. 视需求调用：
   - `like_feed`（`unlike=true` 为取消）
   - `favorite_feed`（`unfavorite=true` 为取消）
   - `post_comment_to_feed`
   - `reply_comment_in_feed`

### C. 发布内容

1. 图文：`publish_content`（确保 `images` 非空）
2. 视频：`publish_with_video`（确保 `video` 为本地绝对路径）
3. 需定时发布时加 `schedule_at`

### D. 真实帖子转写（稳定版）

1. 前置检查（同一机器）：
   - `check_login_status` 为已登录
   - 已配置至少一个 API Key：
     - DashScope：`DASHSCOPE_API_KEY`
     - GLM：`ZHIPUAI_API_KEY`（或 `BIGMODEL_API_KEY`）
2. 在目标实例端口上确认：
   - `/health` 可用
   - MCP 会话握手正常
3. 完成 MCP 会话握手（见 `MCP Session Standard`）。
4. 先拿候选视频帖（`search_feeds` 或 HTTP `/feeds/search`）。
5. 对候选执行 `get_feed_detail` 预检：
   - 若报 `not found in noteDetailMap`，同参数重试最多 3 次。
6. 再执行 `transcribe_feed_video`：
   - 若仍报 `VIDEO_RESOLVE_ERROR`，按同参数重试最多 3 次。
7. 验收产物：
   - `txt_path` 存在且非空
   - `srt_path` 存在且非空
   - `artifacts_cleaned` 与 `keep_artifacts` 一致

## Failure Signals & Fixes

- 错误：`缺少feed_id参数` / `缺少xsec_token参数`  
  处理：先走 `list_feeds` 或 `search_feeds`，再提取标识后调用。

- 错误：`缺少 comment_id 或 user_id`  
  处理：先通过 `get_feed_detail` 获取评论信息再回复。

- 错误：登录相关失败/过期  
  处理：`delete_cookies` → 浏览器登录流程重新登录（必要时同步 cookies 到 MCP 实际读取路径）。

- 错误：发布视频缺少路径  
  处理：确认 `video` 是本地绝对路径且文件存在。

- 错误：`method "tools/list" is invalid during session initialization`  
  处理：漏了 `notifications/initialized` 或 `Mcp-Session-Id`。

- 错误：`feed ... not found in noteDetailMap`  
  处理：这是详情抓取偶发空窗，保持相同参数重试 1~3 次；不要立即换 `feed_id`。

- 错误：`DEPENDENCY_ERROR: 未配置阿里云百炼 API Key`  
  处理：设置 `DASHSCOPE_API_KEY`，或改用 `provider=glm` 并配置 `ZHIPUAI_API_KEY`/`BIGMODEL_API_KEY`。

- 错误：`DASHSCOPE_API_ERROR` / `GLM_API_ERROR`  
  处理：检查对应 provider 的 API Key、模型可用性、配额与网络连通性，再按同参数重试。

## Output Contract（调用时建议）

- 每次调用前先明确：输入参数、期望输出、失败信号。
- 涉及发布/互动的 destructive 工具，先打印待执行参数再执行。
- 需要连续多步时，保留上一步关键字段（`feed_id`、`xsec_token`、`comment_id`、`user_id`）作为上下文证据。
- 转写任务额外保留：`txt_path`、`srt_path`、文件大小、文本前 5~10 行预览。
