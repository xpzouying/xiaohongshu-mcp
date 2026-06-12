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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/app .

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

# 2. 安装 CloakBrowser Chromium 运行依赖、Python 和中文字体
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
    python3 \
    python3-pip \
    wget \
    xdg-utils \
    && rm -rf /var/lib/apt/lists/*

# 3. 创建共享目录并设置权限。
# /opt/cloakbrowser 保存构建阶段预下载的浏览器，不能放到运行时会被 volume 覆盖的 /app/data。
RUN mkdir -p /opt/cloakbrowser/home /opt/cloakbrowser/cache /opt/cloakbrowser/config \
    /app/data/home /app/data/cache /app/data/config /app/images && \
    chmod -R 755 /opt/cloakbrowser && \
    chmod -R 777 /app/data /app/images

# 4. 安装 CloakBrowser 并在构建阶段预下载 Chromium，避免容器运行时下载失败。
COPY build/find_cloakbrowser_binary.py /usr/local/bin/find_cloakbrowser_binary.py
ARG CLOAKBROWSER_VERSION=0.3.31
RUN --mount=type=cache,target=/root/.cache/pip \
    python3 -m pip install "cloakbrowser[geoip]==${CLOAKBROWSER_VERSION}"
RUN --mount=type=cache,target=/var/cache/cloakbrowser-home/.cloakbrowser \
    HOME=/var/cache/cloakbrowser-home \
    XDG_CACHE_HOME=/var/cache/cloakbrowser-cache \
    XDG_CONFIG_HOME=/var/cache/cloakbrowser-config \
    python3 /usr/local/bin/find_cloakbrowser_binary.py \
    --install-home /opt/cloakbrowser/home \
    --link /usr/local/bin/cloak-chromium
RUN \
    test -x /usr/local/bin/cloak-chromium

COPY --from=builder /out/app .

# 5. 设置默认 CloakBrowser Chromium 路径（rod 会用）
ENV HOME=/app/data/home
ENV XDG_CACHE_HOME=/app/data/cache
ENV XDG_CONFIG_HOME=/app/data/config
ENV ROD_BROWSER_BIN=/usr/local/bin/cloak-chromium

EXPOSE 18060

CMD ["./app"]
