# 小红书 MCP 功能文档

## 概述

小红书 MCP (Model Context Protocol) 是一个基于浏览器自动化的小红书操作服务，使用 Go + Rod (Chromium) 构建。它将小红书的核心操作封装为 13 个 MCP 工具，同时提供等效的 HTTP REST API，可被 AI 客户端（如 Claude Code、Cherry Studio、AnythingLLM）或 n8n 等自动化平台调用。

**技术栈**: Go 1.24 / Gin / Rod (Chromium) / MCP SDK
**默认端口**: 18060
**协议支持**: MCP 协议 + HTTP REST API

### 功能总览

| 分类 | 工具名称 | 说明 | 类型 |
|------|----------|------|------|
| **登录认证** | `check_login_status` | 检查登录状态 | 只读 |
| | `get_login_qrcode` | 获取登录二维码 | 只读 |
| | `delete_cookies` | 删除 Cookies 重置登录 | 写入 |
| **内容发布** | `publish_content` | 发布图文内容 | 写入 |
| | `publish_with_video` | 发布视频内容 | 写入 |
| **内容浏览** | `list_feeds` | 获取首页推荐列表 | 只读 |
| | `search_feeds` | 按关键词搜索内容 | 只读 |
| | `get_feed_detail` | 获取笔记详情及评论 | 只读 |
| **互动操作** | `like_feed` | 点赞 / 取消点赞 | 写入 |
| | `favorite_feed` | 收藏 / 取消收藏 | 写入 |
| | `post_comment_to_feed` | 发表评论 | 写入 |
| | `reply_comment_in_feed` | 回复评论 | 写入 |
| **用户信息** | `user_profile` | 获取用户主页信息 | 只读 |

---

## 一、登录认证（3 个工具）

登录是使用所有其他功能的前提。小红书同一账号只允许一个网页端登录，登录 MCP 后不要再在其他网页端登录，否则会被踢出。移动端 App 不受影响。

### 1.1 check_login_status - 检查登录状态

检查当前是否已登录小红书。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 无 | - | - | - |

**返回示例**:
```json
{
  "is_logged_in": true,
  "username": "用户昵称"
}
```

### 1.2 get_login_qrcode - 获取登录二维码

生成小红书登录二维码，返回 Base64 编码的图片。扫码后自动完成登录并保存 Cookies。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 无 | - | - | - |

**返回示例**:
```json
{
  "timeout": "120s",
  "is_logged_in": false,
  "img": "data:image/png;base64,..."
}
```

**说明**:
- 二维码有效期约 120 秒
- 扫码成功后自动保存 Cookies 到 `~/.xhs-mcp/cookies.json`
- 后续启动会自动加载已保存的 Cookies

### 1.3 delete_cookies - 删除 Cookies

删除已保存的 Cookies 文件，重置登录状态。删除后需要重新扫码登录。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 无 | - | - | - |

**返回示例**:
```json
{
  "cookie_path": "~/.xhs-mcp/cookies.json",
  "message": "cookies deleted"
}
```

---

## 二、内容发布（2 个工具）

### 2.1 publish_content - 发布图文内容

发布图文笔记到小红书，支持标题、正文、图片、话题标签、定时发布、可见范围等。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | 是 | 标题，最多 20 个中文字或英文单词 |
| `content` | string | 是 | 正文内容，最多 1000 字，不要包含 `#` 开头的标签 |
| `images` | string[] | 是 | 图片列表（至少 1 张），支持 HTTP(S) 链接或本地绝对路径 |
| `tags` | string[] | 否 | 话题标签列表，最多 10 个，超出自动截断 |
| `schedule_at` | string | 否 | 定时发布时间，ISO 8601 格式，如 `2024-01-20T10:30:00+08:00`，支持 1 小时到 14 天内 |
| `is_original` | bool | 否 | 是否声明原创，默认 false |
| `visibility` | string | 否 | 可见范围：`公开可见`（默认）/ `仅自己可见` / `仅互关好友可见` |

**返回示例**:
```json
{
  "title": "笔记标题",
  "content": "笔记内容",
  "images": 3,
  "status": "published",
  "post_id": "xxx"
}
```

**注意事项**:
- 图片推荐使用本地绝对路径，稳定性更好、速度更快
- 所有话题标签用 `tags` 参数传递，不要写在 `content` 里
- 每天发帖建议不超过 50 篇
- 不要纯搬运内容，属于平台重点打击对象

