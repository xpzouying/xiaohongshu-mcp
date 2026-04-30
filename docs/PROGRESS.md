# 项目经验与进度总结

> 最后更新: 2026-04-30
> 当前分支: `feature/favorite-management`

---

## 📋 项目背景

xiaohongshu-mcp 是一个小红书 MCP 服务器，目标是通过 MCP 协议提供小红书的各种自动化能力（收藏获取、专辑管理、内容发布等）。

---

## 🔴 历史卡点（2026-03-24）

### 卡点 1：CSP 铜墙铁壁

尝试了 **6 种方案** 全部失败：

| 方案 | 结果 | 原因 |
|------|------|------|
| 1. fetch API | ❌ Failed to fetch | CSP 直接拦截 |
| 2. XMLHttpRequest | ❌ status 0 | CSP 阻止网络请求 |
| 3. 注入全局函数 | ❌ 导航后清除 | 页面刷新丢失注入代码 |
| 4. AddScriptToEvaluateOnNewDocument | ❌ 仍然被阻止 | CSP 策略严格 |
| 5. CDP Runtime.evaluate | ❌ JS 语法错误 | 字符串拼接问题 |
| 6. 直接 HTTP 调用 | ❌ 404 | API 端点不公开 |

**结论**：小红书 CSP 无法通过常规方式绕过，API 端点隐藏且有签名机制。

### 卡点 2：浏览器自动化只能在 UI 层面

唯一可行方案是 **UI 自动化**（点击按钮、输入文本），但不受 CSP 限制。

### 卡点 3：服务器环境限制

- 无图形界面，需要 Xvfb 虚拟显示
- 内存有限（1.7GB），Chromium headless 容易 OOM
- 浏览器自动化在服务器上性能差

### 当时的最终方案

用 Go 编写的 `auto-album-sync` 工具，通过 UI 操作（点击、输入）完成专辑同步。

**状态**：工具已编译，功能完整，但需要在本地电脑运行（有图形界面）。

---

## 🟢 新方案：browser-use AI Agent（2026-04-30）

### 思路

