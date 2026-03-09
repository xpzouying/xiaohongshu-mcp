# 小红书 MCP 防爬方案文档

本文档基于源码分析，整理了项目中所有已实现的反检测 / 防爬机制。

---

## 总览

项目采用 **6 层防护** 策略，从浏览器底层到应用层逐级覆盖：

| 层级 | 防护手段 | 核心思路 | 所在文件 |
|------|----------|----------|----------|
| 1 | 浏览器指纹隐藏 | 隐藏自动化特征 | `headless_browser.go` |
| 2 | 随机化延迟体系 | 消除固定时间模式 | `feed_detail.go` 等 |
| 3 | 人类行为模拟 | 鼠标 / 键盘 / 滚动拟人化 | `feed_detail.go`, `publish.go` |
| 4 | 智能自适应策略 | 停滞检测 + 动态调整 | `feed_detail.go` |
| 5 | 会话持久化 | Cookie 复用避免频繁登录 | `cookies.go`, `browser.go` |
| 6 | IP 代理支持 | 隐藏真实 IP | `browser.go` |

---

## 1. 浏览器指纹隐藏（Stealth 模式）

### 1.1 依赖库

```
go-rod/stealth v0.4.9   -- 专业浏览器反检测插件
xpzouying/headless_browser v0.3.0 -- 封装层，自动启用 stealth
```

### 1.2 核心实现

`headless_browser.go:154-156`:

```go
func (b *Browser) NewPage() *rod.Page {
    return stealth.MustPage(b.browser)  // 每个页面都通过 stealth 创建
}
```

**每个新建的页面都经过 `stealth.MustPage()` 处理**，这意味着所有浏览器操作都在反检测保护下执行。

### 1.3 stealth 插件的作用

`go-rod/stealth` 会自动注入 JavaScript 来隐藏以下自动化痕迹：

- `navigator.webdriver = false` — 隐藏 WebDriver 标记
- 伪造 `chrome.runtime` — 模拟真实 Chrome 扩展环境
- 伪造 `Permissions API` — 绕过权限检测
- 隐藏 `Headless Chrome` 特征 — User-Agent、`window.chrome` 等
- 伪造 `plugins`、`languages` 等浏览器属性

### 1.4 User-Agent 伪装

`headless_browser.go:38`:

```go
UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
```

固定使用真实 Chrome 浏览器的 User-Agent，并通过 `launcher.Set("user-agent", ...)` 设置到浏览器启动参数中。

---

## 2. 随机化延迟体系

### 2.1 延迟配置中心

`feed_detail.go:33-45` 定义了 7 种人类行为延迟区间：

```go
type delayConfig struct {
    min, max int  // 毫秒
}

var (
    humanDelayRange   = delayConfig{300, 700}   // 通用人类交互间隔
    reactionTimeRange = delayConfig{300, 800}   // 人类反应时间
    hoverTimeRange    = delayConfig{100, 300}   // 鼠标悬停停留
    readTimeRange     = delayConfig{500, 1200}  // 阅读内容停留
    shortReadRange    = delayConfig{600, 1200}  // 短阅读停留
    scrollWaitRange   = delayConfig{100, 200}   // 滚动间微停顿
    postScrollRange   = delayConfig{300, 500}   // 滚动后等待
)
```

### 2.2 随机延迟函数

`feed_detail.go:329-336`:

```go
func sleepRandom(minMs, maxMs int) {
    if maxMs <= minMs {
        time.Sleep(time.Duration(minMs) * time.Millisecond)
        return
    }
    delay := time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond
    time.Sleep(delay)
}
```

使用 `rand.Intn()` 在 `[min, max)` 范围内产生随机延迟，每次调用结果不同。

### 2.3 各模块中的延迟分布

#### 评论加载 (`feed_detail.go`) — 约 20+ 处

