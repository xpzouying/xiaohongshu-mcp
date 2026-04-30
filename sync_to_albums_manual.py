#!/usr/bin/env python3
"""
小红书收藏专辑同步工具（浏览器自动化版）
将分类后的收藏笔记同步到小红书专辑
"""

import json
import subprocess
import sys

def check_server_running():
    """检查服务器是否运行"""
    try:
        import requests
        resp = requests.get('http://localhost:18060/health', timeout=5)
        return resp.json().get('success', False)
    except:
        return False

def sync_to_albums(categories_file, user_id, dry_run=True):
    """
    同步分类结果到专辑
    
    由于小红书专辑 API 需要特殊的认证，
    建议使用以下方式手动操作：
    
    1. 打开小红书网页版
    2. 进入个人主页
    3. 点击"收藏"标签
    4. 点击"创建专辑"
    5. 按分类创建专辑
    6. 手动将笔记移动到对应专辑
    """
    
    # 加载分类结果
    with open(categories_file) as f:
        data = json.load(f)
    
    categories = data.get('categories', {})
    
    print("=" * 70)
    print("📊 小红书专辑同步指南")
    print("=" * 70)
    
    print("\n由于小红书专辑功能需要浏览器交互，请按以下步骤操作：\n")
    
    print("📋 步骤 1: 打开小红书网页版")
    print("   https://www.xiaohongshu.com/user/profile/" + user_id + "?tab=fav&subTab=note")
    
    print("\n📋 步骤 2: 创建专辑")
    print("   点击页面上的\"创建专辑\"按钮，创建以下专辑：\n")
    
    # 统计需要创建的专辑
    album_plan = []
    for category, cat_data in sorted(categories.items(), key=lambda x: x[1]['count'], reverse=True):
        if category == '其他':
            continue
        
        count = cat_data['count']
        if count > 0:
            album_plan.append((category, count))
            print(f"   □ {category} ({count}条)")
    
    print("\n📋 步骤 3: 移动笔记到专辑")
    print("   对每个专辑，按以下列表找到对应笔记并移动：\n")
    
    for category, count in album_plan:
        items = categories[category].get('items', [])
        
        print(f"\n【{category}】({count}条)")
        print("-" * 60)
        
        # 显示前 10 条
        for i, item in enumerate(items[:10], 1):
            title = item.get('title', '无标题')[:45]
            feed_id = item.get('feed_id', '')
            print(f"  {i:2}. {title}")
            print(f"      ID: {feed_id}")
        
        if len(items) > 10:
            print(f"  ... 还有 {len(items)-10} 条")
        
        print()
    
    print("=" * 70)
    print("\n💡 提示：")
    print("   - 可以在收藏页面搜索笔记 ID 快速定位")
    print("   - 批量操作：按住 Ctrl/Command 多选后批量移动")
    print("   - 建议按分类依次处理，避免混淆")
    print("=" * 70)
    
    # 生成 Markdown 格式的清单
    markdown_file = categories_file.replace('.json', '_专辑同步清单.md')
    with open(markdown_file, 'w', encoding='utf-8') as f:
        f.write("# 小红书收藏专辑同步清单\n\n")
        f.write(f"总笔记数：{data.get('total', 0)} 条\n\n")
        f.write("## 专辑列表\n\n")
        
        for category, count in album_plan:
            f.write(f"### {category} ({count}条)\n\n")
            f.write("| 序号 | 标题 | 笔记 ID |\n")
            f.write("|------|------|--------|\n")
            
            items = categories[category].get('items', [])
            for i, item in enumerate(items, 1):
                title = item.get('title', '无标题')
                feed_id = item.get('feed_id', '')
                f.write(f"| {i} | {title} | `{feed_id}` |\n")
            
            f.write("\n")
    
    print(f"\n✅ 同步清单已保存：{markdown_file}")


def main():
    """主函数"""
    # 配置
    CATEGORIES_FILE = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json'
    USER_ID = '620923cd000000002102474c'
    
    print("🔍 小红书收藏专辑同步工具")
    print("=" * 70)
    
    # 检查文件
    try:
        with open(CATEGORIES_FILE) as f:
            data = json.load(f)
        print(f"✅ 分类文件加载成功：{data.get('total', 0)} 条笔记")
    except Exception as e:
        print(f"❌ 加载分类文件失败：{e}")
        return
    
    # 生成同步指南
    sync_to_albums(CATEGORIES_FILE, USER_ID)


if __name__ == '__main__':
    main()
