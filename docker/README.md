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

## 3. 使用 MCP-Inspector 进行连接

**注意 IP 换成你自己的 IP**

<img width="2606" height="1164" alt="image" src="https://github.com/user-attachments/assets/495916ad-0643-491d-ae3c-14cbf431c16f" />

对应的 Docker 日志一切正常。

<img width="1662" height="458" alt="image" src="https://github.com/user-attachments/assets/309c2dab-51c4-4502-a41b-cdd4a3dd57ac" />

## 4. 扫码登录

1. **重要**，一定要先把 App 提前打开，准备扫码登录。
2. 尽快扫码，有可能二维码会过期。

打开 MCP-Inspector 获取二维码和进行扫码。

<img width="2632" height="1468" alt="image" src="https://github.com/user-attachments/assets/543a5427-50e3-4970-b942-5d05d69596f4" />

<img width="2624" height="1222" alt="image" src="https://github.com/user-attachments/assets/4f38ca81-1014-4874-ab4d-baf02b750b55" />

扫码成功后，再次扫码后，就会提示已经完成登录了。

<img width="2614" height="994" alt="image" src="https://github.com/user-attachments/assets/5356914a-3241-4bfd-b6b2-49c1cc5e3394" />


