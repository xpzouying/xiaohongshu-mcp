#!/bin/bash

# 小红书MCP项目编译脚本
echo "开始编译小红书MCP项目..."

# 进入项目目录
cd /Users/pan/Downloads/xiaohongshu-mcp-main

# 清理模块缓存
echo "清理Go模块缓存..."
go clean -modcache

# 下载依赖
echo "下载项目依赖..."
go mod tidy
go mod download

# 编译项目
echo "编译项目..."
go build -o xiaohongshu-mcp .

# 检查编译结果
if [ -f "xiaohongshu-mcp" ]; then
    echo "✅ 编译成功！生成可执行文件: xiaohongshu-mcp"
    echo "文件大小: $(ls -lh xiaohongshu-mcp | awk '{print $5}')"
    echo ""
    echo "运行方式:"
    echo "  ./xiaohongshu-mcp                    # headless模式"
    echo "  ./xiaohongshu-mcp -headless=false    # 有界面模式"
else
    echo "❌ 编译失败！"
    exit 1
fi