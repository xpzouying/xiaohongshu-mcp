# Web UI 功能规格与 API 映射

> 冻结日期：2026-07-16  
> 基于 commit `8618237`（分支 `feature/web-ui`）  
> 审计范围：`routes.go`、`handlers_api.go`、`types.go`、`service.go`、`account/`、`xiaohongshu/types.go`、`xiaohongshu/search.go`、`middleware.go`

---

## 0. 关键审计发现（T2/T3 必读）

### 0.1 真实 REST 路由前缀是 `/api/v1/`，不是 `/api/`

任务描述中的 `POST /api/login/status` 等简写路径在实际代码中全部注册为 `/api/v1/*`：

```
GET    /api/v1/login/status        ← 注意有 v1
GET    /api/v1/login/qrcode
DELETE /api/v1/login/cookies
POST   /api/v1/publish
POST   /api/v1/publish_video
GET    /api/v1/feeds/list
GET    /api/v1/feeds/search
POST   /api/v1/feeds/search
POST   /api/v1/feeds/detail
POST   /api/v1/user/profile
POST   /api/v1/feeds/comment
POST   /api/v1/feeds/comment/reply
POST   /api/v1/feeds/like
POST   /api/v1/feeds/favorite
GET    /api/v1/user/me
```

来源：`routes.go:119-136`。T2 的代理层转发时**必须**映射到 `/api/v1/*`。

### 0.2 当前后端无任何账号管理 REST 路由

任务描述列出的 `POST /api/accounts`、`GET /api/accounts`、`DELETE /api/accounts/{id}` 等在 `routes.go` 中**完全不存在**。账号管理能力当前只通过 MCP tools 暴露（`account_handlers.go` → `AccountTools`），包括 List / Create / Remove / SetDefault / CheckLoginStatus / GetLoginQRCode / ResetLogin。

**T2 必须二选一：**
- **方案 A（推荐）**：在 upstream `routes.go` 新增 `/api/v1/accounts` REST 路由组（薄封装 `AccountTools`），然后代理层纯转发。
- **方案 B**：代理层直接调用 `AccountTools`（同进程）或自行实现等价逻辑——但 T2 任务体要求"通过 HTTP client 调 18060"，所以方案 B 违背架构约束。

本规格 §3 给出方案 A 的推荐 REST 契约，供 T2 在 upstream 补路由时参照。

### 0.3 旧版登录接口（`/api/v1/login/*`）不走账号路由

`checkLoginStatusHandler`、`getLoginQrcodeHandler`、`deleteCookiesHandler` 直接操作全局 cookie 文件（`cookies.GetCookiesFilePath()`），没有 `withRESTAccountRouting` 包装。多账号场景下这些接口应被视为**遗留兼容**，Web UI 应优先使用 §3 的账号级登录接口。

### 0.4 `account_id` 传递方式

账号路由通过 `restAccountID()`（`routes.go:42-65`）解析，支持两种方式：
- **Query 参数**：`?account_id=xxx`（所有 HTTP 方法）
- **JSON body 字段**：`{"account_id": "xxx", ...}`（POST/PUT/DELETE，自动读取 body 后还原）

Web UI 前端发请求时统一在 JSON body 里带 `account_id` 即可（GET 请求放 query）。

---

## 1. 技术栈选型

| 层 | 选型 | 理由 |
|---|---|---|
| 后端 | 单一 Go binary，`net/http` 标准库（不引入 gin/echo） | 遵循 T2 约束 |
| 前端 | vanilla JS + HTML/CSS，无框架无 CDN | 遵循 T3 约束 |
| 静态资源 | `go:embed` 嵌入 `webui/static/` | 单 binary 部署 |
| 端口 | Web UI `:18080`，upstream MCP `:18060` | 不冲突生产/staging |
| 通信 | Web UI 后端 → upstream REST（HTTP client） | 前端不直连 18060 |

---

## 2. 页面清单（5 页 + 通用组件）

### 2.1 Dashboard（`index.html`）

