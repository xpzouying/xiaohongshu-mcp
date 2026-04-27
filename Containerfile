# ---- build stage ----
FROM registry.fedoraproject.org/fedora:44 AS builder

# 安装 Go 编译环境
RUN dnf install -y golang && dnf clean all

WORKDIR /opt/app-root/src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 不硬编码 GOARCH，让构建系统自动选择架构
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" .

# ---- run stage ----
FROM registry.fedoraproject.org/fedora:44

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

WORKDIR /opt/app-root/src

# 安装 Chromium 及其运行依赖（go-rod 无头浏览器）
RUN dnf install -y --setopt=install_weak_deps=False \
    chromium \
    chromium-headless \
    liberation-fonts \
    alsa-lib \
    at-spi2-atk \
    atk \
    cairo \
    cups-libs \
    dbus-libs \
    expat \
    fontconfig \
    libgbm \
    glib2 \
    gtk3 \
    nspr \
    nss \
    pango \
    libX11 \
    libX11-xcb \
    libxcb \
    libXcomposite \
    libXcursor \
    libXdamage \
    libXext \
    libXfixes \
    libXi \
    libXrandr \
    libXrender \
    libXScrnSaver \
    libXtst \
    wget \
    xdg-utils \
    && dnf clean all

COPY --from=builder /opt/app-root/src/xiaohongshu-mcp .

# 创建共享目录并设置权限
RUN mkdir -p /opt/app-root/src/images && \
    chmod 777 /opt/app-root/src/images

# OpenShift 随机 UID 兼容：设置 HOME 并开放 root 组写权限
ENV HOME=/opt/app-root/src
RUN chmod -R g+rwX /opt/app-root/src

# 设置 Chromium 路径（rod 会用）
ENV ROD_BROWSER_BIN=/usr/bin/chromium-browser

EXPOSE 18060

CMD ["./xiaohongshu-mcp"]
