#!/bin/bash

# 重新发起PR脚本
set -e

echo "🚀 开始重新发起Pull Request..."

# 进入项目目录
cd /Users/pan/Downloads/xiaohongshu-mcp-main

# 检查当前分支状态
echo "📋 当前分支状态:"
git branch --show-current
echo ""

# 检查是否有未提交的更改
echo "📁 检查工作区状态..."
git status --porcelain

# 确保所有更改都已添加和提交
echo "➕ 添加所有更改..."
git add .

# 检查是否有新的更改需要提交
if git diff --cached --quiet; then
    echo "✅ 没有新的更改需要提交"
else
    echo "📝 发现新的更改，正在提交..."
    git commit -m "refactor: 优化浏览器自动化稳定性和错误处理

🔧 主要修复:
- 修复页面秒关问题：优化浏览器生命周期管理
- 替换危险的Must方法为安全的DOM操作方法
- 添加60秒智能等待机制和页面状态监控
- 修复编译错误：解决main函数冲突和缺失导入

⚡ 技术改进:
- 实现Navigate()和WaitLoad()安全方法替代Must方法
- 添加关键元素等待机制，确保页面完全渲染
- 延长超时时间到300秒，提供充足加载时间
- 强化错误处理，提供详细中文错误描述
- 添加详细调试日志便于问题定位

🛡️ 稳定性提升:
- 页面状态监控：每10秒检查页面活跃状态
- 关键元素检测：等待div.upload-content等元素出现
- 重试机制：为关键操作添加重试逻辑
- 超时保护：合理设置超时时间防止无限等待

📊 符合项目规范:
- 使用安全方法进行DOM操作（避免Must方法）
- 实现重试机制（间隔1秒，最多重试5-10次）
- 添加关键元素等待机制
- 强化错误处理规范"
    echo "✅ 提交完成"
fi

# 强制推送到远程分支
echo "🔄 推送到远程仓库 origin/browser-fixier..."
git push --force-with-lease origin browser-fixier

echo ""
echo "🎉 代码推送成功！"
echo ""
echo "🔗 请访问以下链接创建/更新 Pull Request:"
echo "https://github.com/RedMagicVer7/xiaohongshu-mcp/compare/main...browser-fixier"
echo ""
echo "📋 建议的PR标题:"
echo "fix: 修复页面秒关问题和浏览器自动化稳定性优化"
echo ""
echo "📝 PR描述模板已准备，包含详细的修改说明和技术改进点"