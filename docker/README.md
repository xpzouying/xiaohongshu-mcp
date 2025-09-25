# Docker 使用说明
<!-- TOC depthFrom:2 -->

- [1. 自己构建镜像](#1-自己构建镜像)
- [2. 手动 Docker Compose](#2-手动 Docker Compose)

## 1. 自己构建镜像

可以使用源码自己构建镜像，如下：

在有项目的Dockerfile的目录运行

`docker build -t xpzouying/xiaohongshu-mcp .`

`xpzouying/xiaohongshu-mcp`为镜像名称和版本，可以自己起个名字

## 2. 手动 Docker Compose

```
# 启动 docker-compose
docker compose up -d

# 停止 docker-compose
docker compose stop

# 查看实时日志
docker logs -f xpzouying/xiaohongshu-mcp

# 进入容器
docker exec -it xpzouying/xiaohongshu-mcp /bin/bash

# 手动更新容器
docker compose pull && docker compose up -d
```
