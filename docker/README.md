# Docker 使用说明

## 0. 重点注意

写在最前面。

- 启动后，会产生一个 `images/` 目录，用于存储发布的图片。它会挂载到 Docker 容器里面。
  如果要使用本地图片发布的话，请确保图片拷贝到 `./images/` 目录下，并且让 MCP 在发布的时候，指定文件夹为：`/app/images`，否则一定失败。

## 1. 获取 Docker 镜像

### 1.1 从 Docker Hub 拉取（推荐）

我们提供了预构建的 Docker 镜像，可以直接从 Docker Hub 拉取使用：

```bash
# 拉取最新镜像
docker pull xpzouying/xiaohongshu-mcp
```

Docker Hub 地址：[https://hub.docker.com/r/xpzouying/xiaohongshu-mcp](https://hub.docker.com/r/xpzouying/xiaohongshu-mcp)

> ⚠️ 如果你要使用本仓库当前分支新增工具（例如 `transcribe_feed_video`）和 `ContentRemixAgent` 前端，**建议优先使用本地构建**，不要直接依赖远端旧镜像。

### 1.2 从阿里云镜像源拉取（国内用户推荐）

国内用户可以使用阿里云容器镜像服务，拉取速度更快：

```bash
# 拉取最新镜像
docker pull crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp
```

### 1.3 自己构建镜像（可选）

在有项目的Dockerfile的目录运行

```bash
docker build -t xpzouying/xiaohongshu-mcp .
```

`xpzouying/xiaohongshu-mcp`为镜像名称和版本。

推荐（当前仓库）：

```bash
cd docker
docker compose build --no-cache xiaohongshu-mcp
```

该构建会在镜像中预装：

- `ffmpeg`

浏览器由 `go-rod` 在运行时自动下载 Chromium（无需额外配置 `ROD_BROWSER_BIN`）。
视频转写链路已切换为 GLM API，无需在容器内安装 `whisper.cpp`。

推荐在启动前导出 API Key：

```bash
# 默认转写 provider：dashscope
export VIDEO_TRANSCRIBE_PROVIDER=dashscope
export DASHSCOPE_API_KEY=your_key_here

# 如需切换 glm：
# export VIDEO_TRANSCRIBE_PROVIDER=glm
# export ZHIPUAI_API_KEY=your_key_here
# 或 export BIGMODEL_API_KEY=your_key_here
```

然后再到 `docker/` 目录运行 compose。

<img width="2576" height="874" alt="image" src="https://github.com/user-attachments/assets/fe7e87f1-623f-409f-8b54-e11d380fc7b8" />

## 2. 手动 Docker Compose

> **国内用户提示**：如需使用阿里云镜像源，请修改 `docker-compose.yml` 文件，注释掉 Docker Hub 镜像行，取消阿里云镜像行的注释：
> ```yaml
> # image: xpzouying/xiaohongshu-mcp
> image: crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp
> ```

```bash
# 注意：在 docker-compose.yml 文件的同一个目录，或者手动指定 docker-compose.yml。

# --- 启动 docker 容器 ---
# 启动 docker-compose
docker compose up -d

# 查看日志
docker logs -f xpzouying/xiaohongshu-mcp

# 或者
docker compose logs -f
```

查看日志，下面表示成功启动。

<img width="1012" height="98" alt="image" src="https://github.com/user-attachments/assets/c374f112-a5b5-4cf6-bd9f-080252079b10" />


```bash
# 停止 docker-compose
docker compose stop

# 查看实时日志
docker logs -f xpzouying/xiaohongshu-mcp

# 进入容器
docker exec -it xiaohongshu-mcp bash

# 手动更新容器
docker compose pull && docker compose up -d
```

### 2.1 前端 Web 编排工具（ContentRemixAgent）

`docker-compose.yml` 已包含：

- `content-remix-api`（FastAPI）
- `content-remix-worker`（Celery）
- `content-remix-web`（React + Vite）
- `redis`

启动后默认地址：

- MCP: `http://localhost:18060/mcp`
- Remix API: `http://localhost:18061`
- Remix Web: `http://localhost:5173`

快速自检：

```bash
curl -s http://localhost:18061/health
```

## 3. 使用 MCP-Inspector 进行连接

**注意 IP 换成你自己的 IP**

<img width="2606" height="1164" alt="image" src="https://github.com/user-attachments/assets/495916ad-0643-491d-ae3c-14cbf431c16f" />

对应的 Docker 日志一切正常。

<img width="1662" height="458" alt="image" src="https://github.com/user-attachments/assets/309c2dab-51c4-4502-a41b-cdd4a3dd57ac" />

## 4. 配置代理（可选）

如果需要通过代理访问小红书，可以通过 `XHS_PROXY` 环境变量配置。

### 使用 docker run

```bash
docker run -e XHS_PROXY=http://user:pass@proxy:port xpzouying/xiaohongshu-mcp
```

### 使用 docker-compose

在 `docker-compose.yml` 的 `environment` 中添加 `XHS_PROXY`：

```yaml
environment:
  - COOKIES_PATH=/app/data/cookies.json
  - XHS_PROXY=http://user:pass@proxy:port
```

支持 HTTP/HTTPS/SOCKS5 代理。日志中会自动隐藏代理的认证信息，输出示例：

```
Using proxy: http://***:***@proxy:port
```

## 5. 扫码登录

1. **重要**，一定要先把 App 提前打开，准备扫码登录。
2. 尽快扫码，有可能二维码会过期。

打开 MCP-Inspector 获取二维码和进行扫码。

<img width="2632" height="1468" alt="image" src="https://github.com/user-attachments/assets/543a5427-50e3-4970-b942-5d05d69596f4" />

<img width="2624" height="1222" alt="image" src="https://github.com/user-attachments/assets/4f38ca81-1014-4874-ab4d-baf02b750b55" />

扫码成功后，再次扫码后，就会提示已经完成登录了。

<img width="2614" height="994" alt="image" src="https://github.com/user-attachments/assets/5356914a-3241-4bfd-b6b2-49c1cc5e3394" />
