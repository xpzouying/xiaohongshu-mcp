# 小红书发布流程参考

本文档描述通过 CDP（Chrome DevTools Protocol）自动发布内容到小红书创作者中心的完整流程。

## 前置条件

1. **Chrome 浏览器已安装** - 标准 Google Chrome
2. **Python 依赖已安装** - `websockets`、`requests`
3. **首次登录已完成** - 至少登录过一次小红书（cookie 持久化在专用 profile 中）

## 流程概览

**上传图文模式**:
```
生成文案 → 用户确认 → 启动 Chrome → 检查登录 → 导航发布页 → 上传图片 → 填写标题 → 填写正文 → 用户确认发布
```

**写长文模式**:
```
生成文案 → 用户确认 → 启动 Chrome → 检查登录 → 导航发布页 → 点击"写长文"tab → 点击"新的创作" → 填写标题 → 填写正文 → 一键排版 → 用户选择模板 → 下一步 → 填写发布页正文描述 → 用户确认发布
```

## 详细步骤

### 1. 启动 / 连接 Chrome

脚本: `scripts/chrome_launcher.py`

- 检测 `127.0.0.1:9222` 端口是否已有 Chrome 实例
- 若无，启动 Chrome 并附带以下参数:
  - `--remote-debugging-port=9222`
  - `--user-data-dir=%LOCALAPPDATA%/Google/Chrome/XiaohongshuProfile`
  - `--no-first-run`
  - `--no-default-browser-check`
  - `--headless=new`（仅在无头模式下）
- 等待端口就绪（最多 15 秒）

**用户数据目录说明**: 使用独立的 `XiaohongshuProfile` 目录，与用户日常浏览器 profile 完全隔离，不会干扰正常使用。

**无头模式说明**: 使用 `--headless` 参数启动时，Chrome 不会显示窗口，适合自动化发布。如需登录或切换账号，脚本会自动切换到有窗口模式。

### 2. 检查登录状态

脚本: `scripts/cdp_publish.py` → `check_login()`

- 导航到 `https://creator.xiaohongshu.com`
- 检查当前 URL 是否包含 "login"（被重定向到登录页）
- 检查页面是否存在用户信息相关的 DOM 元素
- 若未登录，提示用户在 Chrome 窗口中扫码登录

### 3. 导航到发布页

- 目标 URL: `https://creator.xiaohongshu.com/publish/publish`
- 等待页面完全加载

### 4. 上传图片

脚本: `scripts/cdp_publish.py` → `_upload_images()`

- 通过 CDP `DOM.querySelector` 定位 `input[type="file"]` 元素
- 使用 CDP `DOM.setFileInputFiles` 命令设置文件路径
- 等待图片上传和处理完成

**图片来源**: 如果图片是 URL，先用 `scripts/image_downloader.py` 下载到临时目录，发布后自动清理。

### 5. 填写标题

脚本: `scripts/cdp_publish.py` → `_fill_title()`

- 定位标题输入框
- 设置 value 并触发 `input` 和 `change` 事件

### 6. 填写正文

脚本: `scripts/cdp_publish.py` → `_fill_content()`

- 定位 contenteditable 编辑区域（TipTap/ProseMirror editor）
- 将正文按段落拆分，包裹为 `<p>` 标签写入 innerHTML，段落之间插入 `<p><br></p>` 空行
- 触发 `input` 事件

### 7. 用户确认并发布

- 脚本填写完成后暂停，提示用户在浏览器中检查预览
- 用户确认后，脚本点击发布按钮
- 或用户选择手动点击发布按钮

## 写长文模式详细步骤

### 1-2. 启动 Chrome 和检查登录

同上传图文模式。

### 3. 导航到发布页并点击"写长文"tab

脚本: `scripts/cdp_publish.py` → `_click_long_article_tab()`

- 导航到 `https://creator.xiaohongshu.com/publish/publish`
- 在 `div.creator-tab` 中查找文本为"写长文"的 tab 并点击

### 4. 点击"新的创作"

脚本: `scripts/cdp_publish.py` → `_click_new_creation()`

