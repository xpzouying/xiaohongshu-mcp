# MCP → Web UI 覆盖与验收矩阵（17/17 基线）

> 冻结日期：2026-07-16  
> 契约来源：`mcp_server.go`、`account_handlers.go`、`account_tools.go`、`routes.go`、`account_rest_handlers.go`、`handlers_api.go`、`types.go`  
> UI 来源：`webui/main.go`、`webui/static/{app,accounts,search,publish,detail}.js`  
> 范围依据：已批准任务范围固定为 **7 个账号工具 + 10 个业务工具 = 17 个 MCP 接口**。`quick_add`、`sync_profile`、`user/me` 是 REST/UI 辅助能力，不虚构为 MCP 工具。

## 1. 使用方法与进度统计

每个接口只有同时满足以下条件才可将 `[ ]` 改为 `[x]`：

1. UI 中存在可发现入口；
2. UI 能表达 MCP 的全部有效参数及条件校验；
3. 调用层使用本文件冻结的 REST 映射，并保留 `account_id` 路由语义；
4. loading、success、empty、error 四态均有可观察反馈（动作类接口的 empty 为“不适用”，但需明确处理无返回数据）；
5. 对应自动化测试通过；依赖真实小红书会话的末端行为按人工场景复核。

进度算法：下列 17 个标题中的复选框计数，范围只能是 **0/17～17/17**。当前实现已补齐原有 4 个部分覆盖与 2 个缺失项，因此验收进度为 **17/17**。

| 状态 | 数量 | 接口 |
|---|---:|---|
| 完整 | 17 | 全部 17 个 MCP 工具，精确集合见第 5 节 |
| 部分 | 0 | — |
| 缺失 | 0 | — |

> 注：上表的精确完成集合以第 4 节逐项标题状态为唯一准绳；实现完成度与逐项状态必须同步，禁止只改总数。

## 2. 冻结的公共调用契约

### 2.1 调用层

前端唯一入口保持为：

```text
XHS.api(path, { method = "GET", body?, account = true, signal? }) -> Promise<data>
```

约束：

- `account !== false` 时，从 `XHS.state.selectedAccountId` 读取账号；GET 写入 `?account_id=`，有 body 的 POST/PUT/DELETE 写入 JSON `account_id`。
- 账号管理接口显式使用 `account:false`，账号 ID 在路径或创建 body 中传递。
- 成功 REST 信封：`{success:true,data:any,message?:string}`；失败信封：`{error:string,code:string,details?:any}`。
- 调用层必须以 HTTP 非 2xx、`success:false` 或存在 `error` 作为失败；错误规范化至少保留 `{message, code, status, details}`。
- 后续实现取消/超时时，在 `XHS.api` 透传 `AbortSignal`，不得为每页发明不同请求器。
- 当前 upstream 与 Web UI 代理均为一次请求/一次 JSON 响应，**17 个接口均无流式事件**。二维码登录的“异步”由 UI 每 3 秒调用 `check_login_status`，不是 SSE/WebSocket。

### 2.2 状态模型

每个功能的视图状态固定为：

```text
idle | loading | success | empty | error
```

- `loading`：提交按钮禁用，避免重复提交；耗时查询允许取消。
- `success`：展示结构化结果或明确 toast，不得只依赖控制台。
- `empty`：列表/详情显示可理解的空状态；动作返回空 data 时显示成功消息。
- `error`：保留当前输入，显示规范化错误；`ACCOUNT_LOGIN_REQUIRED` 引导到账号扫码登录。

### 2.3 权限与环境前提

- 账号工具要求 `AccountTools`/registry 初始化；业务工具要求 `account.Manager` 初始化。
- 业务工具必须能通过显式账号、默认账号或唯一账号解析；账号需处于 `active` 且有 cookie。
- `paused`、`risk_hold`、`disabled` 不得执行；`ACCOUNT_BUSY` 可提示稍后重试。
- Web UI 只访问同源 `/api/web/*`；代理只允许白名单映射到 `/api/v1/*`，请求体上限 1 MiB。
- 不在 UI、日志或错误详情中展示 cookie、完整凭据；`xsec_token` 只随当前请求使用。

