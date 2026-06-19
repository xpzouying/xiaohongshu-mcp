# xiaohongshu-mcp 封号/禁言问题根源分析报告

> 数据来源：GitHub Issues (100条) + 核心源码分析
> 分析日期：2026-06-18

---

## 一、封号/禁言相关 Issues 概览

| Issue# | 标题 | 状态 | 创建时间 | 评论数 |
|--------|------|------|----------|--------|
| #715 | 最近使用mcp容易被封号 | open | 2026-06-15 | 2 |
| #668 | 封号 | open | 2026-04-17 | **19** |
| #674 | 建议：增加反检测机制 | open | 2026-04-28 | 6 |
| #711 | 评论操作遇风控拦截页时自动重试导航 | open | 2026-06-06 | 0 |
| #680 | 违规问题 | open | 2026-05-02 | 7 |
| #702 | 发文频率间隔多久比较安全 | open | 2026-05-19 | 1 |
| #699 | XHS域名识别和IP侦测问题 | open | 2026-05-18 | 0 |
| #681 | MCP服务内置登录触发短信验证码拦截 | open | 2026-05-03 | 1 |
| #693 | 发帖功能异常 卡在最后发布阶段 | open | 2026-05-14 | 0 |
| #694 | 发布按钮识别失败，卡在最后发布阶段 | open | 2026-05-14 | 0 |

---

## 二、问题根源（按严重程度排序）

### ⚠️ 根源1：浏览器自动化指纹完全暴露

**检测方式：** `navigator.webdriver` 标志位

项目使用 `go-rod` + `go-rod/stealth` 作为浏览器自动化框架。虽然 `stealth` 包能覆盖部分指纹（如移除 `navigator.webdriver` flag、修改 `chrome.runtime` 等），但：

- **Chrome 启动参数暴露自动化特征**：`headless_browser.go` 中仅设置了 `--no-sandbox` 和自定义 User-Agent，未使用关键反检测参数如 `--disable-blink-features=AutomationControlled`
- **CDP (Chrome DevTools Protocol) 特征明显**：小红书风控可以检测到 CDP 连接，而真人浏览器不会有 CDP 连接
- **`MustPage()` 模式被识别**：`stealth.MustPage(b.browser)` 虽然有一定隐匿效果，但小红书持续更新风控策略，stealth 模式已不足以应对

**Issue #681 明确证实：** headless 自动化浏览器登录会被小红书识别，触发短信验证码拦截；而独立 login 二进制（可见浏览器窗口）则不会触发拦截。

### ⚠️ 根源2：机械化的操作行为模式

源码中大量存在以下可被检测的自动化特征：

1. **固定 Sleep 间隔**（publish.go, login.go, comment_feed.go 遍及）：
   ```
   time.Sleep(2 * time.Second)   // 固定等待
   time.Sleep(1 * time.Second)   // 处处可见
   time.Sleep(500 * time.Millisecond)
   ```
   真人操作不可能每次都精确等待相同时间。

2. **直线鼠标轨迹 & 固定点击坐标**：
   - `tab.Click(proto.InputMouseButtonLeft, 1)` — 直接触发 CDP click，无鼠标轨迹模拟
   - `page.Mouse.MustMoveTo().MustClick()` — 虽然publish.go `clickEmptyPosition()` 有 `rand.Intn()` 随机偏移，但移动是瞬移，无贝塞尔曲线轨迹

3. **无输入节奏模拟**：
   - `inputEl.Input(content)` — 一次性注入所有文本，而非逐字符模拟真人打字节奏
   - 无随机 Thinking Gap / 输入间隔

4. **直接 JS Eval 操作**：
   - `page.MustEval("window.scrollBy(0, delta)")` — 直接调 JS API 滚动，而非模拟鼠标滚轮
   - `page.Eval("...")` 在多处使用，这些 JS 注入点可被检测

### ⚠️ 根源3：无频率限制与节流控制

- 项目**未内置任何速率限制**（rate limiting）
- 用户可以高频调用 `search_feeds`、`post_comment`、`publish_content` 等
- **Issue #702 用户反馈**：连续发3条后就被限制
- **Issue #668 用户反馈**：2小时检测一次评论并自动回复，1天就封号
- 真人不可能在短时间内频繁操作

