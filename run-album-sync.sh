#!/bin/bash
# 小红书收藏专辑自动同步脚本

set -e

echo "======================================================================"
echo "🚀 小红书收藏专辑自动同步工具"
echo "======================================================================"

# 检查文件
CATEGORIES_FILE="/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json"
if [ ! -f "$CATEGORIES_FILE" ]; then
    echo "❌ 分类文件不存在：$CATEGORIES_FILE"
    exit 1
fi

echo "✅ 分类文件存在"

# 显示分类统计
echo ""
echo "📊 分类统计:"
python3 << EOF
import json
with open('$CATEGORIES_FILE') as f:
    data = json.load(f)
for cat, info in sorted(data['categories'].items(), key=lambda x: x[1]['count'], reverse=True):
    if cat != '其他' and info['count'] > 0:
        print(f"   - {cat}: {info['count']}条")
EOF

echo ""
echo "⚠️  警告：即将执行自动同步操作"
echo "   这将自动创建专辑并将笔记移动到对应专辑"
echo ""
read -p "是否继续？(y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
fi

echo ""
echo "🚀 开始同步..."
echo ""

# 运行同步工具
cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp
./album-sync -file="$CATEGORIES_FILE" -headless=true

echo ""
echo "======================================================================"
echo "✅ 同步完成！"
echo "======================================================================"