## 3. 建议文件边界（并行实现必须遵守）

| 文件 | 唯一职责 | 并行修改规则 |
|---|---|---|
| `webui/static/app.js` | `XHS.api`、错误规范化、账号上下文、公共状态/toast/loading | 调用层任务独占；页面任务不得复制请求器 |
| `webui/static/accounts.js` / `accounts.html` | 7 个账号 MCP 工具对应交互 | 账号功能只改这里；`quick_add`/`sync_profile` 保留为增强流程 |
| `webui/static/search.js` / `search.html` | `list_feeds`、`search_feeds`、跳转详情、用户主页入口 | 浏览类功能只改这里 |
| `webui/static/detail.js` / `detail.html` | `get_feed_detail`、点赞、收藏、评论、回复 | 互动类功能只改这里 |
| `webui/static/publish.js` / `publish.html` | 图文与视频发布 | 发布类功能只改这里 |
| `webui/main.go` | `/api/web/*` → `/api/v1/*` 静态白名单，不承载业务状态 | 代理契约任务独占 |
| `webui/server_test.go` | 路由、方法、代理、安全与静态契约 | 后端代理验收 |
| 建议新增 `webui/static/mcp-contract.js` | 17 个 tool 常量、参数序列化/校验（纯函数） | 调用层独占，页面只消费导出函数 |
| 建议新增 `webui/static/mcp-contract.test.js` | 17/17 清单及序列化、校验、错误/取消测试 | 用固定 tool 名集合防遗漏/合并/虚构 |

稳定函数建议：`callTool(toolName, input, {signal}={})`、`validateToolInput(toolName,input)`、`serializeToolInput(toolName,input)`。其内部仍调用 REST 映射，不要求浏览器直连 `/mcp`。

## 4. 17 个接口逐项实现矩阵

### 01. [x] `list_accounts` — 完整

- 请求/校验：MCP 无参数；REST `GET /api/web/accounts`，必须 `account:false`。
- 响应：MCP text 为 `Account[]` JSON；REST data 为 `{accounts: Account[], default_account_id: string|null}`。`Account={id,display_name,owner?,purpose?,status,created_at,updated_at}`。
- UI：`accounts.html` 加载/刷新 → `accounts.js:refresh()` → 表格；全局选择器也复用该结果。
- 四态：刷新期间 loading；有数据渲染表格；空数组显示“暂无账号”；错误 toast 且页面不崩溃。
- 前提：账号 registry 可读；无需已登录。
- 复用/缺口：`XHS.loadAccounts`、`renderAccounts`、账号 REST 已存在；无功能缺口。
- 验收：mock 0/2 个账号及 500 错误，分别观察 empty、正确默认徽章、error；断言请求不携带选中账号。

### 02. [x] `create_account` — 完整

- 请求/校验：`account_id` 必填且匹配 `^[a-z][a-z0-9_]{2,31}$`，长度 3～32，且不为 `accounts/system/root/null/unknown`；`display_name` 必填非空；`owner`、`purpose` 可选字符串。
- 响应：MCP text/REST data 均为新 `Account`，初始 `status=needs_login`；重复或非法 ID 返回账号错误码。
- UI：“高级创建账号”表单表达 ID、名称、Owner、Purpose，提交 `POST /api/web/accounts` body `{id,display_name,owner,purpose}`；成功后刷新并提供“扫码登录”。快速扫码添加继续作为增强流程保留。
- 四态：提交按钮 loading/禁用；成功显示新行；无 data 视为协议错误；字段级校验/服务错误保留输入。
- 前提：registry 可写；无需登录。
- 复用/缺口：REST、`AccountTools.Create` 和 quick-add 创建后的刷新/扫码流程均已复用；无功能缺口。
- 验收：合法创建后字段逐项一致；非法/保留/重复 ID 不发请求或显示明确错误；网络失败可重试且不重复创建。