| 场景 | 延迟范围 | 代码位置 |
|------|----------|----------|
| 初始滚动前 | 300-700ms | `feed_detail.go:158` |
| 检测到底部后 | 300-700ms | `feed_detail.go:212` |
| 点击按钮后的阅读模拟 | 500-1200ms | `feed_detail.go:232` |
| 第二轮点击后 | 600-1200ms | `feed_detail.go:240` |
| 滚动到最后评论后 | 300-500ms | `feed_detail.go:282` |
| 人类点击前的反应时间 | 300-800ms | `feed_detail.go:427` |
| 鼠标悬停后 | 100-300ms | `feed_detail.go:434` |
| 点击后阅读模拟 | 500-1200ms | `feed_detail.go:443` |
| 每次滚动的微停顿 | 100-200ms | `feed_detail.go:486` |
| 连续滚动间隔 | 300-700ms | `feed_detail.go:499` |
| 滚动失败后兜底 | 300-500ms | `feed_detail.go:505` |

#### 内容发布 (`publish.go`) — 约 15+ 处

| 场景 | 延迟 | 代码位置 |
|------|------|----------|
| 页面加载后 | 2s 固定 | `publish.go:50` |
| DOM 稳定后 | 1s 固定 | `publish.go:56` |
| 点击 Tab 后 | 1s 固定 | `publish.go:63` |
| Tab 点击重试间 | 200ms 固定 | `publish.go:125,130,137,143` |
| 每张图片上传后 | 1s 固定 | `publish.go:236` |
| 图片上传轮询间隔 | 500ms 固定 | `publish.go:252,267` |
| 标题输入后 | 500ms 固定 | `publish.go:283` |
| 正文输入前 | 1s 固定 | `publish.go:289` |
| 标签整体输入前 | 1s 固定 | `publish.go:427` |
| 方向键按下间隔 | 10ms 固定 | `publish.go:437` |
| 回车后 | 1s 固定 | `publish.go:448` |
| `#` 输入后 | 200ms 固定 | `publish.go:463` |
| **逐字输入间隔** | **50ms/字** | `publish.go:469` |
| 等待标签联想 | 1s 固定 | `publish.go:472` |
| 联想点击后 | 200ms + 500ms | `publish.go:491,493` |
| 下拉框点击后 | 500ms 固定 | `publish.go:603` |
| 选项点击后 | 200ms 固定 | `publish.go:620` |
| 原创弹窗等待 | 800ms 固定 | `publish.go:743` |
| 确认点击后 | 500ms 固定 | `publish.go:775` |
| 发布按钮点击后 | 3s 固定 | `publish.go:340` |

#### 视频发布 (`publish_video.go`) — 约 6 处

| 场景 | 延迟 | 代码位置 |
|------|------|----------|
| 页面加载后 | 2s | `publish_video.go:38` |
| DOM 稳定后 | 1s | `publish_video.go:43` |
| Tab 点击后 | 1s | `publish_video.go:49` |
| 发布按钮轮询间隔 | 1s | `publish_video.go:128` |
| 标题输入后 | 1s | `publish_video.go:143` |
| 正文和标签后 | 1s | `publish_video.go:157` |
| 发布后 | 3s | `publish_video.go:183` |

#### 评论操作 (`comment_feed.go`) — 约 10 处

| 场景 | 延迟 | 代码位置 |
|------|------|----------|
| DOM 稳定后 | 1s | `comment_feed.go:34` |
| 评论输入后 | 1s | `comment_feed.go:63` |
| 提交后 | 1s | `comment_feed.go:76` |
| 回复页 DOM 稳定后 | 1s | `comment_feed.go:93` |
| 等待评论容器加载 | 2s | `comment_feed.go:101` |
| 滚动到评论位置后 | 1s | `comment_feed.go:112` |
| 回复按钮点击后 | 1s | `comment_feed.go:126` |
| 回复内容输入后 | 500ms | `comment_feed.go:139` |
| 提交后 | 2s | `comment_feed.go:151` |
| 查找评论滚动后 | 300ms + 500ms | `comment_feed.go:218,227` |
| 查找评论轮询间隔 | 800ms | `comment_feed.go:269` |

#### 点赞 / 收藏 (`like_favorite.go`) — 约 5 处