### 2.2 publish_with_video - 发布视频内容

发布视频笔记到小红书，仅支持本地视频文件。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | 是 | 标题，最多 20 个中文字 |
| `content` | string | 是 | 正文描述 |
| `video` | string | 是 | 本地视频绝对路径，如 `/Users/user/video.mp4` |
| `tags` | string[] | 否 | 话题标签列表 |
| `schedule_at` | string | 否 | 定时发布时间，ISO 8601 格式 |
| `visibility` | string | 否 | 可见范围，同上 |

**返回示例**:
```json
{
  "title": "视频标题",
  "content": "视频描述",
  "video": "/path/to/video.mp4",
  "status": "published",
  "post_id": "xxx"
}
```

**注意事项**:
- 仅支持本地视频文件，**不支持 HTTP 链接**
- 视频处理需要时间，请耐心等待
- 建议视频文件大小不超过 1GB

---

## 三、内容浏览（3 个工具）

### 3.1 list_feeds - 获取首页推荐列表

获取小红书首页推荐的笔记列表。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 无 | - | - | - |

**返回示例**:
```json
{
  "feeds": [
    {
      "id": "笔记ID",
      "xsecToken": "访问令牌",
      "noteCard": {
        "displayTitle": "标题",
        "user": { "userId": "...", "nickname": "..." },
        "interactInfo": { "likedCount": "100", "collectedCount": "50" }
      }
    }
  ],
  "count": 20
}
```

**说明**:
- 返回的 `id` 和 `xsecToken` 是调用其他工具（如获取详情、点赞等）的必需参数

### 3.2 search_feeds - 搜索内容

根据关键词搜索小红书内容，支持多维度筛选。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 是 | 搜索关键词 |
| `filters` | object | 否 | 筛选条件（见下表） |

**筛选条件 (`filters`)**:

| 字段 | 可选值 | 默认 |
|------|--------|------|
| `sort_by` | `综合` / `最新` / `最多点赞` / `最多评论` / `最多收藏` | `综合` |
| `note_type` | `不限` / `视频` / `图文` | `不限` |
| `publish_time` | `不限` / `一天内` / `一周内` / `半年内` | `不限` |
| `search_scope` | `不限` / `已看过` / `未看过` / `已关注` | `不限` |
| `location` | `不限` / `同城` / `附近` | `不限` |

**返回**: 与 `list_feeds` 相同的 Feed 列表结构。

### 3.3 get_feed_detail - 获取笔记详情

获取指定笔记的完整详情，包括内容、作者信息、互动数据和评论列表。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | 是 | 笔记 ID，从 Feed 列表获取 |
| `xsec_token` | string | 是 | 访问令牌，从 Feed 列表的 `xsecToken` 获取 |
| `load_all_comments` | bool | 否 | 是否加载全部评论，默认 `false`（仅返回前 10 条） |
| `limit` | int | 否 | 最多加载的一级评论数量，默认 20（仅 `load_all_comments=true` 时生效） |
| `click_more_replies` | bool | 否 | 是否展开二级回复，默认 `false`（仅 `load_all_comments=true` 时生效） |
| `reply_limit` | int | 否 | 跳过回复数超过此值的评论，默认 10（仅 `click_more_replies=true` 时生效） |
| `scroll_speed` | string | 否 | 滚动速度：`slow` / `normal` / `fast`（仅 `load_all_comments=true` 时生效） |

**返回示例**:
```json
{
  "note": {
    "title": "笔记标题",
    "desc": "笔记描述",
    "imageList": [...],
    "user": { "nickname": "作者", "userId": "..." },
    "interactInfo": {
      "likedCount": "1000",
      "collectedCount": "500",
      "commentCount": "200",
      "shareCount": "100"
    }
  },
  "comments": {
    "list": [
      {
        "id": "评论ID",
        "content": "评论内容",
        "likeCount": "10",
        "userInfo": { "nickname": "评论者" },
        "subComments": [...]
      }
    ],
    "cursor": "...",
    "hasMore": true
  }
}
```

**说明**:
- `feed_id` 和 `xsec_token` 两个参数缺一不可，必须从 Feed 列表或搜索结果获取
- 评论加载采用滚动翻页机制，大量评论时可通过 `scroll_speed` 控制速度
- `reply_limit` 可以跳过回复数过多的热门评论，避免加载时间过长

