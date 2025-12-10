# 小红书 MCP HTTP API 文档

## 概述

该项目提供了小红书 MCP (Model Context Protocol) 服务的 HTTP API 接口，同时支持 MCP 协议和标准的 HTTP REST API。本文档描述了 HTTP API 的使用方法。

**Base URL**: `http://localhost:18060`

**注意**: 以下响应示例仅展示主要字段结构，完整的字段信息请通过实际API调用查看。

## 通用响应格式

所有 API 响应都使用统一的 JSON 格式：

### 成功响应
```json
{
  "success": true,
  "data": {},
  "message": "操作成功消息"
}
```

### 错误响应
```json
{
  "error": "错误消息",
  "code": "ERROR_CODE",
  "details": "详细错误信息"
}
```

## API 端点一览

| 方法 | 端点 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/v1/login/status` | 检查登录状态 |
| GET | `/api/v1/login/qrcode` | 获取登录二维码 |
| DELETE | `/api/v1/login/cookies` | 删除 Cookies（重置登录） |
| POST | `/api/v1/publish` | 发布图文内容 |
| POST | `/api/v1/publish_video` | 发布视频内容 |
| GET | `/api/v1/feeds/list` | 获取 Feeds 列表 |
| GET/POST | `/api/v1/feeds/search` | 搜索 Feeds |
| POST | `/api/v1/feeds/detail` | 获取 Feed 详情 |
| POST | `/api/v1/user/profile` | 获取用户主页信息 |
| GET | `/api/v1/user/me` | 获取当前登录用户信息 |
| POST | `/api/v1/feeds/comment` | 发表评论 |
| POST | `/api/v1/feeds/comment/reply` | 回复评论 |

---

## API 端点

### 1. 健康检查

检查服务状态。

**请求**
```
GET /health
```

**响应**
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "service": "xiaohongshu-mcp",
    "account": "ai-report",
    "timestamp": "now"
  },
  "message": "服务正常"
}
```

---

### 2. 登录管理

#### 2.1 检查登录状态

检查当前用户的登录状态。

**请求**
```
GET /api/v1/login/status
```

**响应**
```json
{
  "success": true,
  "data": {
    "is_logged_in": true,
    "username": "用户名"
  },
  "message": "检查登录状态成功"
}
```

#### 2.2 获取登录二维码

获取登录二维码，用于用户扫码登录。

**请求**
```
GET /api/v1/login/qrcode
```

**响应**
```json
{
  "success": true,
  "data": {
    "timeout": "300",
    "is_logged_in": false,
    "img": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..."
  },
  "message": "获取登录二维码成功"
}
```

**响应字段说明:**
- `timeout`: 二维码过期时间（秒）
- `is_logged_in`: 当前是否已登录
- `img`: Base64 编码的二维码图片

#### 2.3 删除 Cookies（重置登录状态）

删除本地存储的 cookies 文件，重置登录状态。

**请求**
```
DELETE /api/v1/login/cookies
```

**响应**
```json
{
  "success": true,
  "data": {
    "cookie_path": "/path/to/cookies.json",
    "message": "Cookies 已成功删除，登录状态已重置。下次操作时需要重新登录。"
  },
  "message": "删除 cookies 成功"
}
```

---

### 3. 内容发布

#### 3.1 发布图文内容

发布图文笔记内容到小红书。

**请求**
```
POST /api/v1/publish
Content-Type: application/json
```

**请求体**
```json
{
  "title": "笔记标题",
  "content": "笔记内容",
  "images": [
    "http://example.com/image1.jpg",
    "http://example.com/image2.jpg"
  ],
  "tags": ["标签1", "标签2"]
}
```

**请求参数说明:**
- `title` (string, required): 笔记标题
- `content` (string, required): 笔记内容
- `images` (array, required): 图片URL数组，至少包含一张图片
- `tags` (array, optional): 标签数组

**响应**
```json
{
  "success": true,
  "data": {
    "title": "笔记标题",
    "content": "笔记内容",
    "images": 2,
    "status": "published",
    "post_id": "64f1a2b3c4d5e6f7a8b9c0d1"
  },
  "message": "发布成功"
}
```

#### 3.2 发布视频内容

发布视频内容到小红书（仅支持本地视频文件）。

**请求**
```
POST /api/v1/publish_video
Content-Type: application/json
```

