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
FROM debian:bookworm-slim

WORKDIR /app

# 1. 配置阿里云镜像源加速下载
RUN sed -i 's|http://deb.debian.org|https://mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources && \
    sed -i 's|http://security.debian.org|https://mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources

# 2. 装 Chromium + 依赖（无头模式运行 rod）
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      chromium \
      ca-certificates \
      fonts-liberation \
      fonts-noto-cjk \
      libasound2 \
      libatk1.0-0 \
      libatk-bridge2.0-0 \
      libcups2 \
      libdrm2 \
      libxkbcommon0 \
      libxcomposite1 \
      libxdamage1 \
      libxfixes3 \
      libxrandr2 \
      libgbm1 \
      libpango-1.0-0 \
      libnss3 \
      libxshmfence1 \
      wget \
      tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/app .

# 2. 设置默认 Chromium 路径（rod 会用）
ENV ROD_BROWSER_BIN=/usr/bin/chromium

EXPOSE 18060

CMD ["./app"]

