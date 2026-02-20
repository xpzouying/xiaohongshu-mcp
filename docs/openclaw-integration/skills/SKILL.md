---
name: xiaohongshu-mcp
description: 小红书完整工具包 — 通过 exec + mcporter 调用 xiaohongshu-mcp 的 13 个 MCP 工具，支持浏览、搜索、点赞、评论、发布等全部操作
homepage: https://github.com/BodaFu/xiaohongshu-mcp/tree/main/docs/openclaw-integration
emoji: 📕
version: 3.0.0

capabilities:
  - search
  - read
  - publish
  - comment
  - reply
  - like
  - favorite

requirements:
  bins: [mcporter]
  network: true
  services: ["http://localhost:18060/mcp"]

---

# 小红书技能 v3

通过 `exec` 工具执行 `mcporter call xiaohongshu.<工具名>` 命令来操作小红书。

**前提**：xiaohongshu-mcp 服务运行在 `localhost:18060`，mcporter 已配置 `xiaohongshu` 服务器。

## 调用方式

**所有小红书操作都通过 exec 工具执行 mcporter 命令**，不是直接函数调用。

```
exec(command="mcporter call xiaohongshu.<工具名> 参数1=值1 参数2=值2", timeout=180)
```

参数值包含空格/中文时用引号包裹：
```
exec(command="mcporter call xiaohongshu.post_comment_to_feed feed_id=abc xsec_token=xyz content=\"评论内容\"", timeout=180)
```

## 完整工具列表

### 浏览与搜索

| mcporter 工具名 | 说明 | 参数 |
|----------------|------|------|
| `list_feeds` | 获取首页推荐（返回约30-35条，无 limit 参数） | - |
| `search_feeds` | 搜索笔记 | keyword(必需), sort_by, note_type, publish_time, search_scope, location |
| `get_feed_detail` | 笔记详情+评论 | feed_id(必需), xsec_token(必需), load_all_comments, limit, click_more_replies, reply_limit, scroll_speed |
| `user_profile` | 用户主页 | user_id(必需), xsec_token(必需) |

### 互动操作

| mcporter 工具名 | 说明 | 参数 |
|----------------|------|------|
| `like_feed` | 点赞 | feed_id(必需), xsec_token(必需), unlike=true取消 |
| `favorite_feed` | 收藏 | feed_id(必需), xsec_token(必需), unfavorite=true取消 |
| `post_comment_to_feed` | 顶级评论 | feed_id(必需), xsec_token(必需), content(必需) |
| `reply_comment_in_feed` | 回复评论（楼中楼） | feed_id(必需), xsec_token(必需), comment_id(必需), content(必需), user_id(可选) |

### 发布操作

| mcporter 工具名 | 说明 | 参数 |
|----------------|------|------|
| `publish_content` | 发布图文 | title(必需), content(必需), images(必需,数组), tags(可选), schedule_at(可选) |
| `publish_with_video` | 发布视频 | title(必需), content(必需), video(必需,本地路径), tags(可选), schedule_at(可选) |

### 账号管理

| mcporter 工具名 | 说明 |
|----------------|------|
| `check_login_status` | 检查登录状态 |
| `get_login_qrcode` | 获取登录二维码 |
| `delete_cookies` | 重置登录 |

## 评论系统

- `get_feed_detail` 返回嵌套评论：每条评论有 `id`、`content`、`userInfo`、`subComments[]`
- 获取全部评论：`load_all_comments=true`
- 展开子评论：`click_more_replies=true`
- 回复评论：用 `reply_comment_in_feed`，传入 `comment_id`

## 使用示例

```bash
# 获取推荐 Feed
mcporter call xiaohongshu.list_feeds

# 搜索笔记
mcporter call xiaohongshu.search_feeds keyword=AI

# 查看笔记详情（含评论）
mcporter call xiaohongshu.get_feed_detail feed_id=xxx xsec_token=yyy

# 点赞
mcporter call xiaohongshu.like_feed feed_id=xxx xsec_token=yyy

# 发表评论
mcporter call xiaohongshu.post_comment_to_feed feed_id=xxx xsec_token=yyy content="很有启发！"

# 发布图文笔记
mcporter call xiaohongshu.publish_content title="我的标题" content="正文内容 #AI #技术" images='["/path/to/image.jpg"]'
```

## 故障排除

- **mcporter 找不到 server**：确认 `~/.mcporter/mcporter.json` 中配置了 `xiaohongshu` 服务器
- **MCP 服务不可用**：手动启动 `cd /path/to/xiaohongshu-mcp && ./xiaohongshu-mcp -port=:18060 -headless=true`
- **未登录**：执行 `mcporter call xiaohongshu.get_login_qrcode` 或 `cd /path/to/xiaohongshu-mcp && go run cmd/login/main.go`
- **操作超时**：确保 exec 调用设置了 `timeout=180`