**请求体**
```json
{
  "title": "视频标题",
  "content": "视频内容描述",
  "video": "/Users/username/Videos/video.mp4",
  "tags": ["标签1", "标签2"]
}
```

**请求参数说明:**
- `title` (string, required): 视频标题
- `content` (string, required): 视频内容描述
- `video` (string, required): 本地视频文件绝对路径
- `tags` (array, optional): 标签数组

**响应**
```json
{
  "success": true,
  "data": {
    "title": "视频标题",
    "content": "视频内容描述",
    "video": "/Users/username/Videos/video.mp4",
    "status": "发布完成",
    "post_id": "64f1a2b3c4d5e6f7a8b9c0d1"
  },
  "message": "视频发布成功"
}
```

**注意事项:**
- 仅支持本地视频文件路径，不支持 HTTP 链接
- 视频处理时间较长，请耐心等待
- 建议视频文件大小不超过 1GB

---

### 4. Feed 管理

#### 4.1 获取 Feeds 列表

获取用户的 Feeds 列表。

**请求**
```
GET /api/v1/feeds/list
```

**响应**
```json
{
  "success": true,
  "data": {
    "feeds": [
      {
        "xsecToken": "security_token_value",
        "id": "feed_id_1",
        "modelType": "note",
        "noteCard": {
          "type": "normal",
          "displayTitle": "笔记标题",
          "user": {
            "userId": "user_id_1",
            "nickname": "用户昵称",
            "nickName": "用户昵称",
            "avatar": "https://example.com/avatar.jpg"
          },
          "interactInfo": {
            "liked": false,
            "likedCount": "100",
            "collected": false,
            "collectedCount": "50",
            "commentCount": "30",
            "sharedCount": "10"
          },
          "cover": {
            "width": 1080,
            "height": 1440,
            "url": "https://example.com/cover.jpg",
            "urlDefault": "https://example.com/cover_default.jpg",
            "urlPre": "https://example.com/cover_pre.jpg",
            "fileId": "file_id",
            "infoList": [
              {
                "imageScene": "WB_DFT",
                "url": "https://example.com/image.jpg"
              }
            ]
          },
          "video": {
            "capa": {
              "duration": 60
            }
          }
        },
        "index": 0
      }
    ],
    "count": 10
  },
  "message": "获取Feeds列表成功"
}
```

**响应字段说明:**
- `xsecToken`: 安全令牌，调用详情等接口时需要
- `id`: Feed ID
- `modelType`: 模型类型，通常为 "note"
- `noteCard.type`: 笔记类型
- `noteCard.video`: 视频信息（仅视频笔记有此字段）
  - `capa.duration`: 视频时长（秒）
- `noteCard.interactInfo`: 互动信息
  - `liked`: 当前用户是否已点赞
  - `collected`: 当前用户是否已收藏
  - `likedCount`: 点赞数
  - `collectedCount`: 收藏数
  - `commentCount`: 评论数
  - `sharedCount`: 分享数
```

#### 4.2 搜索 Feeds

根据关键词搜索 Feeds，支持 GET 和 POST 两种请求方式。

**请求方式一：GET**
```
GET /api/v1/feeds/search?keyword=搜索关键词
```

**查询参数:**
- `keyword` (string, required): 搜索关键词

**请求方式二：POST（支持高级筛选）**
```
POST /api/v1/feeds/search
Content-Type: application/json
```

**请求体**
```json
{
  "keyword": "搜索关键词",
  "filters": {
    "sort_by": "综合",
    "note_type": "不限",
    "publish_time": "不限",
    "search_scope": "不限",
    "location": "不限"
  }
}
```

**筛选参数说明:**
- `sort_by` (string, optional): 排序依据，可选值：`综合`(默认) | `最新` | `最多点赞` | `最多评论` | `最多收藏`
- `note_type` (string, optional): 笔记类型，可选值：`不限`(默认) | `视频` | `图文`
- `publish_time` (string, optional): 发布时间，可选值：`不限`(默认) | `一天内` | `一周内` | `半年内`
- `search_scope` (string, optional): 搜索范围，可选值：`不限`(默认) | `已看过` | `未看过` | `已关注`
- `location` (string, optional): 位置距离，可选值：`不限`(默认) | `同城` | `附近`

**响应**
```json
{
  "success": true,
  "data": {
    "feeds": [
      {
        "xsecToken": "security_token_value",
        "id": "feed_id_1",
        "modelType": "note",
        "noteCard": {
          "type": "normal",
          "displayTitle": "相关笔记标题",
          "user": {
            "userId": "user_id_1",
            "nickname": "用户昵称",
            "avatar": "https://example.com/avatar.jpg"
          },
          "interactInfo": {
            "liked": false,
            "likedCount": "80",
            "collected": false,
            "collectedCount": "40",
            "commentCount": "35",
            "sharedCount": "15"
          },
          "cover": {
            "width": 1080,
            "height": 1440,
            "url": "https://example.com/cover.jpg",
            "urlDefault": "https://example.com/cover_default.jpg"
          },
          "video": null
        },
        "index": 0
      }
    ],
    "count": 5
  },
  "message": "搜索Feeds成功"
}
```

**响应字段说明:**
- 响应结构与"获取 Feeds 列表"接口相同
- `video`: 视频笔记时有此字段，图文笔记为 null
```