---

## 四、互动操作（4 个工具）

### 4.1 like_feed - 点赞 / 取消点赞

为指定笔记点赞或取消点赞。如果已点赞则自动跳过点赞操作，如果未点赞则跳过取消点赞操作。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | 是 | 笔记 ID |
| `xsec_token` | string | 是 | 访问令牌 |
| `unlike` | bool | 否 | `true` 取消点赞，`false` 或不设置则点赞 |

### 4.2 favorite_feed - 收藏 / 取消收藏

收藏指定笔记或取消收藏。逻辑同点赞，已收藏则跳过收藏，未收藏则跳过取消收藏。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | 是 | 笔记 ID |
| `xsec_token` | string | 是 | 访问令牌 |
| `unfavorite` | bool | 否 | `true` 取消收藏，`false` 或不设置则收藏 |

### 4.3 post_comment_to_feed - 发表评论

在指定笔记下发表评论。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | 是 | 笔记 ID |
| `xsec_token` | string | 是 | 访问令牌 |
| `content` | string | 是 | 评论内容 |

### 4.4 reply_comment_in_feed - 回复评论

回复指定笔记下的某条评论。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | 是 | 笔记 ID |
| `xsec_token` | string | 是 | 访问令牌 |
| `content` | string | 是 | 回复内容 |
| `comment_id` | string | 否 | 目标评论 ID，从评论列表获取 |
| `user_id` | string | 否 | 目标评论用户 ID，从评论列表获取 |

**说明**: `comment_id` 和 `user_id` 至少提供一个以定位目标评论。

---

## 五、用户信息（1 个工具）

### 5.1 user_profile - 获取用户主页

获取指定用户的主页信息，包括基本资料、互动统计和笔记列表。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 是 | 用户 ID，从 Feed 列表获取 |
| `xsec_token` | string | 是 | 访问令牌 |

**返回示例**:
```json
{
  "userBasicInfo": {
    "nickname": "用户昵称",
    "desc": "个人简介",
    "redId": "小红书号",
    "gender": 1,
    "ipLocation": "广东"
  },
  "interactions": [
    { "type": "follows", "count": "100" },
    { "type": "fans", "count": "5000" },
    { "type": "interaction", "count": "10000" }
  ],
  "feeds": [...]
}
```

**返回字段说明**:
- `userBasicInfo`: 昵称、简介、小红书号、性别、IP 属地、头像等
- `interactions`: 关注数、粉丝数、获赞与收藏总量
- `feeds`: 该用户公开发布的笔记列表

---

## 六、部署与配置

### 启动参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-headless` | bool | `true` | 无头浏览器模式，设为 `false` 可查看浏览器界面 |
| `-bin` | string | 自动下载 | 自定义 Chrome/Chromium 路径 |
| `-port` | string | `:18060` | HTTP 服务监听端口 |

### 环境变量

| 变量 | 说明 |
|------|------|
| `ROD_BROWSER_BIN` | 自定义 Chrome/Chromium 二进制文件路径 |
| `XHS_PROXY` | 代理地址，支持 HTTP/HTTPS/SOCKS5 |
| `COOKIES_PATH` | 自定义 Cookies 文件路径 |

### 部署方式

1. **预编译二进制** - 支持 macOS (Intel/ARM)、Windows、Linux
2. **源码编译** - 需要 Go 1.24+
3. **Docker** - Docker Hub 镜像 `xpzouying/xiaohongshu-mcp`
4. **Docker Compose** - 完整配置方案

### 文件位置

| 文件 | 路径 |
|------|------|
| Cookies | `~/.xhs-mcp/cookies.json`（可通过 `COOKIES_PATH` 自定义） |
| 浏览器 | Rod 自动管理 |
| Docker 数据 | `/app/data`（Cookies）、`/app/images`（图片） |

---

## 七、重要注意事项

1. **账号安全**: 同一账号只允许一个网页端登录，MCP 登录后勿在其他网页端登录
2. **发帖频率**: 每天建议不超过 50 篇
3. **内容规范**: 禁止纯搬运/引流，属于平台重点打击对象
4. **实名认证**: 新号可能触发实名认证提醒，认证后恢复正常
5. **曝光优化**: 检查内容中是否有违禁词，添加合适的 Tags 可增加流量
6. **图文优先**: 从推荐角度看，图文的流量通常优于视频和纯文字
