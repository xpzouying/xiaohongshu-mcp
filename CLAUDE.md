# xiaohongshu-mcp

小红书 MCP 服务：用 [Camoufox](https://github.com/daijro/camoufox)（Firefox 反检测构建）做常驻浏览器，经 playwright-go（Juggler 协议）驱动，向 AI 助手暴露小红书的登录初始化、搜索、浏览和用户主页读取能力。只读，不含发布/评论/点赞/收藏/通知等写入。

## 常用命令

| 目的 | 命令 |
| --- | --- |
| 安装 Camoufox + playwright 驱动（首次必做，固定版本 + SHA-256 校验） | `go run ./cmd/camoufox-setup` |
| 启动服务（默认 `127.0.0.1:18060`） | `go run .` |
| 启动服务并显示浏览器窗口 | `go run . -headless=false` |
| 扫码登录（首次必做） | `go run ./cmd/login` |
| 编译二进制 | `go build .` |
| 格式化 | `gofmt -w .` |
| 运行测试 | `go test ./...` |

MCP 端点为 `http://localhost:18060/mcp`。多数集成测试依赖真实浏览器与登录态，默认 `t.Skip`。

**供应链约束**：服务运行时绝不自动下载浏览器或驱动。Camoufox 由 `cmd/camoufox-setup`
固定版本、SHA-256 校验后落在 `bin/camoufox`（gitignored）；playwright 驱动（node +
playwright-core）落在 `.playwright-driver`。解析失败一律 fail closed（见
`browser/install.go`、`browser/driver.go`），可用 `XHS_CAMOUFOX_BIN` /
`PLAYWRIGHT_DRIVER_PATH` / `PLAYWRIGHT_NODEJS_PATH` 显式指定。

## 代码结构

- `main.go`、`app_server.go`、`routes.go` —— 进程入口与 HTTP 服务装配
- `mcp_server.go`、`mcp_handlers.go` —— MCP 工具的定义与处理
- `service.go`、`handlers_api.go` —— 业务服务层与 HTTP API
- `browser_session.go` —— 常驻单 Camoufox + 进程内串行 page lease（关 page 不关浏览器；无人为限速）
- `browser/` —— Camoufox 定位/校验（`install.go`）、驱动解析（`driver.go`）、指纹配置（`fingerprint.go`）、playwright-go 引擎（`engine.go`）、cookie 转换（`cookies.go`）
- `humanize/` —— 拟人化鼠标/键盘/滚动输入（基于 playwright Mouse/Keyboard）
- `xiaohongshu/` —— 基于 playwright-go 的读取动作（搜索 / 详情 / 评论读取 / 登录 / 主页）
- `configs/`、`cookies/`、`errors/` —— 配置、登录态、错误

## 开发约定

- 每次改完 Go 代码后执行 `gofmt` 格式化
- 功能改动一律走独立分支；未经同意不推送远程
- 交付顺序：先本地 review，再远程 PR review
- 测试产生的临时脚本与构建中间文件，无用即删
- 不过度设计，保持代码简洁、易读
- 注释用中文，简洁明了，专业名词可用英文
- 错误信息（`error` / `panic`）用英文并遵循 Go 惯例：小写开头、结尾无标点；日志与注释保持中文

## 发版规范

- 发版=打语义化 tag `vX.Y.Z` 推上去（触发 Release），破坏性变更进 major；main 每次推送自动生成的日期 tag `vYYYY.MM.DD.HHMM-sha` 不是发版，别跟它混。

## PR Review 重点

- 反检测由 Camoufox 在 C++ 层完成，**不要再注入 stealth JS 或事后覆盖 UA/Client Hints**（那是 go-rod 时代的做法，已废弃）。
- 指纹/语言/屏幕等经 `CAMOU_CONFIG_<n>` 环境变量传入（见 `browser/fingerprint.go`），新增配置项走这里，不要加 `page.Eval` 注入。
- 读取动作优先用 playwright 原生能力（`page.Evaluate` / `QuerySelector` / `Mouse`），只有 `__INITIAL_STATE__` 数据提取才在页面内 evaluate。