### 03. [x] `remove_account` — 完整

- 请求/校验：`account_id` 必填并符合账号 ID 规则；REST `DELETE /api/web/accounts/{encodeURIComponent(id)}`。
- 响应：`{account_id}`；错误包括 not found/busy。
- UI：账号行“删除”→ 不可撤销二次确认 → 删除 → 刷新。
- 四态：操作中禁用该行；成功 toast/行消失；空响应按成功消息处理；失败保留行并 toast。
- 前提：registry/management/cookie store 可用；账号不能忙。
- 复用/缺口：现有 `accounts.js` 已实现。
- 验收：取消确认不请求；确认只发一次 DELETE；默认/当前账号被删后选择器回退。

### 04. [x] `set_default_account` — 完整

- 请求/校验：合法且存在的 `account_id`；REST `PUT /api/web/accounts/{id}/default`。
- 响应：`{account_id}`。
- UI：账号行“设为默认”→ 刷新默认徽章。
- 四态：按钮 loading/禁用；成功徽章唯一；空 data 仍按 HTTP 成功；错误保留旧默认。
- 前提：registry 可写；无需登录。
- 复用/缺口：现有实现可复用。
- 验收：切换后列表响应的 `default_account_id` 匹配；不存在账号显示 404 错误。

### 05. [x] `check_login_status` — 完整

- 请求/校验：`account_id` 必填且存在；REST `POST /api/web/accounts/{id}/login/status`。
- 响应：`{account_id,is_logged_in,identity?}`。
- UI：二维码弹窗每 3 秒轮询；每个账号行同时提供可发现的“检查状态”独立动作。
- 四态：检查时行级 loading；成功展示已登录/未登录及非敏感 identity；空 identity 合法；错误可见，不能像当前轮询 `catch (_){}` 一样永久吞掉。
- 前提：该账号登录浏览器会话可建立；无需已有 cookie 才能检查。
- 复用/缺口：轮询路径和成功同步已复用；独立触发具备行级 loading 与显式错误反馈。
- 验收：未登录、已登录、接口失败三种 mock 均有明确状态；关闭弹窗后停止轮询；同一账号不产生重叠轮询。

### 06. [x] `get_login_qrcode` — 完整

- 请求/校验：`account_id` 必填且存在；REST `POST /api/web/accounts/{id}/login/qrcode`。
- 响应：MCP 为 JSON text + `image/png` content；REST `{account_id,image?,is_logged_in}`，`image` 可为裸 base64 或 data URI。
- UI：账号行“扫码登录”及 quick-add 流程 → dialog 图片 → 启动状态轮询。
- 四态：获取时全局 loading；二维码成功展示；已登录且无图为合法空图分支；失败 toast。
- 前提：浏览器可启动、能访问小红书登录页。
- 复用/缺口：`openQR/showQRDialog` 已兼容两种图片格式。
- 验收：裸 base64/data URI 都正确显示；`is_logged_in=true` 不打开空弹窗；关闭 dialog 清除 timer。

### 07. [x] `reset_login` — 完整

- 请求/校验：合法 `account_id`；REST `DELETE /api/web/accounts/{id}/login`。
- 响应：`{account_id}`；busy 返回 429/`ACCOUNT_BUSY`。
- UI：账号行“重置”→ 二次确认 → 刷新状态。
- 四态：loading/禁用；成功变为 needs_login；空 data 显示成功；失败保留原状态。
- 前提：cookie store 可写；需取得账号管理锁。
- 复用/缺口：现有实现可复用。
- 验收：取消不请求；成功后 cookie 状态不泄漏且 UI 为需登录；busy 显示可重试提示。

### 08. [x] `publish_content` — 完整