| 区域 | 数据源 | 交互 |
|---|---|---|
| 服务健康卡 | `GET /api/web/health` → `{status, service, account, timestamp}` | 加载时自动请求 |
| 默认账号卡 | `GET /api/web/accounts` → 取 `default_account_id` 对应项 | 显示名称+状态徽章 |
| 快速入口 | 静态链接 | 4 个卡片跳转：账号管理、搜索浏览、内容发布、帖子详情 |

### 2.2 账号管理（`accounts.html`）

| 功能 | 前端操作 | 后端代理 → upstream |
|---|---|---|
| 账号列表 | 表格渲染 | `GET /api/web/accounts` → upstream `GET /api/v1/accounts` |
| 创建账号 | 表单：ID、名称、Owner、Purpose | `POST /api/web/accounts` → upstream `POST /api/v1/accounts` |
| 获取二维码 | 点击"扫码登录"按钮 | `POST /api/web/accounts/{id}/login/qrcode` → upstream |
| 轮询状态 | 二维码弹窗内每 3s 轮询 | `POST /api/web/accounts/{id}/login/status` → upstream |
| 设为默认 | 点击"设为默认" | `PUT /api/web/accounts/{id}/default` → upstream |
| 重置登录 | 点击"重置登录"（二次确认） | `DELETE /api/web/accounts/{id}/login` → upstream |
| 删除账号 | 点击"删除"（二次确认） | `DELETE /api/web/accounts/{id}` → upstream |

状态徽章颜色映射：

| Status | 颜色 | 含义 |
|---|---|---|
| `active` | 绿 | 已登录可用 |
| `needs_login` | 橙 | 需要扫码登录 |
| `paused` | 灰 | 暂停 |
| `risk_hold` | 红 | 风控冻结 |
| `disabled` | 深灰 | 已禁用 |

### 2.3 搜索浏览（`search.html`）

| 区域 | 说明 |
|---|---|
| 搜索栏 | 关键词输入 + 回车/按钮触发 |
| 筛选面板 | 5 组下拉，枚举值见 §4.1 |
| 结果网格 | 卡片式瀑布流：封面图 + 标题 + 作者 + 互动数 |
| 卡片点击 | 跳转 `detail.html?feed_id=xxx&xsec_token=yyy` |

筛选枚举（来自 `xiaohongshu/search.go:37-67`）：

```
sort_by:      综合 | 最新 | 最多点赞 | 最多评论 | 最多收藏
note_type:    不限 | 视频 | 图文
publish_time: 不限 | 一天内 | 一周内 | 半年内
search_scope: 不限 | 已看过 | 未看过 | 已关注
location:     不限 | 同城 | 附近
```

### 2.4 内容发布（`publish.html`）

| Tab | 字段 | 校验规则 |
|---|---|---|
| 图文 | title, content, images[], tags[], schedule_at, visibility, is_original, products[] | 标题 ≤20字；正文 ≤1000字；图片 ≥1张；schedule_at 为 ISO8601 且在 1h~14d 范围 |
| 视频 | title, content, video, tags[], schedule_at, visibility, products[] | video 为本地路径（前端仅展示提示，实际上传由后端处理） |

visibility 枚举：`公开可见`（默认）、`仅自己可见`、`仅互关好友可见`

**定时发布时间选择器**：前端用 `<input type="datetime-local">`，提交时转为 ISO8601。

### 2.5 帖子详情/互动（`detail.html`）

URL 参数：`?feed_id=xxx&xsec_token=yyy`

| 区域 | 数据源 |
|---|---|
| 笔记内容 | 标题、描述、图片列表/视频 |
| 作者信息 | 头像、昵称、用户 ID |
| 互动数据 | 点赞数、收藏数、分享数、评论数 |
| 操作按钮 | 点赞/取消、收藏/取消 |
| 评论列表 | 顶层评论 + 子评论（展开/折叠） |
| 评论操作 | 发表评论、回复评论 |

---

## 3. API 映射（Web UI 代理层 → upstream）

