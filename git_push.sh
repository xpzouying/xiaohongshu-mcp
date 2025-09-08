#!/bin/bash

# Git提交和推送脚本
set -e

echo "开始Git操作..."

# 进入项目目录
cd /Users/pan/Downloads/xiaohongshu-mcp-main

# 检查当前分支
echo "当前分支:"
git branch --show-current

# 添加所有修改的文件
echo "添加修改的文件..."
git add .

# 检查是否有变更
if git diff --cached --quiet; then
    echo "没有文件变更，无需提交"
else
    # 提交变更
    echo "提交变更..."
    git commit -m "fix: 修复页面秒关问题和编译错误

- 修复浏览器生命周期管理问题，确保页面不会被过早关闭
- 将所有危险的Must方法替换为安全的DOM操作方法
- 添加详细的调试日志和页面状态监控
- 添加60秒智能等待机制和关键元素检测
- 修复编译错误：添加缺失的logrus导入，删除冲突的main函数
- 临时禁用浏览器自动关闭用于调试页面秒关问题

技术改进:
- 使用安全的Navigate()和WaitLoad()替代MustNavigate()和MustWaitLoad()
- 添加页面状态监控，每10秒检查页面活跃状态
- 实现关键元素等待机制，确保页面完全渲染
- 延长超时时间到300秒，提供充足的页面加载时间
- 强化错误处理，提供详细的中文错误描述"

    echo "✅ 提交成功"
fi

# 推送到远程仓库
echo "推送到远程仓库..."
git push origin browser-fixier

echo "✅ 推送成功到 origin/browser-fixier"
echo ""
echo "🔗 请访问以下链接创建Pull Request:"
echo "https://github.com/RedMagicVer7/xiaohongshu-mcp/compare/main...browser-fixier"