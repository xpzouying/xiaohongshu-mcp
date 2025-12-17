# 管理器 Docker 部署指南

管理器 (Manager) 是一个 Web GUI 工具，用于管理多个小红书 MCP 用户实例。

## 架构说明

单容器方案：管理器和主程序打包在同一镜像中，管理器在容器内启动多个主程序进程。

```
┌─────────────────────────────────────────────┐
│              Docker Container               │
│  ┌─────────────────────────────────────┐   │
│  │     Manager (Web GUI :18050)        │   │
│  │         管理用户、启停实例            │   │
│  └──────────────┬──────────────────────┘   │
│                 │ 启动子进程                │
│    ┌────────────┼────────────┐             │
│    ▼            ▼            ▼             │
│ ┌──────┐   ┌──────┐    ┌──────┐           │
│ │user-1│   │user-2│    │user-n│           │
│ │:18060│   │:18061│    │:180xx│           │
│ └──────┘   └──────┘    └──────┘           │
└─────────────────────────────────────────────┘
```

## 快速开始

### 1. 使用 Docker Compose（推荐）

```bash
# 启动服务
docker compose -f docker-compose.manager.yml up -d

# 查看日志
docker compose -f docker-compose.manager.yml logs -f

# 停止服务
docker compose -f docker-compose.manager.yml down
```

### 2. 手动 Docker 运行

```bash
# 构建镜像
docker build -f Dockerfile.manager -t xiaohongshu-manager .

# 运行容器
docker run -d \
  --name xiaohongshu-manager \
  --shm-size=1g \
  -p 18050:18050 \
  -p 18060-18160:18060-18160 \
  -v ./data:/app/data \
  -v ./images:/app/images \
  xiaohongshu-manager
```

## 访问管理界面

启动后访问：http://localhost:18050

## 端口说明

| 端口 | 用途 |
|------|------|
| 18050 | 管理器 Web GUI |
| 18060-18160 | 主程序实例端口池（可支持约 100 个用户） |

> **重要**：创建用户时填写的端口必须在 `18060-18160` 范围内，否则只能在容器内部访问。

## 数据持久化

| 路径 | 用途 |
|------|------|
| `./data/manager/users.json` | 管理器配置（用户列表、端口等） |
| `./data/cookies/{user-id}.json` | 各用户登录 cookies |
| `./data/profiles/{user-id}/` | 各用户浏览器 profile |
| `./data/logs/{user-id}.log` | 各用户进程日志 |
| `./images/` | 本地图片发布目录 |

### 配置文件说明

`./data/manager/users.json` 示例：
```json
{
  "bin": "./xiaohongshu-mcp",
  "headless": true,
  "data_dir": "./data",
  "users": [
    {"id": "user1", "port": 18060, "proxy": ""},
    {"id": "user2", "port": 18061, "proxy": "http://proxy:8080"}
  ]
}
```

- `bin`: 主程序路径（容器内为 `./xiaohongshu-mcp`）
- `data_dir`: 数据目录（容器内为 `./data`，即 `/app/data`）
- `users[].port`: **必须在映射端口段内**（默认 18060-18160）

## 使用本地图片发布

如果需要使用本地图片发布功能：

1. 将图片放入 `./images/` 目录
2. 在 MCP 调用时，指定路径为 `/app/images/your-image.jpg`

## 注意事项

1. **共享内存**：多实例场景建议设置 `--shm-size=1g`，避免 Chrome 崩溃
2. **端口范围**：默认开放 18060-18160，约 100 个端口，按需调整
3. **首次启动**：会自动创建 `./data/manager/users.json` 配置文件

## 常见问题

### Chrome 崩溃

增加共享内存：
```bash
docker run --shm-size=2g ...
```

### 端口不足

修改 `docker-compose.manager.yml` 中的端口范围：
```yaml
ports:
  - "18050:18050"
  - "18060-18200:18060-18200"  # 扩展到 140 个端口
```

### 无法访问管理界面

确保防火墙开放 18050 端口。