| 场景 | 延迟 | 代码位置 |
|------|------|----------|
| 页面导航后 DOM 稳定 | 1s | `like_favorite.go:53` |
| 点赞点击后验证 | 3s | `like_favorite.go:110` |
| 第二次点赞点击后 | 2s | `like_favorite.go:124` |
| 收藏点击后验证 | 3s | `like_favorite.go:186` |
| 第二次收藏点击后 | 2s | `like_favorite.go:200` |

#### 登录 (`login.go`) — 约 3 处

| 场景 | 延迟 | 代码位置 |
|------|------|----------|
| 页面加载后 | 1s | `login.go:23` |
| 登录页加载后 | 2s | `login.go:44` |
| 二维码页加载后 | 2s | `login.go:66` |

---

## 3. 人类行为模拟

### 3.1 拟人化鼠标点击（完整链路）

`feed_detail.go:414-465` — `clickElementWithHumanBehavior()`:

```
步骤 1: 平滑滚动到目标元素 (scrollIntoView behavior:'smooth')
    ↓
步骤 2: 等待反应时间 (300-800ms 随机)
    ↓
步骤 3: 计算元素中心坐标，鼠标移动到该位置
    ↓
步骤 4: 鼠标悬停等待 (100-300ms 随机)
    ↓
步骤 5: 左键点击
    ↓
步骤 6: 模拟阅读停留 (500-1200ms 随机)
```

每一步都有随机延迟，整个链路模拟了真实用户的 "看到 → 反应 → 移动鼠标 → 悬停 → 点击 → 阅读" 过程。

### 3.2 鼠标位置随机化

`publish.go:111-114`:

```go
func clickEmptyPosition(page *rod.Page) {
    x := 380 + rand.Intn(100)  // X: 380-479 随机
    y := 20 + rand.Intn(60)    // Y: 20-79 随机
    page.Mouse.MustMoveTo(float64(x), float64(y)).MustClick(proto.InputMouseButtonLeft)
}
```

点击空白区域时不使用固定坐标，而是在一个区域范围内随机选取。

### 3.3 逐字符键盘输入

`publish.go:459-495` — 标签输入模拟：

```go
// 先输入 '#' 号
contentElem.Input("#")
time.Sleep(200 * time.Millisecond)  // 输入 # 后停顿

// 逐字符输入标签文字
for _, char := range tag {
    contentElem.Input(string(char))
    time.Sleep(50 * time.Millisecond)  // 每个字符间隔 50ms
}

time.Sleep(1 * time.Second)  // 等待联想下拉框出现
```

模拟人类打字速度（约 50ms/字符 ≈ 每分钟 240 字符），而不是一次性粘贴。

### 3.4 键盘导航模拟

`publish.go:429-448`:

```go
// 连按 20 次方向键下，每次间隔 10ms
for i := 0; i < 20; i++ {
    ka.Type(input.ArrowDown).Do()
    time.Sleep(10 * time.Millisecond)
}

// 按两次回车确认
ka.Press(input.Enter).Press(input.Enter).Do()
```

---

## 4. 智能滚动系统

### 4.1 多档速度 + 随机抖动

`feed_detail.go:338-347`:

```go
func getScrollInterval(speed string) time.Duration {
    switch speed {
    case "slow":
        return time.Duration(1200+rand.Intn(300)) * time.Millisecond  // 1200-1499ms
    case "fast":
        return time.Duration(300+rand.Intn(100)) * time.Millisecond   // 300-399ms
    default: // normal
        return time.Duration(600+rand.Intn(200)) * time.Millisecond   // 600-799ms
    }
}
```

每档速度都有随机抖动，不会产生固定频率。

### 4.2 滚动距离随机化

`feed_detail.go:530-536`:

```go
func calculateScrollDelta(viewportHeight int, baseRatio float64) float64 {
    // 基础距离 = 视口高度 × 基础比例 + 0~20% 随机方差
    scrollDelta := float64(viewportHeight) * (baseRatio + rand.Float64()*0.2)
    if scrollDelta < 400 {
        scrollDelta = 400  // 最小 400px
    }
    // 再叠加 ±50px 随机偏移
    return scrollDelta + float64(rand.Intn(100)-50)
}
```

