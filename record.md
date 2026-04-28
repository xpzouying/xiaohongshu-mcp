# Claude 问答记录

## 2026-04-27

### Q: 准备爆改这个项目做自动化拉取爆款小红书帖子，有什么注意事项？

A: 给出六类注意事项：

1. **账号风控**：抓取比发布更敏感，建议账号池 + 代理池 + 全局频控；当前 `XHS_PROXY` 仅单代理需扩展。
2. **现有能力**：`search_feeds` 已支持按点赞/收藏/评论排序 + 时间筛选，是"爆款"的天然过滤器；`get_feed_detail` 拿互动数；缺分页、xsec_token 刷新、批量调度。
3. **数据层缺失**：项目无存储，需新增 SQLite/MySQL 做 feed_id 去重和互动数趋势；存储应独立成 `storage/` 包。
4. **反爬识别**：go-rod 默认暴露 webdriver 指纹，高频抓取需 stealth 处理；按 CLAUDE.md 规约避免大量 JS 注入。
5. **合规**：小红书 TOS 禁爬，限定个人选题分析用途，不要落库分发原文原图。
6. **项目规范**：走 feature 分支、不过度设计、中文注释、未同意不推远程；建议以"新增 crawler 子命令 + 新 MCP 工具 search_hot_feeds"切入，保留发布功能不动。

后续待用户确认范围：(a) 仅加 MCP 工具层 vs (b) 完整 pipeline（定时 + 落库 + 看板）。

## 2026-04-28

### 今日工作：新增 get_hot_feeds MCP 工具

**目标**：在 MCP 工具层新增爆款帖子搜索能力，方案 (a)。

**实现内容**：

1. `xiaohongshu/hot_feeds.go`（新文件）
   - `ParseInteractCount`：解析"1.2万"/"999+"等字符串为 int
   - `SortFeeds`：按点赞/收藏/评论客户端降序排序
   - `FilterByThreshold`：按最低互动数阈值过滤

2. `service.go`：新增 `HotFeedsOption` + `GetHotFeeds`，调用 SearchFeeds 后客户端排序过滤

3. `mcp_handlers.go`：新增 `handleGetHotFeeds`，使用独立 3 分钟 context 防止 SSE 断开 panic

4. `mcp_server.go`：新增 `GetHotFeedsArgs`，注册工具 #14 `get_hot_feeds`

5. `docker/docker-compose.override.yml`：本地开发用 xiaohongshu-mcp-local 镜像

**踩坑记录**：
- headless 模式下 hover 触发的 filter 面板经常失效，导致 MustWait 等 60 秒超时，多个 filter 叠加超过 SSE 连接 5 分钟上限
- 解法：放弃 UI filter 点击，改为客户端排序（sort.Slice）
- SSE 连接断开会 cancel MCP request context，导致 rod MustXxx panic；解法：browser 操作用独立 context.Background() + 3 分钟超时
- Docker Desktop 出现 502 Bad Gateway 无法启动新容器，需重启 Docker Desktop

**待确认**：get_hot_feeds 在最新版（客户端排序）下是否正常返回结果（cookies 是否有效）。