### ⚠️ 根源4：IP/环境指纹问题

1. **数据中心 IP 被标记**：
   - 大量用户使用阿里云、腾讯云等 VPS 部署
   - **Issue #680 关键信息**："阿里云服务器一定被封" — 用户 @yyds956 确认
   - 小红书风控系统清楚知道云服务商的 IP 段，直接标记为高风险

2. **IP 与 Cookie 不匹配**：
   - 用户在本地浏览器获取 Cookie，复制到云服务器使用
   - Cookie 中的 Session IP 与实际发布 IP 不一致
   - 小红书风控将此判定为账号盗用或异常登录

3. **浏览器环境不连续**：
   - 每次 MCP 请求可能使用不同浏览器上下文
   - 缺少持续的用户行为画像（浏览历史、滚动模式等）

### ⚠️ 根源5：无真人行为模拟层

- **Issue #674** 详细列出了所需的反检测特性，但截至分析时尚未完全实现：
  1. ❌ 鼠标贝塞尔曲线轨迹
  2. ❌ 点击坐标随机化（仅有 `clickEmptyPosition` 简单实现）
  3. ❌ 时间间隔正态分布随机化
  4. ❌ 逐字符打字节奏模拟
  5. ❌ 分段随机滚动行为
  6. ❌ Canvas/WebGL 指纹混淆
  7. ❌ 发布前随机浏览其他内容

---

## 三、社区已尝试的解决方案及效果

