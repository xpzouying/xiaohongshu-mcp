# Design: 专辑同步 MVP — browser-use Agent

**日期**: 2026-04-30  
**状态**: Draft → 待审核  
**范围**: 专辑同步功能（A），后续扩展 MCP 集成（B）

---

## 1. 背景

### 问题

之前的 xiaohongshu-mcp 专辑同步方案卡在 3 个问题上：
1. 小红书 CSP 无法绕过（5 种方案失败）
2. API 端点隐藏 + 签名机制，逆向成本高
3. 只能走 UI 自动化（选择器硬编码，页面改版即崩溃）

### 解决方案

用 `browser-use`（AI Agent 浏览器自动化框架）替代手写 CDP 选择器：
- **自然语言指令** 替代硬编码 CSS 选择器
- **AI 自适应页面结构**，不怕页面改版
- **复用已有数据**：145 条笔记分类、cookies.json

---

## 2. 架构

```
xiaohongshu-mvp-sync/
├── sync_agent.py          # 主入口：browser-use Agent 驱动
├── config.py              # 配置（LLM、浏览器路径、超时等）
├── tasks/
│   └── xhs_tasks.py       # 定义同步任务步骤
├── utils/
│   ├── cookie_manager.py  # Cookie 加载/注入
│   └── logger.py          # 结构化日志
├── data/
│   └── 收藏分类结果.json   # 已有数据（145 条笔记，7 个分类）
└── .env                   # LLM API Key（不提交）
```

### 技术选型

| 组件 | 选择 | 原因 |
|------|------|------|
| 语言 | Python 3.11 | 服务器已有 |
| 浏览器 | Chromium 133 | 服务器已有，支持 headless |
| 框架 | browser-use 0.12.6 | AI Agent 浏览器自动化 |
| LLM | qwen3.6-plus (百炼) | OpenAI 兼容 API，当前可用 |
| 数据 | 复用现有 JSON | 145 条笔记分类已完成 |

---

## 3. 核心流程

```
1. 启动 headless Chromium
2. 注入 cookies.json 实现自动登录
3. 读取 收藏分类结果.json
4. 对每个分类：
   a. 导航到收藏页面
   b. 创建同名专辑
   c. 选中该分类下的笔记
   d. 执行"移入专辑"操作
   e. 验证移动结果
5. 输出同步报告（成功/失败统计）
```

### Agent 任务模板

```python
task = """
你正在操作小红书的网页版（www.xiaohongshu.com），已登录。

请按以下步骤完成任务：
1. 进入我的收藏页面（https://www.xiaohongshu.com/user/favorite）
2. 创建一个名为"{album_name}"的专辑
3. 从我的收藏中，将以下笔记移入刚创建的专辑：
   - 笔记ID列表：{note_ids}
4. 确认所有笔记都已移入

注意：
- 每一步操作后等待页面加载完成
- 如果遇到弹窗或提示，点击确认/关闭
- 完成后报告成功/失败的笔记数量
"""
```

---

## 4. 组件职责

### sync_agent.py
- 初始化 browser-use Browser + Agent
- 加载配置和 cookie
- 遍历分类，逐个执行同步任务
- 汇总结果，输出报告

### config.py
- LLM 配置（model, base_url, api_key）
- 浏览器配置（headless, viewport, timeout）
- 路径配置（data 目录、cookie 文件）

### utils/cookie_manager.py
- 从 cookies.json 加载 cookie
- 通过 browser-use 注入 cookie 到浏览器
- 验证登录状态

### utils/logger.py
- 结构化日志（每个分类的操作记录）
- 输出到文件 + 控制台

---

## 5. 错误处理

| 场景 | 处理方式 |
|------|---------|
| Cookie 过期 | 提示用户重新登录，等待扫码 |
| 页面结构变化 | browser-use AI 自适应，不依赖选择器 |
| 单个笔记移动失败 | 记录失败，继续处理下一个 |
| 创建专辑失败（重名） | 查找已有专辑，直接使用 |
| LLM API 超时 | 重试 2 次，失败则跳过该分类 |

---

## 6. 测试策略

### MVP 阶段
1. **手动验证**：运行 sync_agent.py，观察浏览器操作是否正确
2. **结果验证**：检查小红书网页版，确认专辑和笔记是否正确

### 后续集成阶段（Phase B）
1. **单元测试**：cookie_manager、config 加载
2. **集成测试**：模拟浏览器操作（可用 playwright mock）
3. **回归测试**：每次页面改版后验证核心流程

---

## 7. 成功标准

- [ ] 能自动登录小红书（通过 cookie）
- [ ] 能创建专辑
- [ ] 能将笔记移入专辑
- [ ] 145 条笔记全部同步完成
- [ ] 输出同步报告（成功/失败统计）

---

## 8. 后续扩展（Phase B）

MVP 验证成功后，集成到 xiaohongshu-mcp Go 服务器：
- 通过 subprocess 调用 Python 脚本
- 作为 MCP 工具暴露：`xhs_sync_albums`
- 支持参数：分类名、笔记列表

---

*设计审核通过 → 进入 Phase 2: Writing Plans*