- 请求/校验：`account_id?`；`title` 必填且 ≤20 Unicode 字符；`content` 字符串且 UI 限 ≤1000；`images` 至少 1 个非空 URL/绝对路径；`tags/products` 字符串数组；`schedule_at` 空或 ISO8601 且距当前 1h～14d；`is_original` bool；`visibility` 为 `公开可见|仅自己可见|仅互关好友可见`。
- 响应：REST `PublishResponse={title,content,images:number,status}`；MCP 为成功/失败 text。
- UI：发布页“图文”Tab → 表单 → `/api/web/publish`。
- 四态：发布中 overlay/防重复；成功 toast；空响应视协议错误；校验或服务错误保留表单。
- 前提：active 登录账号；图片 URL 可下载或路径在 upstream 主机可读；商品绑定需账号权限。
- 复用/缺口：现有 `publish.js` 已覆盖参数和前端主要校验。
- 验收：完整 body 精确序列化；标题 21 字、无图片、越界时间均不请求；成功/500 均明确反馈。

### 09. [x] `list_feeds` — 完整

- 请求/校验：仅可选 `account_id`；REST `GET /api/web/feeds/list`（前端账号通过 query 注入）。
- 响应：`{feeds: Feed[],count:number}`；Feed 至少消费 `id,xsecToken,noteCard`。
- UI：发现页具有可发现的“推荐流/刷新”入口，复用 `renderFeeds`，不以空关键词调用搜索替代。
- 四态：列表 loading；卡片 success；`feeds=[]` empty；错误 toast + 可重试。
- 前提：active 登录账号。
- 复用/缺口：代理白名单与 `renderFeeds` 已复用；无功能缺口。
- 验收：点击推荐入口只发一次 GET 且带账号 query；0/多条/错误分别正确展示；卡片保留 feed ID/token 跳详情。

### 10. [x] `search_feeds` — 完整

- 请求/校验：`keyword` 必填非空；filters 枚举：`sort_by=综合|最新|最多点赞|最多评论|最多收藏`，`note_type=不限|视频|图文`，`publish_time=不限|一天内|一周内|半年内`，`search_scope=不限|已看过|未看过|已关注`，`location=不限|同城|附近`；可选 `account_id`。
- 响应：同 list_feeds。
- UI：搜索页关键词/五组筛选/回车或按钮 → 卡片网格。
- 四态：loading overlay；结果/count；无结果提示；错误 toast。
- 前提：active 登录账号。
- 复用/缺口：现有实现完整。
- 验收：五个筛选值原样序列化；空关键词不请求；0/多条/500 三分支可见。

### 11. [x] `get_feed_detail` — 完整

- 请求/校验：`feed_id`、`xsec_token` 必填；`load_all_comments` bool；仅为 true 时启用 `limit`（正整数，默认 20）、`click_more_replies` bool、`reply_limit`（正整数，默认 10）、`scroll_speed=slow|normal|fast`。REST 要转换为 `{comment_config:{max_comment_items,max_replies_threshold,...}}`。
- 响应：`{feed_id,data:{note,comments}}`；comments 可为空。
- UI：详情页自动加载，并提供“加载全部评论”、条件配置与取消按钮。
- 四态：详情 skeleton/loading；内容/评论成功；缺参数或无评论 empty；错误在内容区显示并可重试，不能只有 toast 后空白。
- 前提：active 登录账号；ID/token 来自 feed 结果。
- 复用/缺口：基本详情、图片、互动、高级评论参数、长请求取消与内容区 error 均已实现。
- 验收：默认请求不发送无效高级配置；启用后 20/10/normal 正确转换；取消产生 AbortError 友好状态；空评论明确显示。

### 12. [x] `user_profile` — 完整

- 请求/校验：`user_id`、`xsec_token` 必填非空；可选 `account_id`。
- 响应：REST data 当前包装为 `{data: UserProfileResult}`；结果包含 `userBasicInfo`、`interactions[]`、`feeds[]`。调用层应只做一次稳定解包，页面不得各自猜层级。
- UI：搜索卡片和详情作者区提供可访问的作者链接，进入用户主页展示头像、昵称、简介、关注/粉丝/获赞收藏和笔记列表。
- 四态：资料 loading；结构化 success；无简介/无笔记 empty；错误区可重试。
- 前提：active 登录账号；必须从 feed 数据取得 user ID 和有效 token。
- 复用/缺口：代理 `/api/web/user/profile` 与 Feed 卡片渲染已复用；无功能缺口。
- 验收：搜索卡片和详情作者至少一处可发现；参数精确传递；空 feeds 与 500 不崩溃；外部图片内容经转义且不插入脚本。