用 [browser-use](https://github.com/browser-use/browser-use)（91k stars）替代手写 CDP 选择器：
- **自然语言指令** 替代硬编码 CSS 选择器
- **AI 自适应页面**，不怕页面改版
- **复用已有数据**：145 条笔记分类 + cookies.json

### 新增目录

```
xiaohongshu-mvp-sync/
├── config.py              # LLM/Browser/Project 配置
├── utils/
│   ├── cookie_manager.py  # Cookie 加载、过期检查
│   └── logger.py          # 结构化日志
├── tasks/
│   └── xhs_tasks.py       # 分类数据解析 + Agent 任务指令生成
├── sync_agent.py          # 主入口：browser-use Agent 循环
├── report.py              # 报告输出（占位）
├── data/
│   └── 收藏分类结果.json   # 软链接到已有数据
├── .env.example           # 环境变量模板
└── .gitignore             # __pycache__
```

### 架构

```
sync_agent.py (主入口)
    ↓
┌─────────────────────────────────────────┐
│  browser-use Agent (AI 决策层)            │
│  LLM: qwen3.6-plus (百炼)                │
│  输入: 自然语言任务指令                   │
│  输出: 浏览器操作决策                     │
├─────────────────────────────────────────┤
│  browser-use Browser (浏览器驱动层)       │
│  基于 Playwright，控制 Chromium          │
├─────────────────────────────────────────┤
│  Chromium (headless 或有图形界面)         │
│  Cookie 自动注入，自动登录小红书          │
└─────────────────────────────────────────┘
```

### 核心代码要点

**任务指令模板**（`tasks/xhs_tasks.py`）：
```python
task = """
你正在操作小红书的网页版（www.xiaohongshu.com），已登录。
1. 导航到我的收藏页面
2. 创建名为「{album_name}」的专辑
3. 将以下笔记移入专辑：{note_list}
4. 确认所有笔记都已移入
"""
```

**Agent 执行循环**（`sync_agent.py`）：
```python
for category, note_ids in classification.items():
    task = build_sync_task(category, note_ids)
    agent = Agent(task=task, llm=llm, browser=browser, use_thinking=True)
    result = await agent.run()
    # 收集结果...
```

### 环境要求

| 组件 | 要求 | 当前状态 |
|------|------|----------|
| Python | >= 3.11 | ✅ 服务器有 3.11 |
| browser-use | pip install | ✅ 已安装测试 |
| Chromium | 任意版本 | ✅ 服务器有 133 |
| LLM API | OpenAI 兼容 | ✅ 百炼 qwen3.6-plus |
| Cookie | cookies.json | ✅ 已有，未过期 |

### 已知问题

**服务器 headless 环境不稳定**：
- 内存有限（784MB available），Chromium headless 启动 ~300-400MB
- browser-use 的 Browser 封装做了额外工作（下载扩展、watchdog 等），内存开销大
- 容易 OOM 被系统 kill

**解决方案**：在有图形界面的本地电脑运行（Mac/PC），浏览器资源充足。

### 本地运行指南

```bash
# 1. 拉取代码
git clone https://github.com/conlanWu/xiaohongshu-mcp.git
cd xiaohongshu-mcp
git checkout feature/favorite-management

# 2. 创建虚拟环境
uv init && uv add browser-use

# 3. 设置环境变量
export DASHSCOPE_API_KEY="sk-sp-xxx"  # 百炼 API Key

# 4. 确保 cookies.json 在项目根目录（从服务器拷贝）

# 5. 运行
cd xiaohongshu-mvp-sync
uv run python sync_agent.py
```

---

## 🔑 关键技术经验

### 经验 1：CDP 是双刃剑

- CDP 能做的事情很多（截图、DOM 操作、网络监听、注入 JS）
- 但 CSP 层面会拦截浏览器内的网络请求
- `Page.setBypassCSP` 可能是绕过 CSP 的关键（未验证）

### 经验 2：浏览器自动化在服务器上跑不通

- 无图形界面 → 需要 Xvfb 或 headless
- headless 模式不稳定，资源消耗大
- 服务器内存 < 2GB 不建议跑浏览器自动化

### 经验 3：AI Agent 浏览器自动化是新方向

- browser-use 用自然语言指令替代硬编码选择器
- AI 能自适应页面结构变化
- 但依赖 LLM API 调用，需要稳定的网络连接
- 内存消耗比纯 Playwright 大（因为要加载 LLM SDK）

### 经验 4：数据准备是关键

- 收藏分类（145 条笔记，7 个分类）已完成
- Cookie 管理（cookies.json）已就绪
- 这些是同步任务的前置条件，已经完成

---

## 📁 项目文件清单

### 已有（历史工作）

| 文件/目录 | 用途 |
|-----------|------|
| `mcp_server.go` | MCP 服务器主文件 |
| `mcp_handlers.go` | MCP 工具 handlers |
| `handlers_api.go` | API handlers（部分 TODO）|
| `xiaohongshu/` | 小红书浏览器操作模块 |
| `favorite_album_handlers.go` | 收藏专辑管理 handlers |
| `favorite_album_tools.go` | 专辑同步工具 |
| `browser_pool.go` | 浏览器池优化 |
| `收藏分类结果.json` | 145 条笔记分类数据 |
| `cookies.json` | 小红书登录 Cookie |

### 新增（本次工作）

| 文件/目录 | 用途 |
|-----------|------|
| `xiaohongshu-mvp-sync/` | browser-use 专辑同步 MVP |
| `superpower/` | Superpowers skill 工具包 |
| `docs/plans/` | 设计文档和实现计划 |

---

## 🚀 下一步

1. **在有图形界面的电脑上运行** `sync_agent.py`
2. 验证 browser-use Agent 能否完成专辑同步
3. 如果成功，考虑集成到 MCP 服务器（Phase B）
4. 如果失败，调试并优化 Agent 任务指令

---

*生成时间: 2026-04-30*