- 在页面中查找包含"新的创作"文本的元素并点击
- 等待长文编辑器页面加载

### 5. 填写长文标题

脚本: `scripts/cdp_publish.py` → `_fill_long_title()`

- 定位 `textarea.d-text[placeholder="输入标题"]` 元素
- 使用 `HTMLTextAreaElement.prototype.value` 的 native setter 设置值
- 触发 `input` 和 `change` 事件

### 6. 填写长文正文

同上传图文模式的正文填写（TipTap/ProseMirror 编辑器）。

### 7. 一键排版

脚本: `scripts/cdp_publish.py` → `_click_auto_format()`

- 查找并点击"一键排版"按钮
- 等待模板列表加载

### 8. 模板选择

脚本: `scripts/cdp_publish.py` → `get_template_names()` + `select_template(name)`

- `get_template_names()` 从 `.template-card .template-title` 获取所有模板名称
- `select_template(name)` 点击指定名称的模板卡片
- 已选中的模板卡片 class 为 `template-card selected`

### 9. 下一步并填写发布页描述

脚本: `scripts/cdp_publish.py` → `click_next_and_prepare_publish(content)`

- 点击"下一步"按钮进入发布预览页
- 发布页有独立的正文描述编辑器（`div.tiptap.ProseMirror`），需要单独填入内容

### 10. 用户确认并发布

同上传图文模式。

## DOM 选择器参考

> **注意**: 小红书前端可能随时更新，以下选择器基于编写时的页面结构。如果自动化失败，需要在浏览器 DevTools 中重新抓取选择器，并更新 `cdp_publish.py` 中的 `SELECTORS` 字典。

| 元素 | 主选择器 | 备选选择器 | 说明 |
|---|---|---|---|
| 图片上传 | `input.upload-input` | `input[type="file"]` | 隐藏的文件输入，通过 CDP 直接操作 |
| 标题输入（图文） | `input[placeholder*="填写标题"]` | `input.d-text` | 图文模式标题输入框 |
| 标题输入（长文） | `textarea.d-text[placeholder="输入标题"]` | - | 长文模式 textarea 标题 |
| 正文编辑 | `div.tiptap.ProseMirror` | `div.ProseMirror[contenteditable="true"]` | TipTap/ProseMirror 富文本编辑器 |
| 发布按钮 | 文本匹配"发布" | - | 通过遍历按钮文本定位 |
| 写长文 tab | 文本匹配"写长文"（`div.creator-tab`） | - | 长文模式入口 |
| 新的创作按钮 | 文本匹配"新的创作" | - | 长文编辑器入口 |
| 一键排版按钮 | 文本匹配"一键排版" | - | 触发模板选择 |
| 模板卡片 | `.template-card` | `.template-card.selected`（已选） | 排版模板列表 |
| 模板名称 | `.template-card .template-title` | - | 模板卡片内的名称 span |
| 下一步按钮 | 文本匹配"下一步" | - | 模板选择后进入发布页 |
| 登录检测 | URL 包含 "login" | `.user-info, .creator-header` | 重定向检测 + DOM 元素检测 |

## 选择器维护指南

当小红书更新页面导致自动化失败时:

1. 在 Chrome 中打开 `https://creator.xiaohongshu.com/publish/publish`
2. 按 F12 打开开发者工具
3. 使用元素选择器（Ctrl+Shift+C）定位目标元素
4. 记录新的选择器
5. 更新 `scripts/cdp_publish.py` 中 `SELECTORS` 字典对应的值

## 错误处理

| 错误 | 原因 | 解决方案 |
|---|---|---|
| Chrome 未启动 | 端口 9222 无响应 | 运行 `chrome_launcher.py` 或手动启动 Chrome |
| 找不到 Chrome | 非标准安装路径 | 检查 Chrome 安装，或在脚本中指定路径 |
| 未登录 | cookie 过期或首次使用 | 在 Chrome 窗口中扫码登录 |
| 选择器失效 | 小红书页面更新 | 按上述维护指南更新选择器 |
| 图片上传失败 | 文件路径错误或格式不支持 | 检查图片路径，确保格式为 jpg/png/webp |
| 发布按钮找不到 | 页面未完全加载 | 增加等待时间或手动点击发布 |

