#!/bin/bash
#
# 小红书收藏专辑自动同步脚本
# 在服务器上完全自动化执行，无需人工介入
#
# 使用方法:
#   ./auto-sync-albums.sh
#
# 依赖:
#   - album-sync (Go 编译的同步工具)
#   - 收藏分类结果.json (AI 分类结果)
#   - MCP 服务器运行中 (提供浏览器会话)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "======================================================================"
echo "🚀 小红书收藏专辑自动同步"
echo "======================================================================"
echo ""

# 1. 检查必要文件
log_info "检查必要文件..."

if [ ! -f "./album-sync" ]; then
    log_error "找不到 album-sync 工具"
    exit 1
fi
log_success "✓ album-sync 工具存在"

if [ ! -f "./收藏分类结果.json" ]; then
    log_error "找不到 收藏分类结果.json"
    exit 1
fi
log_success "✓ 收藏分类结果.json 存在"

# 2. 检查 MCP 服务器状态
log_info "检查 MCP 服务器状态..."

HEALTH_RESPONSE=$(curl -s http://localhost:18060/health 2>/dev/null || echo "")
if echo "$HEALTH_RESPONSE" | grep -q '"status":"healthy"'; then
    log_success "✓ MCP 服务器运行正常"
else
    log_warn "MCP 服务器可能未运行，尝试继续..."
fi

# 3. 显示同步计划
log_info "同步计划:"
echo ""

python3 << 'EOF'
import json

with open('收藏分类结果.json', 'r', encoding='utf-8') as f:
    data = json.load(f)

categories = data.get('categories', {})
total = data.get('total', 0)

print(f"   总收藏笔记：{total} 条\n")
print("   专辑同步清单:")

for category, cat_data in sorted(categories.items(), key=lambda x: x[1]['count'] if isinstance(x[1], dict) else 0, reverse=True):
    if category == '其他':
        continue
    if isinstance(cat_data, dict):
        count = cat_data.get('count', 0)
        if count > 0:
            print(f"   📁 {category}: {count} 条")

print("")
EOF

# 4. 执行同步
echo "======================================================================"
log_info "开始执行自动同步..."
echo "======================================================================"
echo ""

# 设置环境变量
export DISPLAY=:99  # 虚拟显示（如果有 Xvfb）
export HEADLESS=true

# 创建日志目录
mkdir -p logs
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="logs/sync_${TIMESTAMP}.log"

# 执行同步工具
log_info "启动 auto-album-sync 工具..."
log_info "日志文件：$LOG_FILE"
echo ""

# 尝试运行 auto-album-sync（自动化版本，无需交互确认）
if ./auto-album-sync -file=收藏分类结果.json 2>&1 | tee "$LOG_FILE"; then
    log_success "同步完成！"
else
    log_error "同步过程中出现错误，请查看日志"
fi

echo ""
echo "======================================================================"

# 5. 生成报告
log_info "生成同步报告..."

python3 << 'EOF'
import json
import os
from datetime import datetime

# 尝试读取同步结果
result_files = [f for f in os.listdir('.') if f.startswith('专辑同步结果') and f.endswith('.json')]

if result_files:
    latest_result = sorted(result_files)[-1]
    with open(latest_result, 'r', encoding='utf-8') as f:
        result = json.load(f)
    
    print("\n📊 同步结果摘要:\n")
    print(f"   总专辑数：{result.get('total_albums', len(result.get('albums', [])))}")
    print(f"   成功：{result.get('success', 0)}")
    print(f"   失败：{result.get('failed', 0)}")
    print("\n   详情:")
    
    for album in result.get('albums', []):
        status = "✅" if album.get('success') else "❌"
        print(f"   {status} {album.get('name', 'Unknown')}: {album.get('count', 0)} 条")
else
    print("\n⚠️  未找到同步结果文件")
    print("   请检查日志文件了解详情")

print("")
EOF

# 6. 显示下一步
echo "======================================================================"
log_info "完成！"
echo ""
log_info "查看日志：less $LOG_FILE"
log_info "查看报告：cat 专辑同步结果_*.json | python3 -m json.tool"
echo "======================================================================"
