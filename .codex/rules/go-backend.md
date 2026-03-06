# Go Backend Rules

- 修改 Go 代码后必须执行 `gofmt`。
- 新增错误分支必须返回可诊断信息（错误码/上下文）。
- HTTP Handler 改动应同步更新 `docs/API.md`。
- 对外行为变更需要补充示例请求与响应。
