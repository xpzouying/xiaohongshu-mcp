# MCP 与 LAN Web UI 安全部署

本文只描述模板、隔离验证和回滚流程。未经独立 review，不得把本分支应用到生产；示例不包含真实凭据，也不会把 secret 写入镜像、环境变量值或仓库。

## 1. 目标边界与流量路径

- Web UI 默认发布在 `0.0.0.0:18080`，供 LAN 浏览器访问。
- MCP 默认仅发布在 `127.0.0.1:18060`，LAN 无法直连；迁移稳定后可从 Compose 完全删除 MCP 的 `ports`。
- Web UI 不经过 host publish 访问 MCP，而是通过 `xhs` bridge 的容器 DNS 请求 `http://xiaohongshu-mcp:18060`。
- `xhs` 不能是 MCP 的唯一 `internal: true` 网络。MCP 需要通过该非 internal bridge 的默认网关访问小红书。
- 如果以后新增 internal backend 网络，Web UI 与 MCP 可在该网络通信，但 MCP 必须再连接一个非 internal egress 网络。流量路径应为：`LAN -> Web UI -> internal backend -> MCP -> egress -> 小红书`。

当前模板未引入 internal 网络，避免误断 MCP 出站。

## 2. Secret 与鉴权模式

三种模式必须分别配置，不能混用宿主机路径：

- current：`XHS_READ_TOKEN_FILE_HOST`、`XHS_WRITE_TOKEN_FILE_HOST`、`XHS_ADMIN_TOKEN_FILE_HOST`，Web UI 另加 `WEBUI_PASSWORD_FILE_HOST`。
- overlap：current 全部文件，再加三个对应的 `*_PREVIOUS_FILE_HOST`。
- legacy：仅 `XHS_API_TOKEN_FILE_HOST`，Web UI 另加 `WEBUI_PASSWORD_FILE_HOST`；必须使用独立 legacy Compose，不能叠加基础 Compose。

所有文件必须在仓库外且权限为 `0400` 或 `0600`。`*_FILE_HOST` 只保存路径；secret 内容仍只存在文件中，并以只读 Compose secret 挂载到 `/run/secrets/*`。

本地 Compose secret 实际使用 bind mount；Docker Compose v5 不会应用 long syntax 中的 `uid`、`gid`、`mode`。Web UI 镜像因此固定使用非 root 的 `uid=1000,gid=1000`，与本文由普通宿主用户创建的 `0400` secret 所有者一致。创建文件后必须用 `stat -c '%u:%g %a'` 确认两个文件均为 `1000:1000 400`；不要通过 `chmod 0444`、`chmod 0644` 或改为 root 运行来规避不可读问题。若部署主机的专用运行账号不是 uid/gid 1000，应先由基础设施管理员建立 uid/gid 映射或采用受控的 secret 管理方案，不要放宽文件权限。

不得把真实 secret 放入 `.env`、Compose、镜像构建参数、仓库、命令行参数、日志或错误响应。

后端使用 `XHS_AUTH_MODE`，Web UI 使用 `WEBUI_AUTH_MODE`，均支持：

- `off`：关闭鉴权，只允许在隔离测试中使用；仍需配置 token 文件，避免误配置时静默裸奔。
- `warn`：记录未认证访问但暂不拒绝，仅用于短期迁移；日志不得记录凭据。
- `enforce`：强制鉴权，默认值和生产最终状态。

生产必须同时满足：两个 auth mode 都为 `enforce`、`XHS_ALLOW_INSECURE_TEST_MODE=false`、`WEBUI_INSECURE_TEST_MODE=false`、所选模式的 secret 缺失时 fail closed。legacy 仅限已批准且不超过 24 小时的回滚窗口。两个 insecure 开关只允许在隔离测试中显式设为 `true`；`warn` 与 `off` 不是生产降级方案。

## 3. 生产模板的安全默认值

模板默认值如下，渲染时仍应显式核对：

