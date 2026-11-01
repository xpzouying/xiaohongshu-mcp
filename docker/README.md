# Docker 部署

默认使用无界面版；需要查看或辅助操作浏览器时，使用 [GUI 部署](novnc/README.md)。

## 启动

先启动 Docker，在 `docker/` 目录执行以下操作。未下载项目时，也可将 [docker-compose.yml](docker-compose.yml) 保存到空目录，在该目录操作。

国内用户可先将 `docker-compose.yml` 中的 `image` 改为阿里云镜像；默认使用 Docker Hub，可跳过：

```yaml
image: crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp:latest
```

```bash
docker compose up -d
```

MCP 地址：`http://127.0.0.1:18060/mcp`，接入方式见 [客户端配置](../README.md#2-mcp-客户端接入)。

## 登录与素材

- 让 AI 获取小红书登录二维码，再用手机 App 扫码。
- `data/` 保存登录状态和运行数据，更新时保留。
- 图片、视频放入 `images/`，告诉 AI 容器内的文件路径，例如 `/app/images/cover.jpg`。

## 日常管理

| 操作 | 命令 |
| --- | --- |
| 查看日志 | `docker compose logs -f` |
| 停止服务（保留数据） | `docker compose down` |
| 更新 | `docker compose pull && docker compose up -d` |

<details>
<summary>按需配置：访问令牌、代理与自行构建</summary>

### 访问令牌

对外提供 MCP 服务时，设置 `AUTH_TOKEN`：

```bash
AUTH_TOKEN=your-secret-token docker compose up -d
```

Windows PowerShell 先执行 `$env:AUTH_TOKEN = 'your-secret-token'`，再运行 `docker compose up -d`。客户端请求需带 `Authorization: Bearer <token>`。

### 代理

在 Compose 的 `environment` 列表中追加：

```yaml
- XHS_PROXY=http://user:pass@proxy:port
```

支持 HTTP、HTTPS 和 SOCKS5；无需认证时省略 `user:pass@`。

### 自行构建

在项目根目录执行 `docker build -t xiaohongshu-mcp:local .`，再将 Compose 的 `image` 改为 `xiaohongshu-mcp:local`。

使用官方固定版本时，将镜像标签 `latest` 换成 `vX.Y.Z`。

</details>
