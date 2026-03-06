---
name: add-http-endpoint
description: 新增或修改 HTTP API 端点的标准流程
---

# Add HTTP Endpoint

## Checklist

1. 定义请求/响应结构（含错误码）
2. 添加/修改 handler 与 service 逻辑
3. 增补单测与失败路径测试
4. 更新 `docs/API.md`（中英文同步）
5. 执行 `/.codex/commands/verify.sh`

## Done Criteria

- API 可调用
- 失败路径可诊断
- 文档示例可复现