| 项目 | 安全默认值 |
| --- | --- |
| Web UI publish | `0.0.0.0:18080 -> 8080` |
| MCP publish | `127.0.0.1:18060 -> 18060` |
| 容器通信 | `xiaohongshu-mcp:18060` over `xhs` |
| MCP 网络 | 非 internal，保留 egress |
| 后端鉴权 | `XHS_AUTH_MODE=enforce` |
| Web UI 鉴权 | `WEBUI_AUTH_MODE=enforce` |
| insecure 测试开关 | `XHS_ALLOW_INSECURE_TEST_MODE=false`、`WEBUI_INSECURE_TEST_MODE=false` |
| Secret | 必填的仓库外文件，以只读 Compose secret 挂载 |
| 后端 CORS | `XHS_CORS_ALLOWED_ORIGINS` 默认为空，仅接受逗号分隔的精确 Origin |

迁移稳定且确认所有调用都走容器网络后，可删除 `docker/docker-compose.yml` 中 MCP 的整个 `ports` 段；不要把它改回 LAN bind。

## 4. 仅渲染配置

以下命令不会创建或重启容器。示例文件必须存在、权限为 `0400` 或 `0600`，且不能使用生产 secret。

```bash
cd /path/to/xiaohongshu-mcp

# current
export XHS_READ_TOKEN_FILE_HOST=/secure/current/read
export XHS_WRITE_TOKEN_FILE_HOST=/secure/current/write
export XHS_ADMIN_TOKEN_FILE_HOST=/secure/current/admin
export WEBUI_PASSWORD_FILE_HOST=/secure/webui/password
docker compose -f docker/docker-compose.yml config > /tmp/xhs-mcp.current.yml
docker compose -f docker-compose.webui.yml config > /tmp/xhs-webui.current.yml

# overlap（保留上面的 current 变量）
export XHS_READ_TOKEN_PREVIOUS_FILE_HOST=/secure/previous/read
export XHS_WRITE_TOKEN_PREVIOUS_FILE_HOST=/secure/previous/write
export XHS_ADMIN_TOKEN_PREVIOUS_FILE_HOST=/secure/previous/admin
docker compose -f docker/docker-compose.yml -f docker/docker-compose.overlap.yml config > /tmp/xhs-mcp.overlap.yml

# legacy：新 shell 中清空全部 current/previous 变量，只设置单 token
env -u XHS_READ_TOKEN_FILE_HOST -u XHS_WRITE_TOKEN_FILE_HOST -u XHS_ADMIN_TOKEN_FILE_HOST \
  -u XHS_READ_TOKEN_PREVIOUS_FILE_HOST -u XHS_WRITE_TOKEN_PREVIOUS_FILE_HOST \
  -u XHS_ADMIN_TOKEN_PREVIOUS_FILE_HOST \
  XHS_API_TOKEN_FILE_HOST=/secure/legacy/api \
  docker compose -f docker/docker-compose.legacy.yml config > /tmp/xhs-mcp.legacy.yml
env -u XHS_READ_TOKEN_FILE_HOST -u XHS_WRITE_TOKEN_FILE_HOST -u XHS_ADMIN_TOKEN_FILE_HOST \
  -u XHS_READ_TOKEN_PREVIOUS_FILE_HOST -u XHS_WRITE_TOKEN_PREVIOUS_FILE_HOST \
  -u XHS_ADMIN_TOKEN_PREVIOUS_FILE_HOST \
  XHS_API_TOKEN_FILE_HOST=/secure/legacy/api WEBUI_PASSWORD_FILE_HOST=/secure/webui/password \
  docker compose -f docker-compose.webui.legacy.yml config > /tmp/xhs-webui.legacy.yml
```

人工核对渲染结果：

```bash
docker compose -f docker/docker-compose.yml config --services
docker compose -f docker/docker-compose.yml -f docker/docker-compose.overlap.yml config --services
docker compose -f docker/docker-compose.legacy.yml config --services
docker compose -f docker-compose.webui.legacy.yml config --services
```

必须确认：MCP host IP 是 `127.0.0.1`、Web UI host IP 是 `0.0.0.0`、两边网络名相同且 `internal` 不是 `true`、secret source 是预期宿主机路径、容器内 target 位于 `/run/secrets/` 且为只读 secret。

