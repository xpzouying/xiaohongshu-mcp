# xiaohongshu-mcp

小红书 MCP 服务：用 go-rod 驱动常驻浏览器，向 AI 助手暴露小红书的发布、搜索、浏览、互动等能力。

## 常用命令

| 目的 | 命令 |
| --- | --- |
| 启动服务（默认 `127.0.0.1:18060`） | `go run .` |
| 启动服务并显示浏览器窗口 | `go run . -headless=false` |
| 扫码登录（首次必做） | `go run cmd/login/main.go` |
| 编译二进制 | `go build .` |
| 格式化 | `gofmt -w .` |
| 运行测试 | `go test ./...` |

MCP 端点为 `http://localhost:18060/mcp`。多数集成测试依赖真实浏览器与登录态，默认 `t.Skip`。

## 代码结构

- `main.go`、`app_server.go`、`routes.go` —— 进程入口与 HTTP 服务装配
- `mcp_server.go`、`mcp_handlers.go` —— MCP 工具的定义与处理
- `service.go`、`handlers_api.go` —— 业务服务层与 HTTP API
- `browser_pool.go` —— 常驻浏览器 + 有限并发 tab 池，分级超时
- `xiaohongshu/` —— 基于 go-rod 的各业务动作（发布 / 搜索 / 详情 / 评论 / 点赞收藏 / 登录 / 主页）
- `configs/`、`cookies/`、`errors/`、`pkg/`（`xhsutil` 标题、`downloader` 图片） —— 配置、登录态、错误与工具

## 开发约定

- 每次改完 Go 代码后执行 `gofmt` 格式化
- 功能改动一律走独立分支；未经同意不推送远程
- 交付顺序：先本地 review，再远程 PR review
- 测试产生的临时脚本与构建中间文件，无用即删
- 不过度设计，保持代码简洁、易读
- 注释用中文，简洁明了，专业名词可用英文
- 错误信息（`error` / `panic`）用英文并遵循 Go 惯例：小写开头、结尾无标点；日志与注释保持中文

## 发版规范

- 发版=打语义化 tag `vX.Y.Z` 推上去（触发 Release 与 Docker 镜像），破坏性变更进 major；main 每次推送自动生成的日期 tag `vYYYY.MM.DD.HHMM-sha` 不是发版，别跟它混。

## PR Review 重点

- 警惕大量 JS 注入：若某处 `page.Eval` 等 JS 注入并非必要、可用 go-rod 原生行为（点击、输入、查找元素）替代，直接评论要求改用 go-rod
