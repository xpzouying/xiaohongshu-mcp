# xiaohongshu-mcp Issues 分析报告

日期：2026-03-17
仓库：https://github.com/xpzouying/xiaohongshu-mcp

## BUG 类

### 已验证确认的 BUG

| Issue | 标题 | 严重性 | 验证结果 |
|-------|------|--------|---------|
| #470 | Chrome 进程泄漏 | **高** | 3 次 API 调用后产生 11 个 Chrome 进程，30 秒后未回收 |
| #569 | 搜索返回 displayTitle 为空 | **中** | 22 条搜索结果全部 displayTitle 和 user 信息为空 |
| #578 | 评论加载不完整 | **中** | load_all_comments 请求直接超时，无法完成全量加载 |
| #533/#504 | reply_comment_in_feed 评论定位失败 | **高** | 代码审查确认：选择器 `#comment-{id}` 和 `.parent-comment` 可能不匹配当前小红书页面结构 |
| #501 | 发布时重复发送 | **高** | 代码审查确认：发布按钮点击后无防重复机制，缺少状态检查 |
| #511 | 选择原创时发布失败 | **高** | 代码审查确认：JS `checkbox.click()` 不触发 React/Vue change 事件，按钮保持 disabled |

### 已修复的 BUG

| Issue | 标题 | 修复方式 |
|-------|------|---------|
| #547 | check_login_status 返回硬编码用户名 | 从个人主页 `__INITIAL_STATE__` 获取真实昵称（commit 6c174b6） |
| #480 | Docker 本地图片路径无法访问 | 支持 base64 data URL + REST API fallback |

### 未复现的 BUG

| Issue | 标题 | 说明 |
|-------|------|------|
| #491 | 搜索 TypeError: circular structure to JSON | 搜索正常返回 22 条结果，未复现 |

### 需要特定环境验证的 BUG

| Issue | 标题 | 所需环境 |
|-------|------|---------|
| #584 | Docker 容器中 Chrome 崩溃，视频发布失败 | macOS + Docker Desktop |
| #583 | WSL2/Linux 发布 60s 超时 | WSL2 环境 |
| #516 | publish 返回成功但实际没发布 | 需多次发布观察 |
| #573 | 发布内容只有自己能看（被风控） | 账号相关 |
| #577 | 容器内 Chrome 没开沙箱 | 安全审计 |
| #555/#554 | Session 状态机问题，无 sessionId | MCP 协议兼容性 |
| #576 | Docker MCP-Inspector 连接失败 | 社区已有解决方案：Connection type 选 via proxy |

---

## 代码审查详情

### #470 Chrome 进程泄漏

- **根因**：每次 MCP/API 调用都通过 `newBrowser()` 创建新的 Chrome 实例，`defer b.Close()` 在超时或 panic 时可能不执行
- **影响**：几小时内累积数十个僵尸进程，耗尽容器内存
- **建议**：考虑在请求间复用单个浏览器实例，或增加进程清理机制

### #533/#504 评论回复 DOM 选择器不匹配

- **位置**：`xiaohongshu/comment_feed.go:232`
- **根因**：
  - 使用 `#comment-{commentID}` 选择器，但小红书页面可能不提供此 ID
  - 后备选择器 `.comment-item, .comment, .parent-comment` 可能返回混合元素
  - `[data-user-id="{userID}"]` 属性名可能与实际页面不匹配
- **建议**：增加多种备用查找策略（文本匹配、时间戳、用户昵称等）

### #501 发布时重复发送

- **位置**：`xiaohongshu/publish.go:356-362`
- **根因**：
  - 点击发布按钮后未检查按钮 disabled 状态
  - 无防重复标志
  - 网络延迟期间页面可能允许再次点击
- **建议**：
  - 点击后验证按钮是否变为 disabled
  - 添加防重复标志
  - 等待页面反馈确认操作已处理

### #511 选择原创时发布失败

- **位置**：`xiaohongshu/publish.go:713-858`
- **根因**：
  - `checkbox.click()` 不触发 React/Vue 的 change 事件
  - 按钮 disabled 检查后即使重新勾选 checkbox 也直接返回错误，不重试
  - 选择器依赖 `div.footer`、`button.custom-button` 等类名，前端更新后可能失效
- **建议**：
  - 使用 `checkbox.checked = true` + `dispatchEvent(new Event('change', { bubbles: true }))`
  - 勾选后等待按钮状态更新再尝试点击
  - 添加重试机制

### #569 搜索 displayTitle 为空

- **验证数据**：搜索"露营"，图文类型，最新排序，返回 22 条结果全部 displayTitle 和 user 信息为空
- **可能原因**：搜索结果解析逻辑未正确提取 DOM 或 `__INITIAL_STATE__` 中的数据

### #578 评论加载不完整

- **验证数据**：`load_all_comments=true` 请求直接超时（无响应）
- **Issue 描述**：只能获取前 413 条评论，实际有 800+ 条
- **可能原因**：滚动加载逻辑在大量评论时性能不足或卡死

---

## Feature Request（特性请求）

| Issue | 需求 | 优先级 |
|-------|------|--------|
| #572 | 保存草稿到草稿箱 | 中 |
| #579 | 支持 rednote.com（海外用户） | 中 |
| #564 | 关注/取关用户 | 中 |
| #506 | Follow/Unfollow 用户 | 中 |
| #526 | 视频发布自定义封面图 | 低 |
| #553 | 支持小红书自带主图生成 | 低 |
| #454 | 收藏到指定收藏夹 | 低 |
| #449 | 获取收藏列表 | 低 |
| #373 | publish_content 添加合集参数 | 低 |
| #380 | 详情中增加是否挂商品链接字段 | 低 |
| #424 | 支持发长文 | 低 |
| #408 | 支持楼中楼回复 | 低 |
| #543 | 支持语音评论 | 低 |
| #396 | 支持搜索用户 | 低 |
| #444 | 发布返回文章 ID | 低 |

---

## 建议修复优先级

1. **#470 Chrome 进程泄漏** — 影响稳定性，长时间运行必崩
2. **#501 重复发布** — 影响核心功能，用户体验差
3. **#511 原创声明失败** — 影响核心功能
4. **#533/#504 评论回复失败** — 互动功能不可用
5. **#569 搜索数据缺失** — 搜索结果不完整
6. **#578 评论加载超时** — 大量评论场景不可用
