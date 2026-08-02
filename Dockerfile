# syntax=docker/dockerfile:1.6

# ---- build stage ----
FROM golang:1.24 AS builder

WORKDIR /src
# 配置 Go 模块代理为国内源
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# VERSION 由 CI 通过 --build-arg 传入，本地构建默认 dev
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/app .

# ---- run stage ----
FROM ubuntu:22.04

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

WORKDIR /app

# 1. 先安装必要工具，然后配置阿里云镜像源
RUN apt-get update && apt-get install -y ca-certificates wget gnupg && \
    sed -i 's|http://archive.ubuntu.com|https://mirrors.aliyun.com|g' /etc/apt/sources.list && \
    sed -i 's|http://security.ubuntu.com|https://mirrors.aliyun.com|g' /etc/apt/sources.list

# 2. 安装内置浏览器运行依赖（Chromium 库）和中文字体
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-unifont \
    fonts-freefont-ttf \
    fonts-wqy-zenhei \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libc6 \
    libcairo2 \
    libcairo-gobject2 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libexpat1 \
    libfontconfig1 \
    libgdk-pixbuf-2.0-0 \
    libgbm1 \
    libgcc1 \
    libglib2.0-0 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libstdc++6 \
    libx11-6 \
    libx11-xcb1 \
    libxcb1 \
    libxcomposite1 \
    libxcursor1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxi6 \
    libxkbcommon0 \
    libxrandr2 \
    libxrender1 \
    libxshmfence1 \
    libxss1 \
    libxtst6 \
    lsb-release \
    tini \
    wget \
    xdg-utils \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*

# 3. 创建目录并设置权限。
RUN mkdir -p /app/data/home /app/data/config /app/images && \
    chmod -R 777 /app/data /app/images

# 4. 下载并解压内置浏览器。构建阶段预置，运行时零下载。
# 版本号唯一来源：browser/browser_version.txt（Go 也读它，避免两处漂移）。
# 从自建 CDN 下载中性文件名，并校验 SHA256。
#
# 解压位置必须与 Go 端 EnsureBrowser 找的缓存路径一致：
# $XDG_CACHE_HOME/xiaohongshu-mcp/browser/<版本>/，这样运行时零下载、也不需要任何参数。
# 注意 XDG_CACHE_HOME 指向 /app/cache 而非挂载卷内，否则预置的浏览器会被挂载盖掉。
ENV XDG_CACHE_HOME=/app/cache
COPY browser/browser_version.txt /tmp/browser_version.txt
RUN VER="$(cat /tmp/browser_version.txt | tr -d '[:space:]')" && \
    BASE="https://cdn.one-world.ai/browsers/${VER}" && \
    BROWSER_DIR="${XDG_CACHE_HOME}/xiaohongshu-mcp/browser/${VER}" && \
    mkdir -p "${BROWSER_DIR}" && \
    curl -fsSL -o /tmp/browser.tar.xz "${BASE}/linux-x64.tar.xz" && \
    curl -fsSL "${BASE}/SHA256SUMS" | grep " linux-x64.tar.xz$" | awk '{print $1"  /tmp/browser.tar.xz"}' | sha256sum -c - && \
    tar -xJf /tmp/browser.tar.xz -C "${BROWSER_DIR}" --strip-components=1 && \
    rm /tmp/browser.tar.xz /tmp/browser_version.txt && \
    test -x "${BROWSER_DIR}/chrome" && \
    chmod -R 755 /app/cache

COPY --from=builder /out/app .

ENV HOME=/app/data/home
ENV XDG_CONFIG_HOME=/app/data/config

EXPOSE 18060

# 用 tini 当 PID 1，回收浏览器退出后被过继过来的子进程，避免堆积僵尸进程
ENTRYPOINT ["/usr/bin/tini", "--"]

CMD ["./app"]