### 13. [x] `post_comment_to_feed` — 完整

- 请求/校验：`feed_id`、`xsec_token`、trim 后非空 `content` 必填；可选 `account_id`。
- 响应：`{feed_id,success,message}`。
- UI：详情页评论表单。
- 四态：现有详情级流程提供操作反馈；成功清空并刷新；空内容不请求；错误可见。后续统一状态层可再补按钮级 loading/禁用，但不影响当前完整映射认定。
- 前提：active 登录账号；账号具有评论权限。
- 复用/缺口：调用、空内容阻断、成功清空/刷新与错误 toast 已存在；后续可增强按钮级 loading 和重复提交保护。
- 验收：双击只产生一次 POST；空白内容给 warning；失败后文本仍在；成功后刷新评论。

### 14. [x] `reply_comment_in_feed` — 完整

- 请求/校验：`feed_id`、`xsec_token`、非空 `content` 必填；`comment_id` 与 `user_id` 至少一个非空；可选 `account_id`。
- 响应：`{feed_id,target_comment_id?,target_user_id?,success,message}`。
- UI：评论“回复”→ dialog，目标字段预填 → 提交。
- 四态：dialog 内 loading/禁用；成功关闭并刷新；目标缺失/空内容字段错误；失败保留 dialog 与输入。
- 前提：active 登录账号；目标评论/用户仍存在。
- 复用/缺口：基本流程、loading、防重复、空内容与 required_without 前端提示均已实现。
- 验收：仅 comment_id、仅 user_id、二者都有均可；二者都无时不请求；失败不关闭 dialog；成功刷新。

### 15. [x] `publish_with_video` — 完整

- 请求/校验：`title` 必填 ≤20 字；`content` UI 限 ≤1000；`video` 必填且为 upstream 主机可读的本地绝对路径；`tags/products` 数组；`schedule_at` 空或 1h～14d ISO8601；visibility 枚举；可选 `account_id`。
- 响应：`{title,content,video,status}`；MCP text。
- UI：发布页“视频”Tab → `/api/web/publish_video`。
- 四态：loading/防重复；成功 status；空响应协议错误；校验/服务错误保留表单。
- 前提：active 登录账号；视频文件位于 upstream 运行环境而非浏览器本机沙箱。
- 复用/缺口：现有实现完整，并已明确本地路径语义。
- 验收：空路径/越界时间不请求；图文/视频 Tab 不串字段；成功和错误 toast 正确。

### 16. [x] `like_feed` — 完整

- 请求/校验：`feed_id`、`xsec_token` 必填；`unlike` bool 默认 false；可选 `account_id`。
- 响应：`ActionResult={feed_id,success,message}`，服务端幂等跳过已处于目标状态的操作。
- UI：详情页点赞按钮，依据 `interactInfo.liked` 决定 `unlike`。
- 四态：操作中按钮应禁用；成功刷新计数；无 data 仍反馈；错误 toast 且恢复按钮。
- 前提：active 登录账号。
- 复用/缺口：触发/刷新已存在；后续统一状态层时补按钮级禁用但不阻塞当前完整映射认定。
- 验收：liked false/true 分别发 `unlike:false/true`；错误不反转本地状态；快速双击不产生矛盾最终态。

### 17. [x] `favorite_feed` — 完整

- 请求/校验：`feed_id`、`xsec_token` 必填；`unfavorite` bool 默认 false；可选 `account_id`。
- 响应：同 `ActionResult`。
- UI：详情页收藏按钮，依据 `interactInfo.collected` 决定 `unfavorite`。
- 四态：按钮 loading/恢复；成功刷新；空 data 反馈；错误 toast。
- 前提：active 登录账号。
- 复用/缺口：现有实现可复用。
- 验收：collected false/true 分别映射 false/true；错误不污染状态；重复操作保持幂等。

