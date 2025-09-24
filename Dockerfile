FROM golang:1.24-bullseye AS builder

WORKDIR /app

# 设置GOPROXY以解决可能的网络问题
ENV GOPROXY=https://goproxy.cn,direct

# 复制Go模块定义
COPY go.mod go.sum ./
# 尝试下载依赖，如果失败则继续（后续会重试）
RUN go mod download || true

# 复制源代码
COPY . .

# 再次尝试下载依赖（确保所有依赖都已下载）
RUN go mod tidy

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux go build -o xiaohongshu-mcp .
RUN CGO_ENABLED=0 GOOS=linux go build -o login ./cmd/login

# 使用多阶段构建减小最终镜像大小
FROM debian:bullseye-slim

# 设置时区为中国时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 安装Chrome浏览器和所有必要依赖
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    ca-certificates \
    fonts-liberation \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libwayland-client0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxkbcommon0 \
    libxrandr2 \
    xdg-utils \
    libvulkan1 \
    libxss1 \
    xvfb \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# 安装Chromium作为替代方案（更可靠，在Debian仓库中）
RUN apt-get update && apt-get install -y chromium \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# 创建数据目录
RUN mkdir -p /data/cookies

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的应用
COPY --from=builder /app/xiaohongshu-mcp /app/
COPY --from=builder /app/login /app/

# 设置环境变量
ENV COOKIE_FILE_PATH=/data/cookies/cookies.json
ENV DISPLAY=:99

# 暴露应用端口
EXPOSE 18060

# 创建启动脚本
RUN echo '#!/bin/bash\n\
# 启动X虚拟帧缓冲区\n\
Xvfb :99 -screen 0 1280x1024x24 -ac &\n\
\n\
# 检查是否需要登录\n\
if [ ! -f "$COOKIE_FILE_PATH" ] || [ "$FORCE_LOGIN" = "true" ]; then\n\
  echo "需要登录，启动登录流程..."\n\
  ./login -bin /usr/bin/chromium\n\
fi\n\
\n\
# 启动主应用\n\
exec ./xiaohongshu-mcp -bin /usr/bin/chromium "$@"\n\
' > /app/start.sh && chmod +x /app/start.sh

# 设置容器启动命令
ENTRYPOINT ["/app/start.sh"]