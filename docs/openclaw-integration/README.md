# OpenClaw × xiaohongshu-mcp 集成部署方案

让你的 OpenClaw AI Agent 自主运营小红书账号——自动浏览 Feed、点赞、评论、发帖，并通过心跳机制和定时任务实现全天候持续运营。

## 目录

- [方案概述](#方案概述)
- [前置条件](#前置条件)
- [架构说明](#架构说明)
- [部署步骤](#部署步骤)
  - [第一步：启动 xiaohongshu-mcp 服务](#第一步启动-xiaohongshu-mcp-服务)
  - [第二步：配置 mcporter](#第二步配置-mcporter)
  - [第三步：安装 OpenClaw Skill](#第三步安装-openclaw-skill)
  - [第四步：配置工作区文件](#第四步配置工作区文件)
  - [第五步：配置心跳任务](#第五步配置心跳任务)
  - [第六步：配置定时 Cron 任务](#第六步配置定时-cron-任务)
  - [第七步：设置开机自启（macOS）](#第七步设置开机自启macos)
- [Skill 使用方式](#skill-使用方式)
- [工具完整参考](#工具完整参考)
- [频率控制与风控](#频率控制与风控)
- [常见问题排查](#常见问题排查)

---

## 方案概述

```
用户（飞书/其他渠道）
       ↓ 指令
  OpenClaw Agent
       ↓ exec
   mcporter CLI  ←→  xiaohongshu-mcp（Go 服务，端口 18060）
                             ↓ 浏览器自动化
                        小红书网页
```

**核心组件：**

| 组件 | 说明 |
|------|------|
| [xiaohongshu-mcp](https://github.com/xpzouying/xiaohongshu-mcp) | Go 编写的 MCP 服务，通过 Chromium 浏览器自动化操作小红书 |
| [mcporter](https://github.com/openclaw/mcporter) | MCP 客户端 CLI，OpenClaw Agent 通过 `exec` 调用它与 MCP 服务通信 |
| [OpenClaw](https://openclaw.ai) | AI Agent 平台，提供心跳、定时任务、Skill 等机制 |
| OpenClaw Skill | 本方案提供的封装层，将 13 个 MCP 工具暴露给 Agent |

---

## 前置条件

- **macOS**（本方案在 macOS Apple Silicon 上验证，Linux 同理）
- **Go 1.21+**（编译 xiaohongshu-mcp）
- **Node.js 18+**（运行 mcporter）
- **OpenClaw** 已安装并配置好渠道（飞书等）
- **mcporter** 已全局安装：`npm install -g mcporter`
- 一个**小红书账号**（用于 Agent 运营）

---

## 架构说明

### 为什么用 mcporter 而不是直接调用 MCP？

OpenClaw Agent 通过 `exec` 工具执行 shell 命令。`mcporter` 是一个 MCP 客户端 CLI，可以将 MCP 工具调用转为命令行调用，使 Agent 无需了解 MCP 协议细节即可操作小红书。

### 调用链路

```
Agent exec("mcporter call xiaohongshu.list_feeds")
  → mcporter 读取 ~/.mcporter/mcporter.json 找到 xiaohongshu 服务地址
  → HTTP 请求 → xiaohongshu-mcp（localhost:18060）
  → Go 服务驱动 Chromium 浏览器执行操作
  → 返回 JSON 结果给 Agent
```

### 工作区文件结构

Agent 通过读写工作区文件实现状态持久化：

```
~/.openclaw/workspace/
├── HEARTBEAT.md          # 心跳任务指令（每 N 分钟触发一次）
├── SOUL.md               # Agent 人格定义（自定义）
├── TOOLS.md              # 工具使用指南（告知 Agent 如何调用 mcporter）
├── xhs-persona.md        # 小红书账号人设（自定义）
└── memory/
    ├── xhs-state.md      # 每日配额和操作计数
    ├── xhs-digest.md     # 浏览内容摘要积累
    ├── xhs-knowledge.md  # 社区趋势认知
    ├── xhs-content-ideas.md  # 内容创意池
    └── xhs-performance.md    # 发帖效果追踪
```

---

## 部署步骤

### 第一步：启动 xiaohongshu-mcp 服务

**方式 A：直接运行（开发/测试）**

```bash
cd /path/to/xiaohongshu-mcp
go run . -port=:18060 -headless=true
```

**方式 B：编译后运行（推荐生产环境）**

```bash
cd /path/to/xiaohongshu-mcp
go build -o xiaohongshu-mcp .
./xiaohongshu-mcp -port=:18060 -headless=true
```

服务启动后监听 `http://localhost:18060/mcp`。

**登录小红书账号：**

首次使用需要扫码登录，登录信息保存为 `cookies.json`（在项目目录下）：

```bash
cd /path/to/xiaohongshu-mcp
go run cmd/login/main.go
```

浏览器会打开小红书登录页，用手机 App 扫码即可。登录后 cookies 自动保存，后续无头模式启动时自动加载。

> **重要**：`cookies.json` 已在 `.gitignore` 中排除，请勿提交到版本库。

### 第二步：配置 mcporter

安装 mcporter CLI：

```bash
npm install -g mcporter
```

创建系统级配置文件 `~/.mcporter/mcporter.json`：

```json
{
  "mcpServers": {
    "xiaohongshu": {
      "baseUrl": "http://localhost:18060/mcp"
    }
  },
  "imports": []
}
```

验证连接：

```bash
mcporter list xiaohongshu
# 应显示 13 个工具列表

mcporter call xiaohongshu.check_login_status
# 应显示：✅ 已登录
```

### 第三步：安装 OpenClaw Skill

将本方案提供的 Skill 目录复制到 OpenClaw 工作区：

```bash
cp -r docs/openclaw-integration/skills/xiaohongshu-mcp \
  ~/.openclaw/workspace/skills/xiaohongshu-mcp
```

Skill 目录结构：

```
skills/xiaohongshu-mcp/
├── SKILL.md        # Skill 描述（Agent 读取以了解如何使用）
├── package.json    # Node.js 包描述
└── scripts/
    └── index.js    # Skill 主脚本（封装 13 个 mcporter 工具）
```

### 第四步：配置工作区文件

将模板文件复制到 OpenClaw 工作区并按需修改：

```bash
# 复制所有模板
cp docs/openclaw-integration/workspace-templates/HEARTBEAT.md \
  ~/.openclaw/workspace/HEARTBEAT.md

cp docs/openclaw-integration/workspace-templates/TOOLS-xhs-section.md \
  ~/.openclaw/workspace/TOOLS-xhs-section.md
# （将此文件内容追加到你的 TOOLS.md 末尾）

cp docs/openclaw-integration/workspace-templates/memory/xhs-state.md \
  ~/.openclaw/workspace/memory/xhs-state.md

cp docs/openclaw-integration/workspace-templates/memory/xhs-digest.md \
  ~/.openclaw/workspace/memory/xhs-digest.md

cp docs/openclaw-integration/workspace-templates/memory/xhs-performance.md \
  ~/.openclaw/workspace/memory/xhs-performance.md

cp docs/openclaw-integration/workspace-templates/memory/xhs-content-ideas.md \
  ~/.openclaw/workspace/memory/xhs-content-ideas.md
```

**创建账号人设文件** `~/.openclaw/workspace/xhs-persona.md`（按你的需求自定义）：

```markdown
# 小红书账号人设

## 账号定位
[描述你的账号定位，例如：科技博主、生活记录者等]

## 内容方向
[列出主要内容方向]

## 语言风格
[描述发帖和评论的语言风格]

## 话题标签
[常用的话题标签]
```

**在 `openclaw.json` 中配置心跳频率**（推荐 5 分钟）：

```json
{
  "agents": {
    "defaults": {
      "heartbeat": {
        "every": "5m",
        "target": "last"
      },
      "sandbox": {
        "mode": "off"
      }
    }
  }
}
```

> **注意**：`sandbox.mode` 必须设为 `"off"`，否则 `exec` 命令在非主 session 中无法运行（需要 Docker）。

### 第五步：配置心跳任务

心跳任务（`HEARTBEAT.md`）会在每次心跳触发时由 Agent 读取并执行。模板已提供，核心流程：

1. 读取 `memory/xhs-state.md` 检查配额
2. 调用 `list_feeds` 获取推荐内容
3. 挑选感兴趣的笔记查看详情
4. 点赞、评论（根据配额）
5. 检查自己笔记的新评论并回复
6. 更新状态文件

详见 `workspace-templates/HEARTBEAT.md`。

### 第六步：配置定时 Cron 任务

通过 OpenClaw CLI 添加定时任务（以下为示例，时间和内容按需调整）：

**早间创作任务（每天 10:00）：**

```bash
openclaw cron add \
  --name xhs-morning-create \
  --schedule "0 10 * * *" \
  --tz "Asia/Shanghai" \
  --session isolated \
  --message "小红书早间创作时间。立即按步骤执行：
1. 用 read 读取 xhs-persona.md 了解账号人设。
2. 用 read 读取 memory/xhs-state.md 检查今日发帖配额。如果日期不是今天，先用 edit 重置。如果发帖已满，回复"配额已满，跳过"并结束。
3. 用 read 读取 memory/xhs-content-ideas.md，选一个话题。
4. 用 read 读取 memory/xhs-knowledge.md，获取认知背景。
5. 以账号人设创作一篇小红书笔记（300-800字），标题15字以内，3-5个话题标签。
6. 用 exec 执行发布命令（替换实际值）：
   mcporter call xiaohongshu.publish_content title=\"你的标题\" content=\"你的正文 #标签1 #标签2\" images='[\"图片路径\"]'
   注意：如果没有图片，可以先跳过发布，把内容草稿存入 memory/xhs-content-ideas.md。
7. 如果发布成功，用 edit 更新 memory/xhs-performance.md 记录新笔记。
8. 用 edit 更新 memory/xhs-state.md 增加发帖计数。"
```

**下午互动任务（每天 14:00）：**

```bash
openclaw cron add \
  --name xhs-afternoon-interact \
  --schedule "0 14 * * *" \
  --tz "Asia/Shanghai" \
  --session isolated \
  --message "小红书午间互动时间。立即按步骤执行：
1. 用 read 读取 xhs-persona.md 了解账号人设和评论风格。
2. 用 read 读取 memory/xhs-state.md 检查今日配额。如果日期不是今天，先用 edit 重置。
3. 用 exec 浏览推荐内容：
   mcporter call xiaohongshu.list_feeds
4. 从返回结果中挑出 3-5 条感兴趣的笔记，用 exec 查看详情：
   mcporter call xiaohongshu.get_feed_detail feed_id=实际ID xsec_token=实际TOKEN
5. 对优质内容用 exec 点赞：
   mcporter call xiaohongshu.like_feed feed_id=实际ID xsec_token=实际TOKEN
6. 对有想法的内容用 exec 评论（评论要有独特视角，20-80字）：
   mcporter call xiaohongshu.post_comment_to_feed feed_id=实际ID xsec_token=实际TOKEN content=\"你的评论\"
7. 每次 exec 操作之间间隔至少 15 秒（用 exec 执行 sleep 15）。
8. 把有价值的内容摘要用 edit 追加到 memory/xhs-digest.md。
9. 用 edit 更新 memory/xhs-state.md 增加操作计数。"
```

**晚间创作任务（每天 20:00）：**

```bash
openclaw cron add \
  --name xhs-evening-create \
  --schedule "0 20 * * *" \
  --tz "Asia/Shanghai" \
  --session isolated \
  --message "小红书晚间创作时间。立即按步骤执行：
1. 用 read 读取 xhs-persona.md 了解账号人设。
2. 用 read 读取 memory/xhs-state.md 检查今日发帖配额。如果日期不是今天，先用 edit 重置。如果发帖已满，回复"配额已满，跳过"并结束。
3. 用 read 读取 memory/xhs-content-ideas.md，选一个轻松有趣的话题。
4. 用 read 读取 memory/xhs-digest.md，看看今天浏览过什么有趣内容可以参考。
5. 以账号人设创作一篇轻松向笔记（AI日常/互联网冲浪类），300-800字，标题15字以内。
6. 用 exec 执行发布命令（替换实际值）：
   mcporter call xiaohongshu.publish_content title=\"你的标题\" content=\"你的正文 #标签1 #标签2\" images='[\"图片路径\"]'
   注意：如果没有图片，可以先跳过发布，把内容草稿存入 memory/xhs-content-ideas.md。
7. 如果发布成功，用 edit 更新 memory/xhs-performance.md 记录新笔记。
8. 用 edit 更新 memory/xhs-state.md 增加发帖计数。"
```

**周总结任务（每周日 22:00）：**

```bash
openclaw cron add \
  --name xhs-weekly-review \
  --schedule "0 22 * * 0" \
  --tz "Asia/Shanghai" \
  --session isolated \
  --message "小红书周总结时间。立即按步骤执行：
1. 用 read 读取 memory/xhs-performance.md，分析本周所有笔记的表现数据。
2. 用 read 读取 memory/xhs-state.md，查看本周累计数据。
3. 用 read 读取 memory/xhs-digest.md，回顾本周浏览的内容。
4. 分析：哪些内容表现好？哪些不好？什么时间段效果最佳？
5. 将认知洞察用 edit 整合到 memory/xhs-knowledge.md（社区观察、话题趋势、受众画像等）。
6. 根据分析用 edit 调整 memory/xhs-content-ideas.md（增加新灵感、淘汰效果差的方向）。
7. 用 edit 清理 memory/xhs-digest.md 中已过期的旧摘要。
8. 用 edit 更新 memory/xhs-performance.md 增加周总结。
9. 用 edit 重置 memory/xhs-state.md 的本周累计。"
```

### 第七步：设置开机自启（macOS）

编译 xiaohongshu-mcp 二进制文件并通过 launchd 管理：

```bash
# 编译
cd /path/to/xiaohongshu-mcp
go build -o ~/.openclaw/services/xiaohongshu-mcp/xiaohongshu-mcp .

# 创建日志目录
mkdir -p ~/.openclaw/logs
```

创建 launchd plist 文件 `~/Library/LaunchAgents/com.xiaohongshu-mcp.plist`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.xiaohongshu-mcp</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/YOUR_USERNAME/.openclaw/services/xiaohongshu-mcp/xiaohongshu-mcp</string>
        <string>-port=:18060</string>
        <string>-headless=true</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/xiaohongshu-mcp</string>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/.openclaw/logs/xiaohongshu-mcp.out.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/.openclaw/logs/xiaohongshu-mcp.err.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>
```

> **注意**：`WorkingDirectory` 必须指向 xiaohongshu-mcp 项目目录（`cookies.json` 所在位置），否则服务启动后无法加载登录状态。

加载服务：

```bash
launchctl load ~/Library/LaunchAgents/com.xiaohongshu-mcp.plist
```

验证：

```bash
launchctl list | grep xiaohongshu-mcp
# 应显示 PID 和服务名

mcporter call xiaohongshu.check_login_status
# 应显示：✅ 已登录
```

---

## Skill 使用方式

安装 Skill 后，在与 OpenClaw Agent 的对话中可以直接指令操作小红书：

```
# 浏览推荐
"帮我刷一下小红书推荐"

# 搜索
"帮我搜搜 AI agent 相关的笔记"

# 查看笔记详情
"帮我看看这条笔记的评论：https://www.xiaohongshu.com/explore/xxx"

# 互动
"帮我给这条笔记点个赞"
"帮我评论这条笔记，说说你对 AI 的看法"

# 发布
"帮我发一篇关于 AI 工具使用心得的小红书笔记"
```

Agent 会通过 `exec` 工具执行对应的 `mcporter call` 命令完成操作。

---

## 工具完整参考

所有工具通过 `exec` 调用 `mcporter`，**建议设置 timeout=180**（MCP 操作涉及浏览器，耗时较长）：

```
exec(command="mcporter call xiaohongshu.<工具名> 参数1=值1", timeout=180)
```

### 浏览与搜索

| 工具名 | 说明 | 必需参数 | 可选参数 |
|--------|------|----------|----------|
| `list_feeds` | 获取首页推荐（约30-35条，无 limit 参数） | - | - |
| `search_feeds` | 搜索笔记 | `keyword` | `sort_by`, `note_type`, `publish_time`, `search_scope`, `location` |
| `get_feed_detail` | 笔记详情+评论 | `feed_id`, `xsec_token` | `load_all_comments`, `limit`, `click_more_replies`, `reply_limit`, `scroll_speed` |
| `user_profile` | 用户主页信息 | `user_id`, `xsec_token` | - |

### 互动操作

| 工具名 | 说明 | 必需参数 | 可选参数 |
|--------|------|----------|----------|
| `like_feed` | 点赞 | `feed_id`, `xsec_token` | `unlike=true`（取消） |
| `favorite_feed` | 收藏 | `feed_id`, `xsec_token` | `unfavorite=true`（取消） |
| `post_comment_to_feed` | 发表顶级评论 | `feed_id`, `xsec_token`, `content` | - |
| `reply_comment_in_feed` | 回复评论（楼中楼） | `feed_id`, `xsec_token`, `content` | `comment_id`, `user_id` |

### 发布操作

| 工具名 | 说明 | 必需参数 | 可选参数 |
|--------|------|----------|----------|
| `publish_content` | 发布图文笔记 | `title`, `content`, `images`（数组） | `tags`（数组）, `schedule_at` |
| `publish_with_video` | 发布视频笔记 | `title`, `content`, `video`（本地路径） | `tags`（数组）, `schedule_at` |

### 账号管理

| 工具名 | 说明 |
|--------|------|
| `check_login_status` | 检查登录状态 |
| `get_login_qrcode` | 获取登录二维码（Base64 图片） |
| `delete_cookies` | 删除 cookies 重置登录 |

### 评论系统说明

`get_feed_detail` 返回嵌套评论结构：

```json
{
  "comments": [
    {
      "id": "评论ID",
      "content": "评论内容",
      "userInfo": { "userId": "...", "nickname": "..." },
      "subComments": [
        {
          "id": "子评论ID",
          "content": "回复内容",
          "userInfo": { ... }
        }
      ]
    }
  ]
}
```

回复评论时，传入 `comment_id`（一级评论 ID）即可实现楼中楼回复：

```bash
mcporter call xiaohongshu.reply_comment_in_feed \
  feed_id=xxx xsec_token=yyy \
  comment_id=评论ID content="你的回复内容"
```

---

## 频率控制与风控

建议在 `memory/xhs-state.md` 中维护每日配额，Agent 每次操作前检查：

```markdown
# 小红书运营状态

## 今日配额

日期：YYYY-MM-DD

- 发帖：已用 0 / 上限 3
- 点赞：已用 0 / 上限 30
- 评论：已用 0 / 上限 20
- 收藏：已用 0 / 上限 20
- 浏览：已用 0 / 上限 200

## 频率规则

- 任意两次操作之间间隔 ≥ 15 秒
- 同类操作之间间隔 ≥ 1 分钟
- 每小时最多 30 次操作（所有类型合计）
- 活跃时段：08:00 - 23:00（CST）
- 非活跃时段不执行任何操作
- 如果检测到风控信号（操作失败/验证码），立即停止当日所有操作
```

---

## 常见问题排查

**Q: `mcporter call` 报 "Unable to reach server"**

检查 xiaohongshu-mcp 服务是否在运行：
```bash
lsof -i :18060
# 应显示监听进程
```

如果没有，手动启动：
```bash
cd /path/to/xiaohongshu-mcp
./xiaohongshu-mcp -port=:18060 -headless=true &
```

**Q: `check_login_status` 返回"未登录"**

cookies 已过期，需要重新登录：
```bash
cd /path/to/xiaohongshu-mcp
go run cmd/login/main.go
```

**Q: Agent 执行 mcporter 超时**

浏览器操作耗时较长，确保 `exec` 调用时设置了足够的超时时间：
```
exec(command="mcporter call xiaohongshu.post_comment_to_feed ...", timeout=180)
```

**Q: Agent 在 Cron/Heartbeat 中无法执行 exec**

检查 `openclaw.json` 中 `sandbox.mode` 是否为 `"off"`。如果是 `"non-main"` 或 `"all"`，非主 session 中的 exec 需要 Docker 环境。

**Q: 心跳触发了但 Agent 只描述任务不执行**

在 `openclaw.json` 中为心跳添加明确的 prompt：
```json
{
  "agents": {
    "defaults": {
      "heartbeat": {
        "every": "5m",
        "prompt": "你现在被心跳触发了。立刻读取 HEARTBEAT.md，然后按照里面的步骤逐步执行。你必须调用工具完成任务，不要只是描述任务内容。完成所有步骤后回复 HEARTBEAT_OK。"
      }
    }
  }
}
```

**Q: `list_feeds` 传了 `limit` 参数但返回了 30+ 条**

`list_feeds` 不支持 `limit` 参数，始终返回约 30-35 条推荐内容，这是上游 API 的限制。

---

## 相关链接

- [xiaohongshu-mcp 原项目](https://github.com/xpzouying/xiaohongshu-mcp)
- [OpenClaw 官网](https://openclaw.ai)
- [mcporter CLI](https://github.com/openclaw/mcporter)
- [OpenClaw 文档 - Heartbeat](https://docs.openclaw.ai/heartbeat)
- [OpenClaw 文档 - Cron](https://docs.openclaw.ai/cli/cron)
- [OpenClaw 文档 - Skills](https://docs.openclaw.ai/skills)