## 5. 隔离环境启动

不得复用生产端口、容器名、网络、数据目录、账号或 secret。下面使用本地构建镜像、`28060/28080`、独立 bridge 和临时空数据目录：

```bash
cd /path/to/xiaohongshu-mcp
TEST_ROOT=$(mktemp -d /tmp/xhs-security-test.XXXXXX)
install -d -m 0700 "$TEST_ROOT/secrets" "$TEST_ROOT/data" "$TEST_ROOT/images"
openssl rand -hex 32 > "$TEST_ROOT/secrets/read"
openssl rand -hex 32 > "$TEST_ROOT/secrets/write"
openssl rand -hex 32 > "$TEST_ROOT/secrets/admin"
openssl rand -hex 24 > "$TEST_ROOT/secrets/webui_password"
chmod 0400 "$TEST_ROOT/secrets/"*

docker build -t xiaohongshu-mcp:security-local .
docker build -f webui/Dockerfile -t xiaohongshu-webui:security-local .

export XHS_MCP_IMAGE=xiaohongshu-mcp:security-local
export WEBUI_IMAGE=xiaohongshu-webui:security-local
export XHS_MCP_CONTAINER_NAME=xhs-mcp-security-test
export WEBUI_CONTAINER_NAME=xhs-webui-security-test
export XHS_NETWORK_NAME=xhs-security-test
export XHS_MCP_BIND_ADDRESS=127.0.0.1
export XHS_MCP_HOST_PORT=28060
export WEBUI_BIND_ADDRESS=0.0.0.0
export WEBUI_HOST_PORT=28080
export XHS_DATA_DIR_HOST="$TEST_ROOT/data"
export XHS_IMAGES_DIR_HOST="$TEST_ROOT/images"
export XHS_READ_TOKEN_FILE_HOST="$TEST_ROOT/secrets/read"
export XHS_WRITE_TOKEN_FILE_HOST="$TEST_ROOT/secrets/write"
export XHS_ADMIN_TOKEN_FILE_HOST="$TEST_ROOT/secrets/admin"
export WEBUI_PASSWORD_FILE_HOST="$TEST_ROOT/secrets/webui_password"
export XHS_AUTH_MODE=enforce
export WEBUI_AUTH_MODE=enforce
export XHS_ALLOW_INSECURE_TEST_MODE=false
export WEBUI_INSECURE_TEST_MODE=false
export WEBUI_USERNAME=test-admin
export XHS_CORS_ALLOWED_ORIGINS='http://127.0.0.1:28080'

docker compose -p xhs-security-mcp -f docker/docker-compose.yml up -d --no-build
docker compose -p xhs-security-webui -f docker-compose.webui.yml up -d --no-build
```

这两个 Compose 文件共享由后端创建的 `xhs-security-test` bridge；Web UI 将其作为 external network 使用。启动前先用 `docker ps` 和 `docker network inspect` 确认隔离名称没有与生产对象重名。

overlap 启动时额外创建三份 previous 文件并设置三个 `*_PREVIOUS_FILE_HOST`，后端命令改为 `-f docker/docker-compose.yml -f docker/docker-compose.overlap.yml`。legacy 启动必须在清空全部 current/previous 变量后，设置单独的 `XHS_API_TOKEN_FILE_HOST`，并分别使用 `docker/docker-compose.legacy.yml` 与 `docker-compose.webui.legacy.yml`；不得与基础 Compose 叠加。legacy 窗口最长 24 小时。

## 6. 端口、网络与鉴权验证

以下命令只针对上节的隔离对象：