滚动距离 = 视口 × (基础比例 + 0~20% 方差) ± 50px 偏移，确保每次滚动距离都不同。

速度档位对应的基础比例：

```go
func getScrollRatio(speed string) float64 {
    switch speed {
    case "slow":  return 0.5  // 50% 视口
    case "fast":  return 0.9  // 90% 视口
    default:      return 0.7  // 70% 视口
    }
}
```

### 4.3 人类化滚动行为

`feed_detail.go:469-517` — `humanScroll()`:

```
循环执行 pushCount 次 (1~5 次随机):
    1. 计算随机滚动距离
    2. window.scrollBy(0, delta)
    3. 微停顿 (100-200ms)
    4. 检测是否真的滚动了 (delta > 5)
    5. 如果还有下一次推送，等待 300-700ms

如果所有推送都没滚动成功:
    → 兜底: window.scrollTo(0, document.body.scrollHeight)
    → 等待 300-500ms
```

### 4.4 随机推送次数

`feed_detail.go:286-288`:

```go
largeMode := cl.state.stagnantChecks >= largeScrollTrigger  // 停滞 5 次触发
pushCount := 1
if largeMode {
    pushCount = 3 + rand.Intn(3)  // 3-5 次随机推送
}
```

### 4.5 WheelEvent 触发懒加载

`feed_detail.go:553-570`:

```go
func smartScroll(page *rod.Page, delta float64) {
    page.MustEval(`(delta) => {
        let targetElement = document.querySelector('.note-scroller')
            || document.querySelector('.interaction-container')
            || document.documentElement;

        // 构造真实的 WheelEvent，而非仅仅修改 scrollTop
        const wheelEvent = new WheelEvent('wheel', {
            deltaY: delta,
            deltaMode: 0,   // 像素模式
            bubbles: true,
            cancelable: true,
            view: window
        });
        targetElement.dispatchEvent(wheelEvent);
    }`, delta)
}
```

直接修改 `scrollTop` 不会触发某些页面的懒加载机制，而发送 `WheelEvent` 可以。

### 4.6 平滑滚动进入视图

`feed_detail.go:421-424`:

```go
el.MustEval(`() => {
    this.scrollIntoView({behavior: 'smooth', block: 'center'});
}`)
```

使用 `behavior: 'smooth'` 而非默认的瞬间跳转。

---

## 5. 智能自适应策略

### 5.1 停滞检测

`feed_detail.go:20-29` 定义了自适应常量：

```go
const (
    stagnantLimit          = 20  // 连续停滞 20 次后触发大冲刺
    minScrollDelta         = 10  // 判断"有效滚动"的最小像素
    stagnantCheckThreshold = 2   // 确认达标的停滞次数
    largeScrollTrigger     = 5   // 停滞 5 次后切换大滚动模式
    buttonClickInterval    = 3   // 每 3 次尝试点击一次按钮
)
```

### 5.2 自适应行为切换

`feed_detail.go:278-301` — 评论加载的滚动策略会根据停滞状态动态调整：

```
正常模式 (stagnantChecks < 5):
    → pushCount = 1, baseRatio = 0.7

大滚动模式 (stagnantChecks >= 5):
    → pushCount = 3~5, baseRatio = 1.4 (翻倍)

超时大冲刺 (stagnantChecks >= 20):
    → 一次性推送 10 次大滚动
    → 重置停滞计数器
```

### 5.3 每轮点击数随机化

`feed_detail.go:358`:

```go
maxClick := maxClickPerRound + rand.Intn(maxClickPerRound)  // 3~5 次随机
```

每轮最大点击按钮数量是随机的，避免固定模式。

### 5.4 重试机制 (retry-go + jitter)

项目使用 `avast/retry-go/v4` 实现带抖动的重试，分布在多个关键操作中：

#### 页面导航重试

`feed_detail.go:88-100`:

```go
retry.Do(
    func() error {
        page.MustNavigate(url)
        page.MustWaitDOMStable()
        return nil
    },
    retry.Attempts(3),
    retry.Delay(500*time.Millisecond),       // 基础延迟 500ms
    retry.MaxJitter(1000*time.Millisecond),  // 最大抖动 1000ms
)
```

实际等待时间 = 500ms + rand(0~1000ms) = 500~1500ms

#### 点击操作重试

`feed_detail.go:418-453`:

```go
retry.Do(
    func() error { /* 完整的悬停→点击→阅读链路 */ },
    retry.Attempts(3),
    retry.Delay(100*time.Millisecond),
    retry.MaxJitter(200*time.Millisecond),
)
```

#### DOM 查询重试

`feed_detail.go:589-606`, `618-636`, `648-681`, `716-745`, `811-835`:

所有 DOM 查询（滚动位置、评论计数、结束标记检测、数据提取）都包裹在 retry 中，每次最多 3 次重试，基础延迟 100-200ms + 200-300ms 抖动。

---

## 6. 会话持久化（Cookie 管理）

### 6.1 Cookie 生命周期

```
首次扫码登录
    ↓
浏览器获取 Cookie
    ↓
saveCookies() 序列化到 JSON 文件
    ↓
下次启动 → LoadCookies() 从文件读取
    ↓
headless_browser.WithCookies() 注入到浏览器
    ↓
无需再次扫码，直接使用已有会话
```

### 6.2 存储位置

`cookies.go:55-76`:

```go
func GetCookiesFilePath() string {
    // 1. 向后兼容: 旧路径 /tmp/cookies.json
    oldPath := filepath.Join(os.TempDir(), "cookies.json")
    if _, err := os.Stat(oldPath); err == nil {
        return oldPath
    }

    // 2. 环境变量: COOKIES_PATH
    if path := os.Getenv("COOKIES_PATH"); path != "" {
        return path
    }

    // 3. 默认: 当前目录下的 cookies.json
    return "cookies.json"
}
```

### 6.3 Cookie 注入

`browser.go:57-66`:

```go
cookiePath := cookies.GetCookiesFilePath()
cookieLoader := cookies.NewLoadCookie(cookiePath)

if data, err := cookieLoader.LoadCookies(); err == nil {
    opts = append(opts, headless_browser.WithCookies(string(data)))
}
```

`headless_browser.go:131-138` 中将 JSON 反序列化为 `proto.NetworkCookie` 并通过 `browser.MustSetCookies()` 注入。

### 6.4 防爬意义

- 避免每次操作都需要扫码登录，减少异常行为
- 维持稳定的浏览器会话，与真实用户行为一致
- Cookie 过期后才需要重新登录（通常几天到几周）

---

## 7. IP 代理支持

### 7.1 配置方式

`browser.go:51-55`:

```go
if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
    opts = append(opts, headless_browser.WithProxy(proxy))
    logrus.Infof("Using proxy: %s", maskProxyCredentials(proxy))
}
```

通过环境变量 `XHS_PROXY` 配置，支持 HTTP / HTTPS / SOCKS5 三种协议：

```bash
XHS_PROXY=http://proxy:8080 ./xiaohongshu-mcp
XHS_PROXY=socks5://user:pass@proxy:1080 ./xiaohongshu-mcp
```

### 7.2 凭证安全

`browser.go:25-36` — 日志中自动隐藏代理密码：

```go
func maskProxyCredentials(proxyURL string) string {
    u, _ := url.Parse(proxyURL)
    if u.User != nil {
        if _, hasPassword := u.User.Password(); hasPassword {
            u.User = url.UserPassword("***", "***")
        }
    }
    return u.String()
}
```

输入 `socks5://admin:secret@proxy:1080`，日志中显示 `socks5://***:***@proxy:1080`。

### 7.3 底层实现

`headless_browser.go:119-121`:

```go
if cfg.Proxy != "" {
    l = l.Proxy(cfg.Proxy)  // launcher.Proxy() → Chrome --proxy-server 参数
}
```

代理通过 Chrome 的 `--proxy-server` 启动参数设置，所有浏览器流量都经过代理。