#### 4.3 获取 Feed 详情

获取指定 Feed 的详细信息，支持加载全部评论和自定义评论加载配置。

**请求**
```
POST /api/v1/feeds/detail
Content-Type: application/json
```

**请求体**
```json
{
  "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
  "xsec_token": "security_token_here",
  "load_all_comments": false,
  "comment_config": {
    "click_more_replies": true,
    "max_replies_threshold": 50,
    "max_comment_items": 100,
    "scroll_speed": "normal"
  }
}
```

**请求参数说明:**
- `feed_id` (string, required): Feed ID
- `xsec_token` (string, required): 安全令牌
- `load_all_comments` (boolean, optional): 是否加载全部评论，默认 false
- `comment_config` (object, optional): 评论加载配置
  - `click_more_replies` (boolean): 是否点击"更多回复"按钮
  - `max_replies_threshold` (int): 回复数量阈值，超过这个数量的"更多"按钮将被跳过（0表示不跳过任何）
  - `max_comment_items` (int): 最大加载评论数（.parent-comment 数量），0表示加载所有
  - `scroll_speed` (string): 滚动速度等级，可选值：`slow`(慢速) | `normal`(正常) | `fast`(快速)

**响应**
```json
{
  "success": true,
  "data": {
    "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
    "data": {
      "note": {
        "noteId": "64f1a2b3c4d5e6f7a8b9c0d1",
        "xsecToken": "security_token_value",
        "title": "笔记标题",
        "desc": "笔记详细内容描述",
        "type": "normal",
        "time": 1702195200000,
        "ipLocation": "浙江",
        "user": {
          "userId": "user_id_123",
          "nickname": "作者昵称",
          "nickName": "作者昵称",
          "avatar": "https://example.com/avatar.jpg"
        },
        "interactInfo": {
          "liked": false,
          "likedCount": "100",
          "collected": false,
          "collectedCount": "80",
          "commentCount": "50",
          "sharedCount": "20"
        },
        "imageList": [
          {
            "width": 1080,
            "height": 1440,
            "urlDefault": "https://example.com/image1_default.jpg",
            "urlPre": "https://example.com/image1_pre.jpg",
            "livePhoto": false
          }
        ]
      },
      "comments": {
        "list": [
          {
            "id": "comment_id_1",
            "noteId": "64f1a2b3c4d5e6f7a8b9c0d1",
            "content": "评论内容",
            "likeCount": "10",
            "createTime": 1702195200000,
            "ipLocation": "北京",
            "liked": false,
            "userInfo": {
              "userId": "commenter_id",
              "nickname": "评论者昵称",
              "avatar": "https://example.com/commenter_avatar.jpg"
            },
            "subCommentCount": "5",
            "subComments": [
              {
                "id": "sub_comment_id_1",
                "content": "子评论内容",
                "createTime": 1702195300000,
                "userInfo": {
                  "nickname": "回复者昵称"
                }
              }
            ],
            "showTags": ["热评"]
          }
        ],
        "cursor": "next_cursor_value",
        "hasMore": true
      }
    }
  },
  "message": "获取Feed详情成功"
}
```