```bash
# 容器和 host publish
docker compose -p xhs-security-mcp -f docker/docker-compose.yml ps
docker compose -p xhs-security-webui -f docker-compose.webui.yml ps
docker port xhs-mcp-security-test
docker port xhs-webui-security-test

# bridge、容器 DNS 与 MCP egress
docker network inspect xhs-security-test
docker exec xhs-webui-security-test wget -qO- http://xiaohongshu-mcp:18060/health
docker exec xhs-mcp-security-test getent hosts xiaohongshu.com

# Web UI：health 公开，其余页面/静态/API 未认证应为 401
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:28080/api/web/health
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:28080/
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:28080/static/app.js
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:28080/api/web/accounts

# 使用 stdin 配置，避免把凭据展开到进程参数或日志。
curl_basic() {
  { printf 'user = "test-admin:'; tr -d '\n' < "$WEBUI_PASSWORD_FILE_HOST"; printf '"\n'; } |
    curl --config - "$@"
}
curl_bearer() {
  case "$(stat -c '%a' "$XHS_READ_TOKEN_FILE_HOST")" in 400|600) ;; *) return 1 ;; esac
  { printf 'header = "Authorization: Bearer '; tr -d '\r\n' < "$XHS_READ_TOKEN_FILE_HOST"; printf '"\n'; } |
    curl --config - "$@"
}

# Basic 成功后页面可用。
curl_basic --fail http://127.0.0.1:28080/ >/dev/null

# MCP：health 最小公开；无/错 token 为 401；正确 token 才能访问受保护 REST/MCP。
curl --fail http://127.0.0.1:28060/health
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:28060/api/v1/login/status
curl -sS -o /dev/null -w '%{http_code}\n' \
  -H 'Authorization: Bearer deliberately-wrong-token' \
  http://127.0.0.1:28060/api/v1/login/status
curl_bearer --fail http://127.0.0.1:28060/api/v1/login/status >/dev/null

# CORS：值必须是带 scheme 和可选端口的精确 Origin，不能包含路径、通配符或凭据。
# 多个 Origin 使用逗号分隔，例如：
# XHS_CORS_ALLOWED_ORIGINS='https://admin.example.com,https://ops.example.com:8443'
# 默认空值不得出现 allow-origin；只在精确 allowlist 命中时回显对应 Origin。
curl -si -X OPTIONS -H 'Origin: https://cross-site.invalid' \
  -H 'Access-Control-Request-Method: POST' \
  http://127.0.0.1:28060/api/v1/feeds/search
curl_bearer -si -X OPTIONS -H 'Origin: http://127.0.0.1:28080' \
  -H 'Access-Control-Request-Method: POST' \
  http://127.0.0.1:28060/api/v1/feeds/search

# CSRF：跨站、cross-site metadata 和无 Origin 的受保护 POST 均应被拒绝。
curl_basic -si \
  -X POST -H 'Origin: https://cross-site.invalid' \
  http://127.0.0.1:28080/api/web/feeds/search
curl_basic -si \
  -X POST -H 'Sec-Fetch-Site: cross-site' \
  http://127.0.0.1:28080/api/web/feeds/search
```

overlap 验证 current 与 previous 对应 scope 均通过，其他 scope 均为 403。legacy 验证单 token 可访问三类 scope。撤销 legacy 时先停止旧客户端，切回 current 三文件并重新 render，使用基础 Compose 重建两个服务；确认 current token 可用后删除 legacy 挂载和文件。若切回失败，停止新容器、恢复 legacy 两份独立 Compose；仍须遵守总计不超过 24 小时的窗口，且不得删除数据卷。

还必须运行项目门禁：

```bash
gofmt -d app_server.go handlers_api.go middleware.go routes.go security.go \
  webui/main.go webui/security.go
go test ./...
go test -race ./...
go vet ./...
node --test webui/static/*.test.js
git diff --check
```

验证日志时只检查状态和错误类别，不输出 secret：

```bash
docker logs --tail 200 xhs-mcp-security-test 2>&1 | \
  sed -E 's/((token|secret|password)[=: ]+)[^ ]+/\1***REDACTED***/Ig'
docker logs --tail 200 xhs-webui-security-test 2>&1 | \
  sed -E 's/((token|secret|password)[=: ]+)[^ ]+/\1***REDACTED***/Ig'
```

## 7. Secret 缺失与 fail-closed 验证

只在隔离环境执行。停止目标容器、取消一个 secret 的挂载或改为不存在的测试路径后重新渲染/启动，服务必须拒绝受保护请求或无法启动；不得静默变成无鉴权模式。恢复临时 secret 后再继续测试。