| 解决方案 | 提出者 | 效果 |
|----------|--------|------|
| 改用独立 login 二进制 | #681 | ✅ 解决登录拦截，但运行时仍被封 |
| 增加 CloakBrowser 支持 | #714/#713/#706 | 🟡 改善指纹隐匿 |
| 人工化 rod 封装 (#716 PR) | tanxxjun321 | 🟡 最新未合并，改善行为模拟 |
| browser-act stealth 模式 | #674 评论 @ccmagia2-gif | 🟡 声称封号率明显下降 |
| 使用代理IP | #680 讨论 | ❌ 仍被封 |
| 仅自己可见发布 | #680 @TnTeQAQ | ❌ 仍被封 |
| 降低操作频率 | #702 | 🟡 短期有效，长期仍被检测 |

---

## 四、总结：被封号的完整因果链

```
用户部署 MCP 服务
    ↓
浏览器启动 (headless=true, 无反检测参数)
    ↓
CDP 连接建立 → `navigator.webdriver` 或其他自动化指纹暴露
    ↓
操作行为机械化 (固定Sleep、瞬移鼠标、批量文本注入)
    ↓
高频操作 / 无速率限制
    ↓
IP 为数据中心IP / Cookie IP 不匹配
    ↓
小红书风控系统识别为 Bot
    ↓
⚠️ 警告 → ⚠️ 7天禁言 → ❌ 永久封号
```

**根据用户反馈 (#668 @TimYuJian):**
> "未实名发文→冻结→实名解冻之后→再自动化发文就是警告→然后是7天→直接喜提永久"

**核心结论：** 封号是多因素叠加的结果，单个措施（如仅修改 User-Agent 或仅使用 stealth）不足以应对小红书持续升级的风控系统。**行为层 + 指纹层 + IP层 + 频率层** 四维都需要改进。

---

## 五、分支分析（共5个分支）

### 1️⃣ feat/humanize ⭐ 最核心 — 直接反封号

**状态：** 未合并 | 创建者: tanjun | 最后提交: 2026-06-17

这是 **PR #716** 对应的分支，专为降低封号率而设计。新增了完整的 `pkg/humanize/` 行为模拟层：

| 模块 | 文件 | 功能 |
|------|------|------|
| `pkg/humanize/humanize.go` | Actor 入口 | 组合 Mouse + Keyboard，提供统一 API |
| `pkg/humanize/config.go` | 配置系统 | SpeedProfile(Slow/Normal/Fast)、MouseConfig、KeyboardConfig |
| `pkg/humanize/mouse.go` | **鼠标模拟** (14KB) | 多段贝塞尔曲线轨迹 + 随机抖动 + 中间暂停 + 过冲校正 + 滚动注入 |
| `pkg/humanize/keyboard.go` | **键盘模拟** (12KB) | 变量速打字 + 爆发式节奏 + 拼写错误+自动纠正 |
| `pkg/humanize/path.go` | **路径生成** (5KB) | CubicBezier/QuadBezier/Sigmoid 三种曲线族 + 多段分割 |
| `pkg/humanize/rod/hrod.go` | **hrod封装** (16KB) | 将 go-rod Page 包装为人类化操作接口 |
| `browser/browser.go` | **修改版** | 使用 `hrod.NewBrowser()` 替代原 headless_browser 直用 |

**改动核心：** `browser/browser.go` 最后一行调用 `hrod.NewBrowser(hb, humanize.DefaultConfig())`，将普通浏览器实例包裹为人类化行为模式。

**局限性：**
- 依然基于 `go-rod/stealth` + `headless_browser`，浏览器自动化指纹（navigator.webdriver 等）未被根除
- headless 模式仍然可被检测
- 未解决频率限制 / IP 问题
- 尚未合并到 main

---

### 2️⃣ feature/product-binding — Python CDP 备选路线

**状态：** 未合并 | 创建者: tanjun | 最后提交: 2026-03-03

此分支带来了一个**完全不同的方案**——使用 Python 直接通过 Chrome DevTools Protocol (CDP) 操作真实 Chrome 进程：

**核心脚本：**
- `chrome_launcher.py` (297行) — Chrome 进程管理器
  - 检测本地 Chrome 是否已在监听调试端口
  - 使用独立用户数据目录持久化登录态
  - 支持 headless/headed 切换（登录时有头，发布时无头）
  - **多账号支持**：每个账号独立 profile 目录
- `cdp_publish.py` (1057行) — CDP 发布引擎
  - 通过 WebSocket 连接 Chrome CDP 端口
  - 封装了完整的发布流程：检查登录 → 上传图片 → 填写标题 → 填写正文 → 点击发布
  - 支持多账号切换、重新登录
- `SKILL.md` — Agent 技能描述文件

**与 Go-rod 方案的关键差异：**

| 维度 | Go-rod 方案 (main/feat/humanize) | Python CDP 方案 (feature/product-binding) |
|------|------|------|
| 浏览器实例 | go-rod 内部启动 | **复用用户真实 Chrome 进程** |
| 指纹隐匿 | 依赖 stealth 包 | **使用用户真实浏览器指纹** |
| 登录态 | Cookie 导出/导入 | **独立用户数据目录持久化** |
| 多账号 | 无支持 | ✅ 原生支持 |
| 头部/无头切换 | 固定 | ✅ 登录有头/发布无头动态切换 |

---

### 3️⃣ feature/fix-xhs-explore-search ⭐ 搜索路由+筛选Bug双修
**状态：** 未合并 | 创建者: tanjun | 最后提交: 2026-05-28

**修复内容（经网页验证确认有效）：**

| 修改点 | main版本（Bug） | fix分支（修复） |
|--------|---------------|----------------|
| 筛选点击方式 | `div.tags:nth-child(N)` 索引偏移 | **JS Eval按文本匹配**：`tags.find(tag => tag.innerText.trim() === text)` |
| aria-hidden过滤 | 无 | ✅ `tag.getAttribute('aria-hidden') === 'true'` 跳过隐藏标签 |
| 搜索路由 | `/search_result` | ✅ `/search_result_ai`（适配网站改版） |

**为什么nth-child是Bug（经网页验证）：**
```
div.tag-container
  ├── div.tags.active (hidden, aria-hidden="true") — "综合" ← child 1
  ├── div.tags — "综合"  ← child 2
  ├── div.tags — "最新"  ← child 3
  ├── div.tags — "最新"  ← child 4 (重复副本)
  └── ...
```
代码假设 `TagsIndex: 2 → "最新"` 对应 `nth-child(2)`，但实际child 2是可见的"综合"标签。**所有筛选选项偏移1位。**

**Fix分支的修复方案**用JS遍历所有 `div.tags`，跳过 `aria-hidden=true` 的，用text精确匹配。✅ 正确。

### 4️⃣ fix/xhs-search-route-filters
- **与feature/fix-xhs-explore-search完全一致**（同一次提交）
- 筛选Bug修复 + 搜索路由变更
- 未涉及反检测/拟人化

### 5️⃣ main
- 仅 README 文档更新（WeChat QR code, 2026-06-15）
- 封号/禁言的反检测代码 **均未合入**
- 筛选Bug、搜索路由、拟人化 **均未修复**

### 6️⃣ PR #714: CloakBrowser支持（已合并到main，tag: v2026.06.12.1403-5c43e3d）
**状态：** ✅ **已合并到main** | 创建者: tanxxjun321 | 合并: 2026-06-12

**改动内容：** 非代码逻辑修改，而是**浏览器二进制替换**方案

| 修改文件 | 改动内容 |
|----------|---------|
| `Dockerfile` | amd64/arm64双架构构建，安装CloakBrowser Chromium |
| `build/find_cloakbrowser_binary.py` | 新增，自动检测CloakBrowser路径 |
| `main.go` | 新增 `-bin` flag指定浏览器二进制路径 |
| `handlers_api.go` | 传递浏览器路径给browser初始化 |
| `browser/browser.go` | 使用 `ROD_BROWSER_BIN` env var指向CloakBrowser |
| `docker/docker-compose.yml` | 增加构建配置 |

**对反封号的影响：**
- ✅ CloakBrowser Chromium内置了webdriver/plugins/languages/WebGL等指纹伪装
- ❌ **xiaohongshu/和browser/的Go自动化代码未任何修改**
- ❌ 筛选Bug、搜索路由、拟人化行为 **均未修复**

**实质：** 用CloakBrowser替代原生Chromium，浏览器指纹层改善，但行为层（鼠标/键盘/时序）和代码逻辑层（筛选/路由）不受影响。

---

## 六、实际网页验证结果——对预判的修正

> 以下结论基于对 https://www.xiaohongshu.com 的实际浏览器操作验证（29轮测试）

### ❌ 我之前说错的（自查纠正）

| 我之前说的 | 实际验证结果 | 验证方法 |
|-----------|-------------|---------|
| `feeds.go`读不到数据，value为空 | ✅ **数据存在**（35条SSR），只需等hydration完成 | `__INITIAL_STATE__.feed.feeds.value.length = 35` |
| `div.filter-panel`不存在 | ✅ **存在**，初始`display:none`，hover/click后渲染 | `document.querySelector('div.filter-panel')` 返回元素 |
| source参数不匹配（`web_explore_feed` vs `web_search_result_notes`） | ✅ **两种都work**，页面自动加`type=51` | 实测返回44条数据 |
| 登录选择器不对 | ✅ **选择器正确**，`.main-container .user .link-wrapper .channel` 已登录态存在 | `channelInLink: true` |
| 筛选面板需要Click不是Hover | ✅ **Hover也能打开**，`MustHover()`没问题 | `panelAfterHover: true` |

### ✅ 实际验证通过的代码逻辑

1. **搜索数据提取** — `__INITIAL_STATE__.search.feeds.value` ✅ 每条含`id`, `xsecToken`, `modelType`, `noteCard`
2. **浏览feeds** — `__INITIAL_STATE__.feed.feeds.value` SSR有35条 ✅ 但需等Vue hydration
3. **详情页** — `xsec_token` URL参数有效 ✅ `note.noteDetailMap`结构完整 ✅
4. **评论** — 初始在state中为空（API后加载），代码有完整滚动加载逻辑 ✅
5. **登录检测** — `CheckLoginStatus`选择器有效 ✅
6. **筛选面板** — `div.filter` hover打开 ✅ 5组filters（排序依据/笔记类型/发布时间/搜索范围/位置距离）✅

---

## 七、API/功能Bug清单（经网页验证）

### Bug 1: 筛选标签nth-child索引偏移 ⚠️
**影响：筛选功能实际上选不对**

- **文件：** `xiaohongshu/search.go:205`
- **症状：** 选"最新"实际点到"综合"，选"最多点赞"实际点到"最新"
- **根因：** `div.tags.active` 隐藏标签 + 每个选项两副本导致索引偏移
- **状态：** ✅ `fix/xhs-search-route-filters` 已修（JS文本匹配），main未修

### Bug 2: `interactInfo` 字段全为string类型 ℹ️
- **文件：** `xiaohongshu/types.go:50-55`
- **实状：** 小红书API返回 `likedCount: "2764"`（string）
- **影响：** Go json.Unmarshal可自动转数字字符串为int，但如果值是`""`或`"N/A"`会失败
- **状态：** 所有分支未修。优先度低（实测API都返回数字字符串）

### Bug 3: feeds.go 1s可能不够 hydration 🐢
- **文件：** `xiaohongshu/feeds.go:30`
- **症状：** 慢网络下1s不够Vue hydration完成，读到空的`feeds.value`
- **状态：** 所有分支未修。可用`MustWaitStable()`替代固定sleep

### Bug 4: 搜索路由过时 🔄
- **文件：** `xiaohongshu/search.go:250`
- **症状：** 当前网页搜索跳转到`/search_result_ai`而非`/search_result`
- **状态：** ✅ `fix/xhs-search-route-filters` 已修，main未修

### Bug 5: 无反检测CDP参数 🛡️
- **原Go代码**分支都依赖 `go-rod/stealth` 无额外参数
- 缺少 `--disable-blink-features=AutomationControlled`
- 未移除 `navigator.webdriver` flag
- 无viewport随机化，默认User-Agent
- ⚠️ **PR #714 CloakBrowser** 部分解决：切换为CloakBrowser Chromium（内置指纹伪装：webdriver/plugins/languages/WebGL等），但Go代码层**未追加额外CDP参数**
- **状态：** 原代码未修；使用CloakBrowser时浏览器指纹层改善

### Bug 6: 拟人化不足 🤖
- 固定sleep、瞬移鼠标、一次性文字注入、JS直接滚动
- **状态：** ✅ `feat/humanize` 已修（pkg/humanize层），main未修

---

## 八、总览：三条反封号路线 + 一条功能修复线

```
┌─ 路线A: feat/humanize ──────────────────────────────────────┐
│  "行为层拟人化"                                               │
│  鼠标：贝塞尔曲线 + 多段分割 + 抖动 + 过冲 + 中途滚动         │
│  键盘：变速 + 爆发节奏 + 拼写错误 + 自动纠正                  │
│  依赖：go-rod + stealth + headless_browser (底层未变)        │
│  局限性：浏览器指纹仍暴露、headless可检测、无频率限制          │
│  **状态：未合并到main**                                       │
└──────────────────────────────────────────────────────────────┘

┌─ 路线B: feature/product-binding ────────────────────────────┐
│  "环境层真人化"                                               │
│  复用真实 Chrome 进程 → 真实浏览器指纹                         │
│  独立用户数据目录 → 持久登录态                                 │
│  有头/无头动态切换 → 登录通过真人验证                          │
│  多账号管理 → 降低单账号风险                                   │
│  局限性：需要本地Chrome、Python运行时、未集成到Go服务           │
│  **状态：未合并到main**                                       │
└──────────────────────────────────────────────────────────────┘

┌─ 路线C: PR #714 CloakBrowser ─────────────────────────────┤
│  "浏览器层反指纹"                                            │
│  切换二进制到CloakBrowser Chromium                           │
│  内置指纹伪装：webdriver/plugins/languages/WebGL等           │
│  Docker amd64+arm64双架构支持                                │
│  不影响Go代码（xiaohongshu/和browser/未动）                  │
│  局限性：行为层仍AI化、Go代码Bug未修、需额外下载CloakBrowser   │
│  **状态：已合并到main（tag: v2026.06.12.1403-5c43e3d）**     │
└──────────────────────────────────────────────────────────────┘

结论：三条路线互补。
- 路线A解决"操作像不像人"
- 路线B解决"环境像不像人"
- 路线C解决"浏览器指纹像不像人"

小红书风控检测的是多维特征（浏览器指纹 + 行为模式 + API请求模式 + 频率），
单一维度改进不足以规避封号。当前main仅合入了路线C（浏览器层），
路线A和路线B仍为独立分支未合并，且Go代码中的功能Bug（筛选索引、时序、搜索路由）
也未修复。
