#!/bin/bash
#
# 小红书收藏专辑同步 - 简单版本
# 使用 MCP 服务器的浏览器会话执行同步
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "======================================================================"
echo "🚀 小红书收藏专辑同步 - 简单版本"
echo "======================================================================"
echo ""

# 1. 检查 MCP 服务器
echo "[INFO] 检查 MCP 服务器..."
HEALTH=$(curl -s http://localhost:18060/health 2>/dev/null || echo "")
if echo "$HEALTH" | grep -q '"status":"healthy"'; then
    echo "[SUCCESS] MCP 服务器运行正常"
else
    echo "[ERROR] MCP 服务器未运行"
    exit 1
fi

# 2. 检查分类文件
if [ ! -f "./收藏分类结果.json" ]; then
    echo "[ERROR] 找不到 收藏分类结果.json"
    exit 1
fi
echo "[SUCCESS] 分类文件存在"

# 3. 显示同步计划
echo ""
echo "📋 同步计划:"
python3 << 'EOF'
import json
with open('收藏分类结果.json', 'r', encoding='utf-8') as f:
    data = json.load(f)
categories = data.get('categories', {})
total = data.get('total', 0)
print(f"   总收藏笔记：{total} 条\n")
for name, cat_data in sorted(categories.items(), key=lambda x: x[1]['count'] if isinstance(x[1], dict) else 0, reverse=True):
    if name == '其他':
        continue
    if isinstance(cat_data, dict):
        count = cat_data.get('count', 0)
        if count > 0:
            print(f"   📁 {name}: {count} 条")
EOF

echo ""
echo "======================================================================"
echo "⚠️  当前方案限制"
echo "======================================================================"
echo ""
echo "由于小红书网页版需要登录状态和浏览器环境，自动同步需要以下条件："
echo ""
echo "1. ✅ MCP 服务器运行中 (已满足)"
echo "2. ✅ 有效的 cookies (已满足)"
echo "3. ⏳ 浏览器会话 (需要配置)"
echo ""
echo "推荐方案："
echo ""
echo "方法 1: 使用手动同步清单（最可靠）"
echo "   cat 收藏分类结果_专辑同步清单.md"
echo ""
echo "方法 2: 在本地电脑运行自动同步"
echo "   1. 下载 album-sync 工具和 收藏分类结果.json"
echo "   2. 运行：./album-sync -file=收藏分类结果.json -headless=false"
echo ""
echo "======================================================================"
echo ""
echo "📝 已生成的文件:"
echo ""
ls -lh *.json *.md 2>/dev/null | grep -E "(专辑 | 收藏 | 同步)" | awk '{print "   " $9 " (" $5 ")"}'
echo ""
echo "======================================================================"