---

## 8. 页面等待策略

### 8.1 多层等待

项目在页面导航后采用分层等待，确保 DOM 完全就绪：

```go
// 第一层: 等待页面基本加载
page.WaitLoad()

// 第二层: 固定等待（让异步资源加载）
time.Sleep(2 * time.Second)

// 第三层: 等待 DOM 稳定（无新增/变更节点）
page.WaitDOMStable(time.Second, 0.1)

// 第四层: 再次固定等待
time.Sleep(1 * time.Second)
```

代码示例 — `publish.go:42-56`:

```go
if err := pp.WaitLoad(); err != nil {
    logrus.Warnf("等待页面加载出现问题: %v，继续尝试", err)
}
time.Sleep(2 * time.Second)

if err := pp.WaitDOMStable(time.Second, 0.1); err != nil {
    logrus.Warnf("等待 DOM 稳定出现问题: %v，继续尝试", err)
}
time.Sleep(1 * time.Second)
```

### 8.2 等待 JavaScript 状态

```go
// 等待页面数据初始化完成
page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)
```

不盲目等待固定时间，而是检测页面的实际数据状态。

### 8.3 容错等待

等待失败不会直接中断流程，而是记录警告后继续（`logrus.Warnf` + 继续尝试），增强了健壮性。

---

## 9. 操作状态感知

### 9.1 点赞 / 收藏防重复

`like_favorite.go:90-106`:

```go
// 先读取当前状态
liked, _, err := a.getInteractState(page, feedID)

// 如果目标是点赞，但已经点赞了，跳过
if targetLiked && liked {
    logrus.Infof("feed %s already liked, skip clicking", feedID)
    return nil
}

// 如果目标是取消点赞，但本来就没点赞，跳过
if !targetLiked && !liked {
    logrus.Infof("feed %s not liked yet, skip clicking", feedID)
    return nil
}
```

**先检查状态再操作**，避免不必要的重复点击，减少操作频率。

### 9.2 操作后验证 + 重试

```go
// 第一次点击后等 3 秒
a.performClick(page, SelectorLikeButton)
time.Sleep(3 * time.Second)

// 验证状态
liked, _, err := a.getInteractState(page, feedID)
if liked == targetLiked {
    return nil  // 成功
}

// 状态未变化，再试一次
a.performClick(page, SelectorLikeButton)
time.Sleep(2 * time.Second)
```

---

## 10. 项目中尚未实现的防护

| 能力 | 现状 | 风险 |
|------|------|------|
| 请求频率限制 | 无全局限流 | 短时间大量操作可能触发风控 |
| IP 自动轮换 | 仅支持单一代理 | 同 IP 大量操作有封禁风险 |
| 浏览器指纹多样化 | 共用同一 UA 和浏览器实例 | 单账号场景影响小 |
| 验证码自动处理 | 未实现 | 遇到验证码会卡住 |
| User-Agent 随机轮换 | 固定 Chrome 124 UA | 可能被特征匹配 |
| Canvas / WebGL 指纹混淆 | 依赖 stealth 插件默认行为 | 高级指纹检测可能识别 |

---

## 附录：防护机制分布热力图

```
文件                       随机延迟  重试机制  行为模拟  状态检测
feed_detail.go              ●●●●●    ●●●●●    ●●●●●    ●●●
publish.go                  ●●●●     ○        ●●●      ○
publish_video.go            ●●       ○        ○        ○
comment_feed.go             ●●●      ○        ○        ●
like_favorite.go            ●●       ○        ○        ●●●
login.go                    ●        ○        ○        ●
search.go                   ○        ○        ○        ○
feeds.go                    ●        ○        ○        ○
navigate.go                 ○        ○        ○        ○
browser.go                  ○        ○        ○        ○
cookies.go                  ○        ○        ○        ○
headless_browser.go         ○        ○        ○        ○

● = 实现程度 (越多越完善)    ○ = 未实现
```

`feed_detail.go` 是防护最完善的文件，因为评论加载涉及大量滚动和交互，是最容易触发反爬的场景。