## 5. 逐项自动化验收清单

建议在 `webui/static/mcp-contract.test.js` 固定以下精确集合；测试必须断言集合相等而非只断言数量，以防遗漏、合并或用辅助 REST 能力冒充 MCP：

```text
list_accounts
create_account
remove_account
set_default_account
check_login_status
get_login_qrcode
reset_login
publish_content
list_feeds
search_feeds
get_feed_detail
user_profile
post_comment_to_feed
reply_comment_in_feed
publish_with_video
like_feed
favorite_feed
```

最低自动化矩阵（2026-07-17 联调证据）：

- [x] 精确名称集合 17/17，无额外名称：`mcp-contract.test.js` 断言精确集合，Node 13/13 通过。
- [x] 每项至少一个 UI selector/入口存在且可触发：`server_test.go:TestMCPWebCoverageStaticContracts` 逐项绑定页面、脚本和入口 token。
- [x] 每项请求 method/path/body/query 与本文件一致：`TestMCPWebCoverageProxyContracts` 与 `mcp-contract.test.js` 双层验证；页面统一经 `XHS.callTool` 接入，不再复制请求映射。
- [x] 必填、枚举、条件参数校验：契约测试覆盖账号 ID、空关键词、非法筛选、详情正整数、回复 `required_without`、图片/视频素材；浏览器原生 required/min/max 补充基础字段约束。
- [x] success/error/empty/loading 与并发：契约状态层覆盖四态并取消旧请求；页面显示结构化内容或 toast/内容区错误，表单和互动按钮在 pending 时禁用。
- [x] `ACCOUNT_LOGIN_REQUIRED`、`ACCOUNT_BUSY`、`UPSTREAM_UNAVAILABLE` 由 `app.js:normalizeError`/错误消息表统一归一化。
- [x] 二维码 dialog 关闭会清理轮询 timer；详情请求通过 `AbortController` 可取消，竞态回归测试保留五个高级参数。
- [x] 质量门禁：`node --test webui/static/*.test.js` 13/13、全量 `node --check`、`go test ./... -count=1`、`go test -race ./... -count=1`、`go vet ./...`、`go build ./...`、显式 `go build -o /tmp/xhs-webui-task-a9e86a9c ./webui`、`git diff --check` 全部通过。
- [x] 真实服务只读冒烟：Web UI `127.0.0.1:18081` 代理实际 MCP 服务 `127.0.0.1:18060`；health、账号列表与静态页面均为 HTTP 200；使用默认 active 账号调用推荐流为 HTTP 200，返回 35 条真实 Feed。未执行发布、评论、回复、点赞、收藏、重置或删除等写操作。

## 6. 当前代码差异与风险记录

1. `docs/web-ui-spec.md` 冻结于旧 commit `8618237`，其中“账号 REST 不存在”的发现已过期；当前 `account_rest_handlers.go` 和 `routes.go` 已实现账号 REST。后续以实时代码和本文为准。
2. `mcp_server.go` 末尾日志仍写死 `Registered 13 MCP tools`，多账号模式实际注册 7 个账号工具加 10 个业务工具；该日志不能作为 17 项清单来源，后续实现任务应修正为动态/准确计数。
3. 旧单账号工具 `delete_cookies` 在多账号模式被 `reset_login` 替代，不属于批准的 17 项，Web UI 不应为覆盖数字重复计入它。
4. REST 辅助能力 `quick_add`、`sync_profile`、`user/me` 有产品价值，但不是 MCP 工具；测试应允许它们存在，却不能把它们计入 17/17。
5. `XHS.api` 已保留 status/details/AbortSignal，`XHS.callTool` 将 17 项页面统一接入 `mcp-contract.js` 的校验、序列化、超时和取消契约；`quick_add`、`sync_profile`、health 等非 MCP 辅助流程继续直接调用 REST。