`off`、`warn`、`XHS_ALLOW_INSECURE_TEST_MODE=true` 和 `WEBUI_INSECURE_TEST_MODE=true` 必须由测试人员在隔离环境显式设置，且测试结束立即恢复 `enforce` 和 `false`。不能通过放宽 Origin、CORS、Basic 或 Bearer 校验来“修复”测试。

## 8. 隔离环境清理

```bash
docker compose -p xhs-security-webui -f docker-compose.webui.yml down --remove-orphans
docker compose -p xhs-security-mcp -f docker/docker-compose.yml down --remove-orphans
rm -rf -- "$TEST_ROOT"
```

只允许删除明确命名的隔离对象。即使是隔离环境也不要使用 `down -v`，以免形成会被复制到生产操作的危险习惯。

## 9. 生产迁移前记录

任何生产变更前先保存可回滚元数据；不要在记录中包含 secret 内容：

```bash
ROLLBACK_DIR=/path/to/backup/xhs-$(date +%Y%m%d_%H%M%S)
install -d -m 0700 "$ROLLBACK_DIR"
docker inspect xiaohongshu-mcp > "$ROLLBACK_DIR/mcp-inspect.json"
docker inspect xiaohongshu-webui > "$ROLLBACK_DIR/webui-inspect.json"
docker inspect xiaohongshu-mcp --format '{{.Config.Image}} {{.Image}}' \
  > "$ROLLBACK_DIR/mcp-image.txt"
docker inspect xiaohongshu-webui --format '{{.Config.Image}} {{.Image}}' \
  > "$ROLLBACK_DIR/webui-image.txt"
docker inspect xiaohongshu-mcp --format '{{json .Mounts}}' \
  > "$ROLLBACK_DIR/mcp-mounts.json"
docker compose -f docker/docker-compose.yml config \
  > "$ROLLBACK_DIR/mcp-compose.rendered.yml"
docker compose -f docker-compose.webui.yml config \
  > "$ROLLBACK_DIR/webui-compose.rendered.yml"
```

另外备份原 Compose 文件，并记录原镜像 digest/tag、原端口映射、所有 named volume 名称与 bind mount 绝对路径。数据备份方式应与当前存储类型匹配；不得把“有旧镜像”误当成“已有数据备份”。

## 10. 无损回滚

出现阻断问题时按以下顺序回滚：

1. 停止新 Web UI 和 MCP 容器，避免继续写入；不得执行 `docker compose down -v`、`docker volume rm` 或删除 bind mount 目录。
2. 恢复上一版 Compose 文件，并把镜像固定为迁移前记录的 tag/digest。
3. 恢复旧端口映射。若旧客户端不支持 token，可临时恢复 `18060:18060`，但这会重新暴露 LAN 写入口，必须限定回滚窗口并尽快重新收口。
4. 按 `mcp-mounts.json` 重新挂载完全相同的原 named volume 或 bind mount 路径。不要初始化空卷，不要复制覆盖原数据，不要改变 volume 名称。
5. 先启动旧 MCP，再启动旧 Web UI；只重建这两个目标服务，不重启无关依赖。
6. 验证 `/health`、容器状态、原账号/数据可见性和关键只读业务路径，再恢复写流量。
7. 保留新 secret 文件供下一次迁移使用；回滚不要求删除 secret 或任何数据卷。

示例骨架（必须替换为已记录的旧镜像和旧 Compose 文件）：

```bash
docker compose -f /path/to/new/mcp-compose.yml stop xiaohongshu-mcp
docker compose -f /path/to/new/webui-compose.yml stop webui

docker compose -f "$ROLLBACK_DIR/mcp-compose.previous.yml" up -d --no-deps xiaohongshu-mcp
docker compose -f "$ROLLBACK_DIR/webui-compose.previous.yml" up -d --no-deps webui

docker compose -f "$ROLLBACK_DIR/mcp-compose.previous.yml" ps
docker compose -f "$ROLLBACK_DIR/webui-compose.previous.yml" ps
curl --fail http://127.0.0.1:18060/health
curl --fail http://127.0.0.1:18080/api/web/health
```