### 3.1 Web UI 自有路由

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/` | 首页（index.html） |
| GET | `/static/*` | 静态资源 |
| GET | `/api/web/health` | Web UI 自身健康检查 |

### 3.2 账号管理代理（需 T2 在 upstream 补 REST 路由）

> 以下 `/api/web/accounts/*` 代理到 upstream `/api/v1/accounts/*`。upstream 路由当前不存在，需 T2 新增。

#### GET /api/web/accounts → upstream GET /api/v1/accounts

**响应**（来源：`account.Account` + `registryDocument.default_account_id`）：

```json
{
  "success": true,
  "data": {
    "default_account_id": "acct_main",
    "accounts": [
      {
        "id": "acct_main",
        "display_name": "主账号",
        "owner": "",
        "purpose": "",
        "status": "active",
        "created_at": "2026-07-16T10:00:00Z",
        "updated_at": "2026-07-16T10:00:00Z"
      }
    ]
  },
  "message": "获取账号列表成功"
}
```

Account ID 校验规则（`account/types.go:96-107`）：`^[a-z][a-z0-9_]{2,31}$`，保留词：accounts/system/root/null/unknown。

Status 枚举：`active` | `needs_login` | `paused` | `risk_hold` | `disabled`

#### POST /api/web/accounts → upstream POST /api/v1/accounts

**请求**（来源：`account.CreateAccountInput`）：

```json
{
  "id": "acct_new",
  "display_name": "新账号",
  "owner": "可选",
  "purpose": "可选"
}
```

**响应 data**：单个 `account.Account` 对象，`status` 初始为 `needs_login`。

#### DELETE /api/web/accounts/{id} → upstream DELETE /api/v1/accounts/{id}

**响应 data**：`{"account_id": "acct_new"}`

#### PUT /api/web/accounts/{id}/default → upstream PUT /api/v1/accounts/{id}/default

**响应 data**：`{"account_id": "acct_new"}`

#### POST /api/web/accounts/{id}/login/qrcode → upstream POST /api/v1/accounts/{id}/login/qrcode

**响应**（来源：`AccountQRCode`，`account_tools.go:21-25`）：

```json
{
  "success": true,
  "data": {
    "account_id": "acct_new",
    "image": "<base64 PNG 或 data URI>",
    "is_logged_in": false
  }
}
```

> `image` 字段为 base64 编码的 PNG 图片数据。前端用 `data:image/png;base64,<image>` 渲染。

#### POST /api/web/accounts/{id}/login/status → upstream POST /api/v1/accounts/{id}/login/status

**响应**（来源：`AccountLoginStatus`，`account_tools.go:15-19`）：

```json
{
  "success": true,
  "data": {
    "account_id": "acct_new",
    "is_logged_in": false,
    "identity": ""
  }
}
```

#### DELETE /api/web/accounts/{id}/login → upstream DELETE /api/v1/accounts/{id}/login

重置指定账号的登录状态。**响应 data**：`{"account_id": "acct_new"}`

### 3.3 业务功能代理

所有业务接口代理到 upstream `/api/v1/*`，前端请求时在 body 或 query 中带 `account_id`。

#### 通用响应信封（`types.go:8-19`）

```json
// 成功
{ "success": true, "data": {...}, "message": "操作成功" }

// 错误
{ "error": "错误描述", "code": "ERROR_CODE", "details": "..." }
```

#### 健康检查

`GET /api/web/health`（代理到 upstream `GET /health`）

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "service": "xiaohongshu-mcp",
    "account": "ai-report",
    "timestamp": "now"
  }
}
```

#### 推荐流

`POST /api/web/feeds/list`（代理到 upstream `GET /api/v1/feeds/list`）

**请求**：`{"account_id": "acct_main"}`

**响应 data**（来源：`FeedsListResponse`，`service.go:108-112`）：

```json
{
  "feeds": [
    {
      "xsecToken": "abc",
      "id": "feed123",
      "modelType": "note",
      "noteCard": {
        "type": "normal",
        "displayTitle": "标题",
        "user": { "userId": "u1", "nickname": "作者", "nickName": "作者", "avatar": "url" },
        "interactInfo": {
          "liked": false,
          "likedCount": "123",
          "sharedCount": "45",
          "commentCount": "6",
          "collectedCount": "78",
          "collected": false
        },
        "cover": { "width": 800, "height": 600, "url": "...", "urlPre": "...", "urlDefault": "..." }
      },
      "index": 0
    }
  ],
  "count": 1
}
```

#### 搜索

`POST /api/web/feeds/search`（代理到 upstream `POST /api/v1/feeds/search`）

**请求**（来源：`SearchFeedsRequest`，`types.go:57-60`）：

```json
{
  "account_id": "acct_main",
  "keyword": "关键词",
  "filters": {
    "sort_by": "综合",
    "note_type": "不限",
    "publish_time": "不限",
    "search_scope": "不限",
    "location": "不限"
  }
}
```

**响应 data**：同推荐流 `FeedsListResponse` 结构。

#### 帖子详情

`POST /api/web/feeds/detail`（代理到 upstream `POST /api/v1/feeds/detail`）

**请求**（来源：`FeedDetailRequest`，`types.go:50-55`）：

```json
{
  "account_id": "acct_main",
  "feed_id": "feed123",
  "xsec_token": "abc",
  "load_all_comments": false,
  "comment_config": {
    "click_more_replies": false,
    "max_replies_threshold": 0,
    "max_comment_items": 0,
    "scroll_speed": "normal"
  }
}
```

**响应 data**（来源：`FeedDetailResponse` → `xiaohongshu.FeedDetailResponse`）：

```json
{
  "feed_id": "feed123",
  "data": {
    "note": {
      "noteId": "feed123",
      "xsecToken": "abc",
      "title": "标题",
      "desc": "正文描述",
      "type": "normal",
      "time": 1721000000,
      "ipLocation": "上海",
      "user": { "userId": "u1", "nickname": "作者", "avatar": "url" },
      "interactInfo": { "liked": false, "likedCount": "123", "commentCount": "6", "collectedCount": "78", "collected": false },
      "imageList": [
        { "width": 800, "height": 600, "urlDefault": "...", "urlPre": "..." }
      ]
    },
    "comments": {
      "list": [
        {
          "id": "c1",
          "noteId": "feed123",
          "content": "评论内容",
          "likeCount": "5",
          "createTime": 1721000000,
          "ipLocation": "北京",
          "liked": false,
          "userInfo": { "userId": "u2", "nickname": "评论者", "avatar": "url" },
          "subCommentCount": "2",
          "subComments": []
        }
      ],
      "cursor": "",
      "hasMore": false
    }
  }
}
```

#### 发布图文

`POST /api/web/publish`（代理到 upstream `POST /api/v1/publish`）

**请求**（来源：`PublishRequest`，`service.go:55-64`）：

```json
{
  "account_id": "acct_main",
  "title": "标题（≤20字）",
  "content": "正文（≤1000字）",
  "images": ["https://example.com/1.jpg", "/local/path/2.png"],
  "tags": ["标签1"],
  "schedule_at": "2026-07-17T10:00:00+08:00",
  "is_original": true,
  "visibility": "公开可见",
  "products": ["商品关键词"]
}
```

**响应 data**（来源：`PublishResponse`，`service.go:80-86`）：

```json
{ "title": "标题", "content": "正文", "images": 2, "status": "发布完成" }
```

#### 发布视频

`POST /api/web/publish_video`（代理到 upstream `POST /api/v1/publish_video`）

**请求**（来源：`PublishVideoRequest`，`service.go:89-97`）：

```json
{
  "account_id": "acct_main",
  "title": "标题",
  "content": "正文",
  "video": "/local/path/video.mp4",
  "tags": [],
  "schedule_at": "",
  "visibility": "公开可见",
  "products": []
}
```

**响应 data**（来源：`PublishVideoResponse`，`service.go:100-106`）：

```json
{ "title": "标题", "content": "正文", "video": "/local/path/video.mp4", "status": "发布完成" }
```

#### 用户主页

`POST /api/web/user/profile`（代理到 upstream `POST /api/v1/user/profile`）

**请求**（来源：`UserProfileRequest`，`types.go:101-104`）：

```json
{ "account_id": "acct_main", "user_id": "u1", "xsec_token": "abc" }
```

**响应 data**（来源：`UserProfileResponse`，`service.go:114-119`）：

```json
{
  "data": {
    "userBasicInfo": {
      "gender": 0, "ipLocation": "上海", "desc": "简介",
      "imageb": "背景图URL", "nickname": "昵称", "images": "头像URL", "redId": "小红书号"
    },
    "interactions": [
      { "type": "follows", "name": "关注", "count": "100" },
      { "type": "fans", "name": "粉丝", "count": "5000" },
      { "type": "interaction", "name": "获赞与收藏", "count": "10000" }
    ],
    "feeds": [ /* Feed 数组 */ ]
  }
}
```

#### 我的资料

`GET /api/web/user/me`（代理到 upstream `GET /api/v1/user/me`）

**请求**：Query `?account_id=acct_main`

**响应**：同 user/profile 结构。

#### 发表评论

`POST /api/web/feeds/comment`（代理到 upstream `POST /api/v1/feeds/comment`）

**请求**（来源：`PostCommentRequest`，`types.go:69-73`）：

```json
{ "account_id": "acct_main", "feed_id": "f1", "xsec_token": "abc", "content": "评论内容" }
```

**响应 data**（来源：`PostCommentResponse`，`types.go:76-80`）：

```json
{ "feed_id": "f1", "success": true, "message": "评论发表成功" }
```

#### 回复评论

`POST /api/web/feeds/comment/reply`（代理到 upstream `POST /api/v1/feeds/comment/reply`）

**请求**（来源：`ReplyCommentRequest`，`types.go:82-89`）：

```json
{
  "account_id": "acct_main",
  "feed_id": "f1",
  "xsec_token": "abc",
  "comment_id": "c1",
  "user_id": "u2",
  "content": "回复内容"
}
```

> `comment_id` 和 `user_id` 至少提供一个（`required_without`）。

**响应 data**（来源：`ReplyCommentResponse`，`types.go:92-98`）：

```json
{
  "feed_id": "f1",
  "target_comment_id": "c1",
  "target_user_id": "u2",
  "success": true,
  "message": "评论回复成功"
}
```

#### 点赞/取消点赞

`POST /api/web/feeds/like`（代理到 upstream `POST /api/v1/feeds/like`）

**请求**（来源：`LikeFeedRequest`，`types.go:106-110`）：

```json
{ "account_id": "acct_main", "feed_id": "f1", "xsec_token": "abc", "unlike": false }
```

**响应 data**（来源：`ActionResult`，`types.go:119-123`）：

```json
{ "feed_id": "f1", "success": true, "message": "点赞成功或已点赞" }
```

#### 收藏/取消收藏

`POST /api/web/feeds/favorite`（代理到 upstream `POST /api/v1/feeds/favorite`）

**请求**（来源：`FavoriteFeedRequest`，`types.go:112-116`）：

```json
{ "account_id": "acct_main", "feed_id": "f1", "xsec_token": "abc", "unfavorite": false }
```

**响应 data**：同 ActionResult 结构。

---

## 4. 安全约束

### 4.1 路径校验

- Account ID 必须匹配 `^[a-z][a-z0-9_]{2,31}$`（`account/types.go:96`）
- 代理层禁止转发到非 `/api/v1/*` 的 upstream 路径（防止 SSRF）
- 静态文件服务禁用目录遍历（`filepath.Clean` + 前缀校验）

### 4.2 凭据隔离

- **不向前端暴露** cookie 文件路径、cookie 内容、token 原文
- 账号列表响应中不包含任何凭据字段（`Account` struct 本身无敏感字段）
- 二维码 `image` 字段仅返回 base64 图片数据，不含 cookie

### 4.3 CORS

- Web UI 后端 CORS 仅允许同源（T2 约束 `Access-Control-Allow-Origin` 回显 Origin + 白名单校验）
- upstream MCP 服务的 CORS 当前为 `*`（`middleware.go:13`），Web UI 代理层应收紧

### 4.4 请求体限制

- upstream 已有 `maxRESTRequestBody = 1 << 20`（1MB，`routes.go:15`），代理层应保持一致
- 图片/视频发布路径涉及本地文件路径，代理层不做路径转义，由 upstream 校验

### 4.5 错误码透传

账号路由错误码（来源：`account/errors.go:10-26`，`routes.go:67-88`）：

| code | HTTP status | 含义 |
|---|---|---|
| `INVALID_ACCOUNT_ID` | 400 | 账号 ID 格式无效 |
| `ACCOUNT_REQUIRED` | 400 | 必须指定账号 |
| `ACCOUNT_NOT_FOUND` | 404 | 账号不存在 |
| `ACCOUNT_LOGIN_REQUIRED` | 401 | 账号未登录（cookie 不存在） |
| `ACCOUNT_BUSY` | 429 | 账号正在执行其他操作 |
| `OPERATION_CANCELED` | 408 | 操作被取消 |
| `ACCOUNT_PAUSED` | 409 | 账号已暂停 |
| `ACCOUNT_RISK_HOLD` | 409 | 账号风控冻结 |
| `ACCOUNT_DISABLED` | 409 | 账号已禁用 |
| `INTERNAL_ERROR` | 500 | 内部错误 |

代理层应原样透传 HTTP status + error code，前端按 code 做差异化提示（如 `ACCOUNT_LOGIN_REQUIRED` → 跳转扫码登录弹窗）。

---

## 5. 多账号选择器交互设计

### 5.1 全局账号选择器（顶部导航栏）

- 页面加载时 `GET /api/web/accounts` 获取列表
- 下拉显示：`display_name (id)` + 状态圆点
- 切换账号时写入 `localStorage["selected_account_id"]`
- 所有页面共享同一选择，通过 `<script src="/static/js/account-selector.js">` 注入
- 不可选状态：`disabled`（灰色禁用）、`risk_hold`（红色禁用 + tooltip）
- 未选择且无默认账号时：所有业务按钮禁用 + 提示"请先选择或创建账号"

### 5.2 账号上下文传递

```javascript
// 通用请求封装（每个页面引用）
async function api(path, options = {}) {
  const accountId = localStorage.getItem('selected_account_id');
  const body = options.body ? JSON.parse(options.body) : {};
  if (accountId && ['POST','PUT','DELETE'].includes(options.method || 'GET')) {
    body.account_id = accountId;
    options.body = JSON.stringify(body);
  }
  const url = accountId && options.method === 'GET' 
    ? `${path}?account_id=${accountId}` : path;
  options.headers = { 'Content-Type': 'application/json', ...(options.headers||{}) };
  const res = await fetch(url, options);
  const json = await res.json();
  if (!json.success) throw new Error(json.error || '请求失败');
  return json.data;
}
```

---

## 6. 通用前端组件

| 组件 | 说明 |
|---|---|
| 顶部导航 | 5 个 Tab（Dashboard / 账号 / 搜索 / 发布 / 详情）+ 全局账号选择器 |
| Toast | 右上角浮动提示，3s 自动消失，支持 success/error/warning |
| Loading | 全局遮罩 + Spinner，用于耗时操作（搜索、发布、获取详情） |
| 错误处理 | 统一拦截 `!success` 响应，按 error code 映射友好提示 |
| 二维码弹窗 | Modal 展示 base64 图片 + 3s 轮询登录状态 + 倒计时 |
| 确认弹窗 | 删除账号、重置登录等高危操作使用二次确认 |

---

## 7. 验收标准

- [ ] `go build` 成功（Web UI 后端）
- [ ] `go vet` 通过
- [ ] `curl http://localhost:18080/` 返回 index.html
- [ ] `curl http://localhost:18080/api/web/health` 返回 200
- [ ] 账号列表页可加载并渲染
- [ ] 搜索页可输入关键词并展示结果卡片
- [ ] 发布页表单校验（标题字数、图片必填）
- [ ] 详情页可展示笔记内容和评论
- [ ] 全局账号选择器在所有页面一致工作
- [ ] 无外部 CDN 依赖（断网可正常加载）
