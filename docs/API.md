# 小红书 MCP HTTP API 文档

## 概述

该项目提供了小红书 MCP (Model Context Protocol) 服务的 HTTP API 接口，同时支持 MCP 协议和标准的 HTTP REST API。本文档描述了 HTTP API 的使用方法。

**Base URL**: `http://localhost:8080`

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
            "avatar": "https://example.com/avatar.jpg"
          },
          "interactInfo": {
            "likedCount": "100",
            "commentCount": "50"
          },
          "cover": {
            "url": "https://example.com/cover.jpg"
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

#### 4.2 搜索 Feeds

根据关键词搜索 Feeds。

**请求**
```
GET /api/v1/feeds/search?keyword=搜索关键词
```

**查询参数:**
- `keyword` (string, required): 搜索关键词

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
          "displayTitle": "相关笔记标题",
          "user": {
            "userId": "user_id_1",
            "nickname": "用户昵称"
          },
          "interactInfo": {
            "likedCount": "80",
            "commentCount": "35"
          }
        },
        "index": 0
      }
    ],
    "count": 5
  },
  "message": "搜索Feeds成功"
}
```

#### 4.3 获取 Feed 详情

获取指定 Feed 的详细信息。

**请求**
```
POST /api/v1/feeds/detail
Content-Type: application/json
```

**请求体**
```json
{
  "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
  "xsec_token": "security_token_here"
}
```

**请求参数说明:**
- `feed_id` (string, required): Feed ID
- `xsec_token` (string, required): 安全令牌

**响应**
```json
{
  "success": true,
  "data": {
    "feed_id": "64f1a2b3c4d5e6f7a8b9c0d1",
    "data": {
      "note": {
        "noteId": "64f1a2b3c4d5e6f7a8b9c0d1",
        "title": "笔记标题",
        "desc": "笔记详细内容描述",
        "user": {
          "userId": "user_id_123",
          "nickname": "作者昵称"
        },
        "interactInfo": {
          "likedCount": "100",
          "commentCount": "50"
        },
        "imageList": [
          {
            "urlDefault": "https://example.com/image1_default.jpg"
          }
        ]
      },
      "comments": {
        "list": [
          {
            "id": "comment_id_1",
            "content": "评论内容",
            "userInfo": {
              "nickname": "评论者昵称"
            }
          }
        ],
        "hasMore": true
      }
    }
  },
  "message": "获取Feed详情成功"
}
```

---

### 5. 用户信息

获取用户主页信息。

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
        "redId": "xiaohongshu_id"
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
        }
      ],
      "feeds": [
        {
          "id": "feed_id_1",
          "noteCard": {
            "displayTitle": "用户的笔记标题"
          }
        }
      ]
    }
  },
  "message": "获取用户主页成功"
}
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