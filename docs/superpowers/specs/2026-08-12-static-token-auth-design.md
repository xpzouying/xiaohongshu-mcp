# 静态 Token 鉴权设计

## 目标

为 HTTP API 和 MCP Streamable HTTP 端点增加可选的静态 Token 鉴权。未配置 Token 时保持现有行为；配置 Token 后，客户端必须通过标准 Bearer Token 访问受保护端点。

## 配置

- 新增启动参数 `-token`。
- 新增环境变量 `AUTH_TOKEN`。
- `-token` 的默认值取自 `AUTH_TOKEN`，因此显式启动参数优先。
- 最终 Token 为空时关闭鉴权，保证向后兼容。
- 日志和错误响应均不输出 Token。

## 路由与中间件

新增 Gin 中间件 `authMiddleware(token string)`，并将以下端点放入同一个受保护路由组：

- `/api/v1/*`
- `/mcp`
- `/mcp/*path`

`/health` 保持公开，用于 Docker 和编排平台探活。CORS 中间件在鉴权中间件之前执行，`OPTIONS` 预检请求继续直接返回成功，不要求 Token。

当 Token 为空时，中间件直接放行。Token 非空时，只接受以下请求头：

```http
Authorization: Bearer <token>
```

服务端使用常量时间比较校验 Token，避免通过比较耗时泄露信息。

## 错误处理

请求头缺失、格式错误或 Token 不匹配时：

- 返回 HTTP `401 Unauthorized`；
- 设置 `WWW-Authenticate: Bearer`；
- 返回项目统一格式的 JSON 错误；
- 不把配置值或请求中的 Token 写入响应和日志。

## Docker 支持

- Dockerfile 声明 `AUTH_TOKEN` 的空默认值，不在镜像中写入真实密钥。
- `docker/docker-compose.yml` 使用 `${AUTH_TOKEN:-}` 从宿主环境透传。
- 未设置宿主环境变量时，容器继续以无鉴权模式运行。
- Docker 文档说明如何通过 `docker run -e AUTH_TOKEN=...` 和 Compose 配置 Token。

## 文档

更新以下文档：

- `README.md`
- `README_EN.md`
- `docker/README.md`

文档包含二进制、源码、Docker、HTTP curl 和 MCP 客户端的 Bearer Token 配置示例。

## 测试

单元及路由测试覆盖：

- 未配置 Token 时受保护端点正常放行；
- 正确 Bearer Token 放行；
- 请求头缺失、格式错误或 Token 错误时返回 401；
- `/health` 在启用鉴权时仍然公开；
- HTTP API 与 MCP 路由均受鉴权保护；
- CORS `OPTIONS` 预检请求不受鉴权阻断。

实现遵循现有项目结构，不引入外部鉴权依赖，也不扩展到 OAuth、Token 轮换或多 Token 管理。
