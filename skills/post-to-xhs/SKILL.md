---
name: post-to-xhs
description: >
  小红书内容发布技能。支持两种输入方式：(1) 用户提供完整内容和图片/图片URL，直接发布；
  (2) 用户提供网页URL，自动提取内容和图片，适当总结后发布。如果从URL提取不到图片，
  提示用户手动下载并提供。适用于任何类型的内容发布。
---

# 小红书内容发布

根据用户输入自动判断发布方式，简化发布流程。

## 工作流程

```
用户输入
    │
    ├─ 完整内容 + 图片/图片URL → 直接进入发布流程
    │
    └─ 网页 URL → WebFetch 提取内容和图片
                      │
                      ├─ 有图片 → 适当总结内容 → 发布流程
                      │
                      └─ 无图片 → 提示用户手动下载图片
                                  │
                                  └─ 用户提供图片后 → 发布流程
```

## Step 1: 判断输入类型

根据用户输入判断：

- **完整内容模式**：用户提供了标题、正文内容、以及图片（本地路径或URL）
- **URL 提取模式**：用户只提供了一个网页 URL

如果不确定，询问用户。

## Step 2: 处理内容

### 完整内容模式

直接使用用户提供的标题和正文，跳到 Step 3。

### URL 提取模式

1. 使用 WebFetch 提取网页内容
2. 提取关键信息：标题、正文、图片URL
3. 适当总结内容，保持：
   - 关键信息完整
   - 语言自然流畅
   - 适合小红书阅读习惯

#### 图片提取失败处理

如果从网页中提取不到图片URL，或图片URL无法访问，**必须**：

1. 告知用户图片提取失败
2. 提供原网页链接，请用户手动访问
3. 指导用户：
   - 在浏览器中打开原网页
   - 右键点击想要的图片 → "图片另存为" 或 "复制图片地址"
   - 将保存的图片路径或复制的图片URL提供给我
4. 等待用户提供图片后再继续发布流程

**示例提示语**：
```
从网页中未能提取到可用的图片。请手动获取：

1. 打开原文链接：[URL]
2. 找到合适的配图，右键另存为本地，或复制图片地址
3. 将图片路径或URL发给我

拿到图片后我们继续发布。
```

## Step 3: 内容检查

### 标题检查

标题长度必须 ≤ 38，计算规则：
- 中文字符和中文标点（《》、，。等）：每个计 2
- 英文字母/数字/空格/ASCII标点：每个计 1

如果超长，自动生成符合长度要求的新标题，保持语义一致。

### 正文格式

- 段落之间使用双换行分隔
- 语言自然，避免机器翻译感
- 简体中文

## Step 4: 发布到小红书

完整发布流程参考: [references/publish-workflow.md](references/publish-workflow.md)

### 4.1 用户确认内容

通过 `AskUserQuestion` 向用户展示即将发布的内容（标题、正文、图片），获得明确确认后再继续。

### 4.2 选择发布模式

通过 `AskUserQuestion` 让用户选择发布模式：

- **无头模式**（推荐）：后台运行，速度快，无浏览器窗口。发布完成后直接报告结果。
- **有窗口模式**：显示浏览器窗口，可以预览内容。需要用户确认后再点击发布。

```
AskUserQuestion 示例：
问题：选择发布模式
选项：
  - 无头模式（推荐）：后台快速发布，无需预览
  - 有窗口模式：显示浏览器，可预览确认
```

### 4.3 写入临时文件

将标题和正文写入临时 UTF-8 文本文件。不要在 `python -c` 中内联中文文本。

### 4.4 运行 Pipeline

根据用户选择的模式执行发布脚本：

**无头模式**（添加 `--headless` 参数）：
```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\publish_pipeline.py" --headless --title-file title.txt --content-file content.txt --image-urls "URL1" "URL2"
```

**有窗口模式**（不添加 `--headless`）：
```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\publish_pipeline.py" --title-file title.txt --content-file content.txt --image-urls "URL1" "URL2"
```

**其他参数**：
```bash
# 发布到指定账号
python ... --account myaccount ...

# 使用本地图片
python ... --images "C:\path\to\image.jpg"
```

处理输出：
- `NOT_LOGGED_IN` (exit code 1) → 脚本自动切换到有窗口模式，提示用户扫码登录，确认后重新运行
- `READY_TO_PUBLISH` (exit code 0) → 根据模式进入下一步
- Exit code 2 → 报告错误

### 4.5 用户预览确认（仅有窗口模式）

**仅当用户选择有窗口模式时**，使用 `AskUserQuestion` 请用户在浏览器中检查预览，确认后再发布。

无头模式跳过此步骤，直接进入 4.6。

### 4.6 点击发布

点击发布按钮：

```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" click-publish
```

### 4.7 报告结果

根据命令输出告知用户发布是否成功。

## 重要提示

- **绝不自动发布** - 必须在 Step 4.4 获得用户确认
- **图片必须有** - 小红书发布必须有图片，没有图片不能发布
- **无头模式**：使用 `--headless` 参数自动化发布。如需登录，脚本自动切换到有窗口模式
- 如果页面结构变化导致选择器失效，参考 `references/publish-workflow.md` 更新

## 账号管理

系统支持多个小红书账号，每个账号有独立的 Chrome profile。

### 列出账号

```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" list-accounts
```

### 添加账号

```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" add-account myaccount --alias "我的账号"
```

### 登录

```bash
# 默认账号
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" login

# 指定账号
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" --account myaccount login
```

### 切换账号

```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" switch-account
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" --account otheraccount switch-account
```

### 设置默认账号

```bash
python "C:\Users\admin\AI\.claude\skills\post-to-xhs\scripts\cdp_publish.py" set-default-account myaccount
```