**响应字段说明:**
- `note.time`: 笔记发布时间戳（毫秒）
- `note.ipLocation`: 发布者 IP 归属地
- `note.type`: 笔记类型
- `note.interactInfo`: 互动信息
  - `liked`: 当前用户是否已点赞
  - `collected`: 当前用户是否已收藏
- `note.imageList[].livePhoto`: 是否为 Live Photo
- `comments.list[].createTime`: 评论发布时间戳（毫秒）
- `comments.list[].ipLocation`: 评论者 IP 归属地
- `comments.list[].likeCount`: 评论点赞数
- `comments.list[].liked`: 当前用户是否已点赞该评论
- `comments.list[].subCommentCount`: 子评论数量
- `comments.list[].subComments`: 子评论列表
- `comments.list[].showTags`: 显示标签（如 "热评"）
- `comments.cursor`: 分页游标
- `comments.hasMore`: 是否有更多评论
```

---

### 5. 用户信息

#### 5.1 获取用户主页信息

获取指定用户的主页信息，包括基本信息、互动数据和发布的笔记列表。

**请求**
```
POST /api/v1/user/profile
Content-Type: application/json
```

**请求体**
```json
{
  "user_id": "64f1a2b3c4d5e6f7a8b9c0d1",
  "xsec_token": "security_token_here"
}
```

**请求参数说明:**
- `user_id` (string, required): 用户ID
- `xsec_token` (string, required): 安全令牌

**响应**
```json
{
  "success": true,
  "data": {
    "data": {
      "userBasicInfo": {
        "nickname": "用户昵称",
        "desc": "用户个人描述",
        "redId": "xiaohongshu_id",
        "gender": 1,
        "ipLocation": "浙江",
        "images": "https://example.com/avatar.jpg",
        "imageb": "https://example.com/background.jpg"
      },
      "interactions": [
        {
          "type": "follows",
          "name": "关注",
          "count": "1000"
        },
        {
          "type": "fans",
          "name": "粉丝",
          "count": "5000"
        },
        {
          "type": "interaction",
          "name": "获赞与收藏",
          "count": "10000"
        }
      ],
      "feeds": [
        {
          "xsecToken": "security_token_value",
          "id": "feed_id_1",
          "modelType": "note",
          "noteCard": {
            "displayTitle": "用户的笔记标题",
            "interactInfo": {
              "likedCount": "100",
              "collectedCount": "50"
            }
          },
          "index": 0
        }
      ]
    }
  },
  "message": "获取用户主页成功"
}
```

**响应字段说明:**
- `userBasicInfo.gender`: 性别（1: 男, 2: 女, 0: 未知）
- `userBasicInfo.ipLocation`: IP 归属地
- `userBasicInfo.images`: 头像图片 URL
- `userBasicInfo.imageb`: 背景图片 URL
- `userBasicInfo.redId`: 小红书号
- `interactions`: 互动数据数组
  - `type`: 类型（follows: 关注, fans: 粉丝, interaction: 获赞与收藏）
  - `name`: 显示名称
  - `count`: 数量
- `feeds`: 用户发布的笔记列表（结构同 Feed 列表）
```

#### 5.2 获取当前登录用户信息

获取当前登录用户的个人信息（无需传入 user_id），通过侧边栏导航到个人主页获取。

**请求**
```
GET /api/v1/user/me
```

**响应**
```json
{
  "success": true,
  "data": {
    "data": {
      "userBasicInfo": {
        "nickname": "当前用户昵称",
        "desc": "个人描述",
        "redId": "xiaohongshu_id",
        "gender": 1,
        "ipLocation": "浙江",
        "images": "https://example.com/my_avatar.jpg",
        "imageb": "https://example.com/my_background.jpg"
      },
      "interactions": [
        {
          "type": "follows",
          "name": "关注",
          "count": "100"
        },
        {
          "type": "fans",
          "name": "粉丝",
          "count": "500"
        },
        {
          "type": "interaction",
          "name": "获赞与收藏",
          "count": "2000"
        }
      ],
      "feeds": [
        {
          "xsecToken": "security_token_value",
          "id": "feed_id_1",
          "modelType": "note",
          "noteCard": {
            "displayTitle": "我的笔记标题",
            "interactInfo": {
              "likedCount": "50",
              "collectedCount": "30"
            }
          },
          "index": 0
        }
      ]
    }
  },
  "message": "获取我的主页成功"
}
```

