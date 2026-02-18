# HEARTBEAT — 立即执行以下步骤

你现在被心跳触发了。不要分析这个文件，不要描述任务，立即用工具逐步执行。

## 步骤 1：读取运营状态

用 read 工具读取 `memory/xhs-state.md`。
- 如果日期不是今天 → 用 edit 工具把日期改为今天，所有计数归零。
- 如果所有配额已满 → 直接回复 HEARTBEAT_OK，结束。

## 步骤 2：刷小红书 Feed

用 exec 工具执行以下命令（直接复制粘贴，不要修改）：

```
mcporter call xiaohongshu.list_feeds
```

阅读返回的 JSON 结果，从中挑出 1-3 条有趣的笔记。
对感兴趣的笔记，用 exec 工具执行（把 `实际ID` 和 `实际TOKEN` 替换为真实值）：

```
mcporter call xiaohongshu.get_feed_detail feed_id=实际ID xsec_token=实际TOKEN
```

把有价值的内容摘要用 edit 工具追加到 `memory/xhs-digest.md`。

## 步骤 3：互动

对喜欢的内容用 exec 工具执行点赞命令：

```
mcporter call xiaohongshu.like_feed feed_id=实际ID xsec_token=实际TOKEN
```

如果你对某条内容有想说的话，先读 `xhs-persona.md` 确保风格一致，然后用 exec 工具发表评论（注意 content 值用引号包裹）：

```
mcporter call xiaohongshu.post_comment_to_feed feed_id=实际ID xsec_token=实际TOKEN content="你的评论内容"
```

每次操作间隔至少 15 秒。

## 步骤 4：检查自己笔记的评论

读取 `memory/xhs-performance.md`，找到最近发布的笔记 ID。
如果有笔记记录，用 exec 工具执行获取评论：

```
mcporter call xiaohongshu.get_feed_detail feed_id=笔记ID xsec_token=TOKEN load_all_comments=true
```

如果有新评论需要回复，用 exec 工具回复（注意 comment_id 从评论列表获取）：

```
mcporter call xiaohongshu.reply_comment_in_feed feed_id=笔记ID xsec_token=TOKEN comment_id=评论ID content="你的回复内容"
```

## 步骤 5：更新状态

用 edit 工具更新 `memory/xhs-state.md`，增加今天的操作计数。

## 完成

回复 HEARTBEAT_OK。

## 故障排除

- 如果 mcporter 返回 "offline" 或 "Unable to reach server"，用 exec 执行：`mcporter config list` 确认 xiaohongshu server 已配置
- 如果返回 "未登录"，用 exec 执行：`mcporter call xiaohongshu.get_login_qrcode` 获取登录二维码
- 所有 mcporter 命令都通过 exec 工具执行，不能直接调用函数
