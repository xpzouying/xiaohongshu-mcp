## 小红书工具

> 将此部分追加到你的 `~/.openclaw/workspace/TOOLS.md` 文件末尾。

**重要：小红书工具通过 `exec` 工具执行 `mcporter call` 命令来调用，不是直接函数调用。**

调用格式（**注意设置 timeout 为 180 秒**，MCP 操作涉及浏览器，耗时较长）：
```
exec(command="mcporter call xiaohongshu.<工具名> 参数1=值1 参数2=值2", timeout=180)
```

如果参数值包含空格或中文，用引号包裹：
```
exec(command="mcporter call xiaohongshu.post_comment_to_feed feed_id=abc xsec_token=xyz content=\"这是评论内容\"", timeout=180)
```

### 浏览与搜索

| exec 命令 | 说明 | 参数 |
|-----------|------|------|
| `mcporter call xiaohongshu.list_feeds` | 获取首页推荐（返回约30-35条，无 limit 参数） | - |
| `mcporter call xiaohongshu.search_feeds keyword=关键词` | 搜索笔记 | keyword(必需), sort_by, note_type, publish_time, search_scope, location |
| `mcporter call xiaohongshu.get_feed_detail feed_id=ID xsec_token=TOKEN` | 笔记详情+评论 | feed_id(必需), xsec_token(必需), load_all_comments, limit, click_more_replies, reply_limit, scroll_speed |
| `mcporter call xiaohongshu.user_profile user_id=ID xsec_token=TOKEN` | 用户主页 | user_id(必需), xsec_token(必需) |

### 互动操作

| exec 命令 | 说明 | 参数 |
|-----------|------|------|
| `mcporter call xiaohongshu.like_feed feed_id=ID xsec_token=TOKEN` | 点赞 | unlike=true 取消 |
| `mcporter call xiaohongshu.favorite_feed feed_id=ID xsec_token=TOKEN` | 收藏 | unfavorite=true 取消 |
| `mcporter call xiaohongshu.post_comment_to_feed feed_id=ID xsec_token=TOKEN content="评论"` | 顶级评论 | - |
| `mcporter call xiaohongshu.reply_comment_in_feed feed_id=ID xsec_token=TOKEN comment_id=CID content="回复"` | 回复评论 | user_id(可选) |

### 发布操作

| exec 命令 | 说明 | 参数 |
|-----------|------|------|
| `mcporter call xiaohongshu.publish_content title="标题" content="正文" images='["path1.jpg"]'` | 发布图文 | tags, schedule_at |
| `mcporter call xiaohongshu.publish_with_video title="标题" content="正文" video="path.mp4"` | 发布视频 | tags, schedule_at |

### 账号管理

| exec 命令 | 说明 |
|-----------|------|
| `mcporter call xiaohongshu.check_login_status` | 检查登录状态 |
| `mcporter call xiaohongshu.get_login_qrcode` | 获取登录二维码 |
| `mcporter call xiaohongshu.delete_cookies` | 重置登录 |

### 评论系统要点

- `get_feed_detail` 返回嵌套评论结构：每个评论有 `id`、`content`、`userInfo`、`subComments[]`
- 获取全部评论：设置 `load_all_comments=true`
- 展开子评论：设置 `click_more_replies=true`
- 回复某条评论：用 `reply_comment_in_feed`，传入该评论的 `comment_id`

注意：遵守 `memory/xhs-state.md` 中的频率限制，操作间隔 ≥ 15 秒。
