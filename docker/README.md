# Docker 部署

默认使用无界面镜像，已内置浏览器和中文字体。需要查看或辅助操作浏览器时，使用 [GUI 部署](novnc/README.md)。

## 启动

在本目录执行；如果没有下载项目，先将 [docker-compose.yml](docker-compose.yml) 保存到一个空目录，再执行同样的命令：

```bash
docker compose up -d
```

MCP 地址为 `http://127.0.0.1:18060/mcp`，接入方式见 [MCP 客户端配置](../README.md#2-mcp-客户端接入)。

## 镜像源

默认使用 Docker Hub 的 `xpzouying/xiaohongshu-mcp:latest`。国内用户可将 `docker-compose.yml` 的 `image` 改为阿里云镜像：

```yaml
image: crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp:latest
```

两边提供相同版本；需要固定版本时，将 `latest` 换成 `vX.Y.Z`。

## 登录与素材

- 在 MCP 客户端调用获取登录二维码工具，然后用手机 App 扫码。
- `./data` 保存登录状态和运行数据，更新或重建容器时保留此目录。
- 将图片、视频放入 `./images`，调用工具时使用容器内的完整文件路径，例如 `/app/images/cover.jpg`。

## 访问令牌与代理

对外提供 MCP 服务时，设置 `AUTH_TOKEN`：

```bash
AUTH_TOKEN=your-secret-token docker compose up -d
```

Windows PowerShell 先执行 `$env:AUTH_TOKEN = 'your-secret-token'`，再运行 `docker compose up -d`。客户端请求需带 `Authorization: Bearer <token>`。

如需代理，在 Compose 的 `environment` 列表中追加：

```yaml
- XHS_PROXY=http://user:pass@proxy:port
```

支持 HTTP、HTTPS 和 SOCKS5；无需认证时省略 `user:pass@`。

## 日常管理

| 操作 | 命令 |
| --- | --- |
| 查看日志 | `docker compose logs -f` |
| 停止服务（保留数据目录） | `docker compose down` |
| 更新镜像并重建容器 | `docker compose pull && docker compose up -d` |

## 自行构建（可选）

在项目根目录执行：

```bash
docker build -t xiaohongshu-mcp:local .
```

使用自编译版本时，将 Compose 的 `image` 改为 `xiaohongshu-mcp:local`。