## CLI 用法

所有脚本位于 `scripts/` 目录。

### 方式 A: 统一 pipeline（推荐）

```bash
# 无头模式（推荐）- 无浏览器窗口，更快
python publish_pipeline.py --headless --title "标题" --content "正文" --image-urls URL1 URL2

# 无头模式 - 从文件读取标题和正文
python publish_pipeline.py --headless --title-file title.txt --content-file body.txt --image-urls URL1

# 有窗口模式 - 用于调试或首次登录
python publish_pipeline.py --title "标题" --content "正文" --image-urls URL1 URL2

# 使用本地图片文件
python publish_pipeline.py --headless --title "标题" --content "正文" --images img1.jpg img2.jpg

# 填写并自动发布
python publish_pipeline.py --headless --title "标题" --content "正文" --image-urls URL1 --auto-publish
```

输出状态码:
- 退出码 0 + `READY_TO_PUBLISH` = 表单已填写，等待确认
- 退出码 0 + `PUBLISHED` = 已发布
- 退出码 1 + `NOT_LOGGED_IN` = 未登录，需扫码（无头模式下会自动切换到有窗口模式）
- 退出码 2 = 其他错误

### 方式 B: 分步调用（图文模式）

```bash
# 1. 启动 Chrome（可选 --headless）
python chrome_launcher.py
python chrome_launcher.py --headless

# 2. 检查登录（退出码 0=已登录, 1=未登录）
python cdp_publish.py check-login
python cdp_publish.py --headless check-login

# 3. 填写表单
python cdp_publish.py fill --title "标题" --content-file body.txt --images img1.jpg
python cdp_publish.py --headless fill --title "标题" --content-file body.txt --images img1.jpg

# 4. 用户确认后点击发布
python cdp_publish.py click-publish

# 或一步完成填写+发布
python cdp_publish.py --headless publish --title "标题" --content-file body.txt --images img1.jpg
```

### 方式 C: 分步调用（长文模式）

```bash
# 1. 启动 Chrome
python chrome_launcher.py

# 2. 检查登录
python cdp_publish.py check-login

# 3. 填写长文 + 一键排版（输出包含 TEMPLATES JSON）
python cdp_publish.py long-article --title-file title.txt --content-file content.txt

# 4. 选择模板
python cdp_publish.py select-template --name "模板名称"

# 5. 下一步 + 填写发布页正文描述
python cdp_publish.py click-next-step --content-file content.txt

# 6. 用户确认后点击发布
python cdp_publish.py click-publish
```

### 方式 D: Pipeline 长文模式

```bash
# 长文模式（图片可选）
python publish_pipeline.py --mode long-article --title-file title.txt --content-file content.txt
python publish_pipeline.py --mode long-article --title "标题" --content "正文" --images img1.jpg
```

### 账号管理

```bash
# 首次登录或 session 过期 - 打开浏览器扫码登录
python cdp_publish.py login

# 切换账号 - 清除 cookie 并打开登录页
python cdp_publish.py switch-account

# 关闭 Chrome
python chrome_launcher.py --kill

# 重启 Chrome（可选无头模式）
python chrome_launcher.py --restart
python chrome_launcher.py --restart --headless
```

### Claude Code 集成

在 Claude Code 中通过 Bash 工具调用。推荐使用 pipeline 方式:

1. 将中文标题和正文写入临时文本文件（UTF-8 编码）
2. 调用 `publish_pipeline.py --headless` 传入文件路径和图片 URL
3. 根据输出状态码处理结果：
   - 未登录 → 脚本自动切换到有窗口模式，提示用户扫码
   - 已填写 → 请用户确认预览
4. 用户确认后调用 `cdp_publish.py click-publish` 发布

**切换账号流程**:
1. 调用 `cdp_publish.py switch-account`
2. 等待用户扫码确认
3. 继续正常发布流程
