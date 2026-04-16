#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARCH=$(uname -m)
NO_CACHE=false

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-cache|-n)
            NO_CACHE=true
            shift
            ;;
        *)
            echo "[WARN] 未知参数: $1"
            shift
            ;;
    esac
done

echo "=========================================="
echo "  xiaohongshu-mcp Docker 启动脚本"
echo "=========================================="
echo ""

cd "$SCRIPT_DIR"

if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
    echo "[INFO] 检测到 ARM64 架构 (Apple Silicon)"
    echo "[INFO] 使用 docker-compose.arm64.yml"

    if [ "$NO_CACHE" = true ]; then
        echo "[INFO] 强制重新构建 (不走 cache)"
        docker compose -f docker-compose.arm64.yml build --no-cache
    else
        docker compose -f docker-compose.arm64.yml build
    fi
    docker compose -f docker-compose.arm64.yml up -d
    echo ""
    echo "查看日志: docker compose -f docker-compose.arm64.yml logs -f"
    echo "停止服务: docker compose -f docker-compose.arm64.yml down"
else
    echo "[INFO] 检测到 AMD64/Intel 架构"
    echo "[INFO] 使用 docker-compose.yml (从 Docker Hub 拉取镜像)"
    docker compose -f docker-compose.yml up -d
fi

echo ""
echo "[INFO] 构建/启动完成!"
echo "[INFO] 服务已启动，监听端口: 18060"
echo ""
echo "查看日志: docker compose logs -f"
echo "停止服务: docker compose down"
echo "=========================================="