**响应字段说明:**
- 响应结构与"获取用户主页信息"接口相同
- 此接口无需 `user_id` 和 `xsec_token` 参数，自动获取当前登录用户信息
```

---

### 6. 评论管理

#### 6.1 发表评论

对指定 Feed 发表评论。

**请求**
```
POST /api/v1/feeds/comment
Content-Type: application/json
```

**请求体**
```json
{
  "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
  "xsec_token": "security_token_here",
  "content": "评论内容"
}
```

**请求参数说明:**
- `feed_id` (string, required): Feed ID
- `xsec_token` (string, required): 安全令牌
- `content` (string, required): 评论内容

**响应**
```json
{
  "success": true,
  "data": {
    "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
    "success": true,
    "message": "评论发表成功"
  },
  "message": "评论发表成功"
}
```

#### 6.2 回复评论

回复指定评论。

**请求**
```
POST /api/v1/feeds/comment/reply
Content-Type: application/json
```

**请求体**
```json
{
  "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
  "xsec_token": "security_token_here",
  "comment_id": "comment_id_to_reply",
  "user_id": "target_user_id",
  "content": "回复内容"
}
```

**请求参数说明:**
- `feed_id` (string, required): Feed ID
- `xsec_token` (string, required): 安全令牌
- `comment_id` (string, required*): 要回复的评论 ID（与 user_id 二选一必填）
- `user_id` (string, required*): 要回复的用户 ID（与 comment_id 二选一必填）
- `content` (string, required): 回复内容

**响应**
```json
{
  "success": true,
  "data": {
    "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
    "target_comment_id": "comment_id_to_reply",
    "target_user_id": "target_user_id",
    "success": true,
    "message": "回复评论成功"
  },
  "message": "回复评论成功"
}
```

---

## 错误代码

所有 API 在发生错误时会返回统一格式的错误响应。以下是可能出现的错误代码：

| 错误代码 | HTTP 状态码 | 描述 |
|----------|-------------|------|
| `INVALID_REQUEST` | 400 | 请求参数错误或格式不正确 |
| `MISSING_KEYWORD` | 400 | 搜索时缺少关键词参数 |
| `STATUS_CHECK_FAILED` | 500 | 检查登录状态失败 |
| `DELETE_COOKIES_FAILED` | 500 | 删除 Cookies 失败 |
| `PUBLISH_FAILED` | 500 | 发布图文内容失败 |
| `PUBLISH_VIDEO_FAILED` | 500 | 发布视频内容失败 |
| `LIST_FEEDS_FAILED` | 500 | 获取 Feeds 列表失败 |
| `SEARCH_FEEDS_FAILED` | 500 | 搜索 Feeds 失败 |
| `GET_FEED_DETAIL_FAILED` | 500 | 获取 Feed 详情失败 |
| `GET_USER_PROFILE_FAILED` | 500 | 获取用户主页信息失败 |
| `GET_MY_PROFILE_FAILED` | 500 | 获取当前用户信息失败 |
| `POST_COMMENT_FAILED` | 500 | 发表评论失败 |
| `REPLY_COMMENT_FAILED` | 500 | 回复评论失败 |
| `INTERNAL_ERROR` | 500 | 服务器内部错误 |

---

## 注意事项

1. **认证**: 部分 API 需要有效的登录状态，建议先调用登录状态检查接口确认登录。

2. **安全令牌**: `xsec_token` 是小红书的安全令牌，在调用需要该参数的接口时必须提供。

3. **图片上传**: 发布接口中的 `images` 参数需要提供可访问的图片URL。

4. **错误处理**: 所有接口在出错时都会返回统一格式的错误响应，请根据 `code` 字段进行相应的错误处理。

5. **日志记录**: 所有API调用都会被记录到服务日志中，包括请求方法、路径和状态码。

6. **跨域支持**: API 支持跨域请求 (CORS)。

## MCP 协议支持

除了上述HTTP API，本服务同时支持 MCP (Model Context Protocol) 协议：

- **MCP 端点**: `/mcp` 和 `/mcp/*path`
- **协议类型**: 支持 JSON 响应格式的 Streamable HTTP
- **用途**: 可以通过MCP客户端调用相同的功能

更多MCP协议相关信息请参考 [Model Context Protocol 官方文档](https://modelcontextprotocol.io/)。