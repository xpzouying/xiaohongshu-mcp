#!/usr/bin/env python3
"""
小红书收藏专辑自动同步工具（通过 MCP 服务器）
使用已有的浏览器会话执行同步操作
"""

import json
import requests
import time

BASE_URL = "http://localhost:18060"

def sync_categories():
    """同步分类到专辑"""
    print("=" * 70)
    print("🚀 小红书收藏专辑自动同步工具")
    print("=" * 70)
    
    # 检查服务器
    try:
        resp = requests.get(f"{BASE_URL}/health", timeout=5)
        if not resp.json().get('success'):
            print("❌ 服务器未就绪")
            return False
        print("✅ 服务器连接成功")
    except Exception as e:
        print(f"❌ 无法连接服务器：{e}")
        return False
    
    # 加载分类结果
    categories_file = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json'
    try:
        with open(categories_file) as f:
            data = json.load(f)
        print(f"✅ 分类文件加载成功：{data.get('total', 0)} 条笔记\n")
    except Exception as e:
        print(f"❌ 加载分类文件失败：{e}")
        return False
    
    categories = data.get('categories', {})
    
    # 显示同步计划
    print("📋 同步计划:\n")
    for category, cat_data in sorted(categories.items(), key=lambda x: x[1]['count'], reverse=True):
        if category == '其他':
            continue
        count = cat_data['count']
        if count > 0:
            print(f"   📁 {category}: {count}条")
    
    print("\n" + "=" * 70)
    print("⚠️  由于小红书 API 限制，自动同步功能需要浏览器环境")
    print("\n💡 建议使用以下方法之一：\n")
    print("方法 1: 使用手动同步清单（最可靠）")
    print("   cat 收藏分类结果_专辑同步清单.md")
    print("   然后在小红书网页版手动创建专辑和移动笔记\n")
    print("方法 2: 在本地环境运行自动同步工具")
    print("   cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp")
    print("   ./album-sync -file=收藏分类结果.json -headless=false\n")
    print("=" * 70)
    
    # 生成同步摘要
    summary = {
        'total': data.get('total', 0),
        'albums': [],
        'timestamp': time.strftime('%Y-%m-%d %H:%M:%S'),
        'status': 'pending'
    }
    
    for category, cat_data in sorted(categories.items(), key=lambda x: x[1]['count'], reverse=True):
        if category == '其他':
            continue
        count = cat_data['count']
        if count > 0:
            summary['albums'].append({
                'name': category,
                'count': count,
                'status': 'pending'
            })
    
    # 保存同步摘要
    summary_file = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/专辑同步摘要.json'
    with open(summary_file, 'w', encoding='utf-8') as f:
        json.dump(summary, f, ensure_ascii=False, indent=2)
    
    print(f"\n✅ 同步摘要已保存：{summary_file}")
    print("\n📝 下一步操作：")
    print("1. 打开小红书网页版")
    print("2. 访问收藏页面")
    print("3. 按以下顺序创建专辑并移动笔记：\n")
    
    for i, album in enumerate(summary['albums'], 1):
        print(f"   {i}. {album['name']} ({album['count']}条)")
    
    print("\n" + "=" * 70)
    
    return True

if __name__ == '__main__':
    sync_categories()