回滚完成标准是：旧镜像与旧端口映射已恢复、原数据卷仍是同一对象、健康与业务复测通过、没有执行任何删卷操作。

## 11. P1 scope、审计与 TLS 迁移

后端分别读取 `XHS_READ_TOKEN_FILE`、`XHS_WRITE_TOKEN_FILE`、`XHS_ADMIN_TOKEN_FILE`。三个 token 权限互不继承：`admin` 只管理账号和登录，不默认包含 `read` 或业务 `write`。Web UI 同样挂载三份 secret，并按路由选择最小 token：账号列表、发现和详情使用 read；发布和互动使用 write；账号创建、移除、默认账号、二维码、重置和资料同步使用 admin。旧 `XHS_API_TOKEN_FILE` 仅作为迁移兼容入口，临时保留全部三类权限，完成客户端迁移后必须移除。

轮换采用 current/previous 双 token overlap。后端可额外挂载只读旧 token，并设置 `XHS_READ_TOKEN_PREVIOUS_FILE`、`XHS_WRITE_TOKEN_PREVIOUS_FILE`、`XHS_ADMIN_TOKEN_PREVIOUS_FILE`；先发布新 token 给客户端，再开启短 overlap，确认旧 token 无调用后删除 previous 挂载并重建容器。不要用环境变量承载 token 内容，也不要长期保留 overlap。

三种可执行模式均只读挂载宿主 secret：默认模板是 current 三 scope；overlap 使用 `docker compose -f docker/docker-compose.yml -f docker/docker-compose.overlap.yml config`，增加三份 previous secret；legacy rollback 使用 `docker compose -f docker/docker-compose.yml -f docker/docker-compose.legacy.yml config`，Web UI 对应使用 `docker compose -f docker-compose.webui.yml -f docker-compose.webui.legacy.yml config`。legacy override 清空三 scope 路径并只设置 `XHS_API_TOKEN_FILE=/run/secrets/xhs_api_token`。legacy 仅限已批准且不超过 24 小时的回滚窗口；窗口结束后先撤销旧客户端，再删除 legacy override/挂载和文件，恢复 current 三 scope 并重新 render 验证。任一已声明路径缺失均 fail closed；不得把 token 内容放进 env。

结构化审计记录 `request_id`、token 哈希 actor、scope、operation、账号/目标哈希、outcome 和 duration；禁止记录 cookie、二维码、xsec_token、正文、素材路径或原始 secret。REST 接受并回显 `X-Request-ID`；发布和评论若返回 408/504，审计 outcome 为 `UNKNOWN`，网关和客户端不得自动重试。完整幂等存储与冲突检测属于 `docs/write-operation-e2e-safety-plan.md` 的后续实现。

LAN 生产入口必须终止 TLS。推荐 Caddy 放在 Web UI 前：仅 Caddy 发布 LAN `443`，Web UI 和 MCP 不发布 LAN 端口；使用内部 CA 时把根证书受控安装到客户端信任库。示例骨架：

```caddyfile
xhs.internal.example {
  tls internal
  reverse_proxy xiaohongshu-webui:8080
}
```

迁移顺序：先配置域名和证书信任，隔离验证 HTTPS WebUI/curl/Hermes MCP 客户端，再把 HTTP 入口重定向到 HTTPS，最后关闭 HTTP publish。Basic/Bearer 不得长期明文穿越不可信 LAN。旧无 token 客户端在 `warn` 阶段只告警；切换 `enforce` 后必须得到明确 401，禁止静默降级。兼容矩阵如下：

| 客户端 | warn | enforce |
| --- | --- | --- |
| Web UI | Basic 登录，代理按路由选择 scope | 同左，必须 HTTPS |
| curl | 旧单 token 可短期迁移 | 必须使用对应 scope token |
| Hermes MCP | MCP 建连与每个 tool wrapper 双重校验 | 越权 tool 返回 FORBIDDEN |
| 旧无 token 客户端 | 告警并临时放行 | 401，不降级 |
