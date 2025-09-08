#!/bin/bash

# 创建新分支并重新发起PR的脚本
set -e

echo "🚀 开始创建新分支并重新发起PR..."

# 进入项目目录
cd /Users/pan/Downloads/xiaohongshu-mcp-main

# 显示当前状态
echo "📋 当前Git状态:"
git status --short
echo ""

# 获取当前commit的hash
echo "📝 获取当前commit信息..."
current_commit=$(git rev-parse HEAD)
echo "当前commit: $current_commit"
echo ""

# 切换到main分支
echo "🔄 切换到main分支..."
git checkout main

# 拉取最新的main分支
echo "⬇️ 拉取最新的main分支..."
git pull origin main

# 创建新的分支名（使用时间戳确保唯一性）
new_branch="fix-browser-automation-$(date +%Y%m%d-%H%M%S)"
echo "🌿 创建新分支: $new_branch"
git checkout -b $new_branch

# Cherry-pick当前的commit到新分支
echo "🍒 Cherry-pick commit到新分支..."
git cherry-pick $current_commit

# 推送新分支到远程
echo "⬆️ 推送新分支到远程仓库..."
git push -u origin $new_branch

echo ""
echo "🎉 新分支创建和推送成功！"
echo ""
echo "📋 分支信息:"
echo "- 旧分支: browser-fixier"
echo "- 新分支: $new_branch"
echo "- 当前commit: $current_commit"
echo ""
echo "🔗 请访问以下链接创建新的Pull Request:"
echo "https://github.com/RedMagicVer7/xiaohongshu-mcp/compare/main...$new_branch"
echo ""
echo "💡 建议删除旧分支 browser-fixier（可选）:"
echo "git push origin --delete browser-fixier"