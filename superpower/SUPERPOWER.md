# 🦸 Superpower 方案指南

> 用 browser-use + devtools-protocol 突破 xiaohongshu-mcp 的技术卡点

---

## 📋 背景

xiaohongshu-mcp 项目之前卡在三个问题：
1. **CSP 铜墙铁壁** — 5 种绕过方案全部失败
2. **API 逆向走不通** — 端点隐藏 + 签名机制
3. **UI 自动化太慢** — 服务器端浏览器性能差

**结论**：只能走 UI 自动化（点按钮、输文字），效率低且脆弱。

---

## 🎯 Superpower 核心思路

```
devtools-protocol (CDP)
    ├── Page.setBypassCSP → 绕过 CSP 限制
    ├── Network.domain → 拦截/监听真实 API 请求
    └── Runtime.evaluate → 在浏览器内执行任意 JS

browser-use (AI Agent)
    ├── 自然语言指令 → 自动理解页面
    ├── AI 选择器 → 不怕页面改版
    └── 决策层 → 选择最佳操作方式

两者结合 = 降维打击
```

---

## 🔧 阶段 1：CSP Bypass 验证（最关键）

### 原理

CDP 有一个鲜为人知的命令：

```json
{
  "method": "Page.setBypassCSP",
  "params": { "enabled": true }
}
```

这个命令告诉 Chrome：**对这个标签页禁用 CSP 策略**。

一旦 CSP 被禁用，浏览器内的 `fetch()` 和 `XMLHttpRequest` 就不再被拦截，可以直接调用小红书内部 API。

### 操作步骤

```bash
# 1. 启动 Chrome（调试模式）
google-chrome --remote-debugging-port=9222
# 或使用项目里的 chrome_launcher.py

# 2. 手动打开小红书并登录
# 访问 https://www.xiaohongshu.com

# 3. 运行验证脚本
cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/superpower
python3 csp_bypass_test.py
```

### 预期结果

- **成功**：CSP Bypass 后，`fetch()` 能拿到 API 数据
- **失败**：需要排查 Chrome 版本（该命令需要 Chrome 73+）

### ⚠️ 关键细节

`Page.setBypassCSP` 有几个重要特性：
1. **只对当前标签页生效** — 新开的标签页不受影响
2. **导航后仍然有效** — 刷新页面后 CSP 依然被禁用 ✅
3. **不影响其他安全策略** — 只关掉 CSP

---

## 🔧 阶段 2：API 逆向自动化

CSP 打通后，下一步就是**自动发现**小红书的真实 API 端点。

### 方案 A：CDP 网络拦截

```python
# 用 CDP 监听所有网络请求
cdp_send(ws, "Network.enable")

# 之后会收到 Network.requestWillBeSent 事件
# 包含所有请求的 URL、headers、payload
```

**操作流程**：
1. 启用 `Page.setBypassCSP`
2. 启用 `Network.enable`
3. 手动在页面上操作（点击收藏、创建专辑等）
4. CDP 自动捕获所有 API 请求
5. 分析请求结构，提取端点、参数、签名

### 方案 B：浏览器内拦截

```javascript
// 通过 Runtime.evaluate 注入请求拦截器
const originalFetch = window.fetch;
window.fetch = async function(...args) {
    const response = await originalFetch.apply(this, args);
    const clone = response.clone();
    const data = await clone.json();
    console.log('API:', args[0], data);
    return response;
};
```

---

## 🔧 阶段 3：browser-use 集成

当 API 端点确认后，用 browser-use 做**智能决策层**：

```python
from browser_use import Agent, Browser

# 自然语言指令
agent = Agent(
    task="登录小红书，进入创作者中心，创建专辑'技术干货'，"
         "把收藏的10篇笔记移进去",
    browser=Browser()
)

result = await agent.run()
```

### 为什么用 browser-use？

| 维度 | 现有方案 | browser-use |
|------|---------|-------------|
| 选择器维护 | 硬编码 CSS，改版就崩 | AI 自动识别 |
| 异常处理 | 手写 try/catch | AI 自适应 |
| 开发速度 | 需要逆向 + 写脚本 | 自然语言描述 |
| 鲁棒性 | 低 | 高 |

---

## 🔧 阶段 4：完整架构

```
┌──────────────────────────────────────────────────┐
│                  用户指令层                        │
│   "把收藏的技术笔记整理到'技术'专辑"               │
├──────────────────────────────────────────────────┤
│               browser-use (决策层)                 │
│   AI 理解意图 → 生成操作计划                       │
├──────────────┬───────────────────┬────────────────┤
│  API 直调    │   UI 自动化       │   混合模式      │
│  (快)        │   (稳)            │   (最佳)        │
│              │                   │                │
│  CSP Bypass  │  CDP DOM 操作     │  AI 判断哪个   │
│  → fetch     │  → 点击/输入      │  更可靠就用    │
│  调用 API    │                   │  哪个           │
├──────────────┴───────────────────┴────────────────┤
│              CDP (执行层)                          │
│  Page.setBypassCSP / Network / Runtime / DOM      │
├──────────────────────────────────────────────────┤
│              Chrome Browser                        │
└──────────────────────────────────────────────────┘
```

---

## 📂 文件结构

```
superpower/
├── csp_bypass_test.py      # 阶段1: CSP绕过验证脚本
├── api_discover.py          # 阶段2: API发现脚本 (TODO)
├── xhs_agent.py             # 阶段3: browser-use集成 (TODO)
└── SUPERPOWER.md            # 本指南
```

---

## 🚀 行动清单

### 立即执行
- [x] 写 CSP Bypass 验证脚本 (`csp_bypass_test.py`)
- [ ] 运行验证脚本，确认 CSP 可以被绕过
- [ ] 如果成功，记录 API 调用结果

### 下一步
- [ ] 写 API 发现脚本，自动捕获小红书 API 端点
- [ ] 整理捕获的 API，建立端点文档
- [ ] 用 browser-use 重写专辑同步流程
- [ ] 集成到现有的 xiaohongshu-mcp 服务器

### 长期
- [ ] 通用化：做成可复用的 CDP 自动化框架
- [ ] 发布：如果效果显著，可以做成开源工具或教程

---

## 💡 关键洞察

1. **Page.setBypassCSP 是破局关键** — 之前 5 种方案失败是因为都在 CSP 生效的环境下尝试。这个 CDP 命令直接从浏览器引擎层面关掉 CSP，是降维打击。

2. **browser-use 解决维护性问题** — 即使 UI 自动化是唯一选择，browser-use 的 AI 选择器也比硬编码 CSS 强 100 倍。

3. **混合方案最优** — 能用 API 直调的地方用 API（快），不能的地方用 UI 自动化（稳），AI 自动决策。

---

*最后更新: 2026-04-30*
