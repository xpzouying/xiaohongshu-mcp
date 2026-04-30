#!/usr/bin/env python3
"""
小红书收藏专辑自动同步工具
自动创建专辑并将分类后的笔记移动到对应专辑
"""

import json
import subprocess
import sys
import time
import requests

BASE_URL = "http://localhost:18060"

def check_server():
    """检查服务器是否运行"""
    try:
        resp = requests.get(f"{BASE_URL}/health", timeout=5)
        return resp.json().get('success', False)
    except:
        return False

def sync_categories_to_albums(categories_file, dry_run=False):
    """
    同步分类结果到专辑
    
    Args:
        categories_file: 分类结果 JSON 文件
        dry_run: 是否仅预览
    """
    print("=" * 70)
    print("🚀 小红书收藏专辑自动同步工具")
    print("=" * 70)
    
    # 检查服务器
    if not check_server():
        print("❌ 服务器未运行，请先启动 xiaohongshu-mcp-local")
        return False
    
    print("✅ 服务器运行正常")
    
    # 加载分类结果
    try:
        with open(categories_file, 'r', encoding='utf-8') as f:
            data = json.load(f)
        print(f"✅ 分类文件加载成功：{data.get('total', 0)} 条笔记")
    except Exception as e:
        print(f"❌ 加载分类文件失败：{e}")
        return False
    
    categories = data.get('categories', {})
    
    # 获取现有专辑
    print("\n📁 获取现有专辑...")
    resp = requests.get(f"{BASE_URL}/api/v1/albums/list", timeout=30)
    existing_albums = {}
    if resp.status_code == 200:
        album_data = resp.json()
        if album_data.get('success'):
            for album in album_data.get('data', []):
                existing_albums[album['name']] = album['id']
            print(f"   现有专辑：{len(existing_albums)} 个")
            for name in existing_albums:
                print(f"     - {name}")
    
    # 规划同步
    print("\n📋 同步计划:\n")
    sync_plan = []
    
    for category, cat_data in sorted(categories.items(), key=lambda x: x[1]['count'], reverse=True):
        if category == '其他':
            continue
        
        count = cat_data['count']
        if count == 0:
            continue
        
        action = "创建" if category not in existing_albums else "更新"
        album_id = existing_albums.get(category)
        
        sync_plan.append({
            'category': category,
            'count': count,
            'action': action,
            'album_id': album_id,
            'items': cat_data.get('items', [])
        })
        
        status = "✅" if album_id else "🆕"
        print(f"   {status} {action}【{category}】{count}条")
    
    print("\n" + "=" * 70)
    
    if dry_run:
        print("⚠️  预览模式（dry_run=True）")
        print("   设置 dry_run=False 执行实际同步")
        return True
    
    # 执行同步
    print("\n🚀 开始自动同步...\n")
    
    success_count = 0
    fail_count = 0
    
    for plan in sync_plan:
        category = plan['category']
        count = plan['count']
        items = plan['items']
        
        print(f"【{category}】({count}条)")
        
        # 创建或获取专辑
        if plan['album_id']:
            album_id = plan['album_id']
            print(f"  使用现有专辑：{album_id}")
        else:
            print(f"  创建专辑...")
            resp = requests.post(
                f"{BASE_URL}/api/v1/albums/create",
                json={'name': category},
                timeout=30
            )
            
            if resp.status_code == 200:
                result = resp.json()
                if result.get('success'):
                    album_id = result['data'].get('id')
                    print(f"  ✅ 创建成功：{album_id}")
                else:
                    print(f"  ❌ 创建失败：{result.get('error')}")
                    fail_count += 1
                    continue
            else:
                print(f"  ❌ 请求失败：{resp.status_code}")
                fail_count += 1
                continue
        
        # 提取笔记 ID
        note_ids = [item['feed_id'] for item in items if item.get('feed_id')]
        
        if not note_ids:
            print(f"  ⚠️  没有有效的笔记 ID")
            continue
        
        # 批量添加（每次最多 20 条）
        batch_size = 20
        for i in range(0, len(note_ids), batch_size):
            batch = note_ids[i:i+batch_size]
            batch_num = i // batch_size + 1
            total_batches = (len(note_ids) + batch_size - 1) // batch_size
            
            print(f"  批次 {batch_num}/{total_batches}: 添加 {len(batch)} 条笔记...")
            
            resp = requests.post(
                f"{BASE_URL}/api/v1/albums/add_notes",
                json={
                    'album_id': album_id,
                    'note_ids': batch
                },
                timeout=60
            )
            
            if resp.status_code == 200:
                result = resp.json()
                if result.get('success'):
                    print(f"    ✅ 成功")
                else:
                    print(f"    ⚠️  部分失败：{result.get('error')}")
            else:
                print(f"    ❌ 请求失败：{resp.status_code}")
            
            time.sleep(2)  # 避免请求过快
        
        print(f"  ✅ 完成\n")
        success_count += 1
        time.sleep(3)  # 每个分类之间等待
    
    # 总结
    print("=" * 70)
    print(f"✅ 同步完成！")
    print(f"   成功：{success_count} 个专辑")
    print(f"   失败：{fail_count} 个专辑")
    print("=" * 70)
    
    return True


def main():
    """主函数"""
    # 配置
    CATEGORIES_FILE = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json'
    DRY_RUN = False  # 设置为 True 仅预览，False 执行实际同步
    
    print("🔍 小红书收藏专辑自动同步工具")
    print("=" * 70)
    
    # 检查文件
    try:
        with open(CATEGORIES_FILE) as f:
            data = json.load(f)
        print(f"✅ 分类文件加载成功：{data.get('total', 0)} 条笔记")
    except Exception as e:
        print(f"❌ 加载分类文件失败：{e}")
        return
    
    # 确认执行
    if not DRY_RUN:
        print("\n⚠️  警告：即将执行实际同步操作")
        print("   这将自动创建专辑并移动笔记")
        print("\n💡 提示：由于小红书 API 限制，建议先使用 dry_run=True 预览")
        print("   然后在小红书网页版手动操作，或确保已正确配置 API 认证")
        print()
        # 非交互模式，直接执行
        # response = input("是否继续？(y/N): ")
        # if response.lower() != 'y':
        #     print("已取消")
        #     return
    
    # 执行同步
    sync_categories_to_albums(CATEGORIES_FILE, dry_run=DRY_RUN)


if __name__ == '__main__':
    main()
