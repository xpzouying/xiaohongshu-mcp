#!/usr/bin/env python3
"""
小红书收藏专辑自动同步工具 - 直接 API 版本
使用 MCP 服务器的 cookies 直接调用小红书 API，无需浏览器自动化

使用方法:
    python3 auto_sync_direct.py

依赖:
    - MCP 服务器的 cookies 文件
    - 收藏分类结果.json
"""

import json
import requests
import time
import sys
import os
from datetime import datetime

# 小红书 API 端点
API_BASE = "https://edith.xiaohongshu.com/api/sns/web/v1"

# Cookies 文件路径
COOKIE_PATHS = [
    '/tmp/cookies.json',
    '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/cookies.json',
    '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/cookies/cookies.json',
    os.path.expanduser('~/.config/xiaohongshu-mcp/cookies.json'),
]

def get_cookies():
    """加载 cookies"""
    for path in COOKIE_PATHS:
        if os.path.exists(path):
            try:
                with open(path, 'r') as f:
                    cookies_data = json.load(f)
                # 转换为 requests cookie jar
                cookie_jar = requests.utils.cookiejar_from_dict({
                    c['name']: c['value'] for c in cookies_data
                })
                print(f"[SUCCESS] Cookies 加载成功：{path}")
                return cookie_jar
            except Exception as e:
                print(f"[WARN] 加载 cookies 失败 {path}: {e}")
    print("[ERROR] 未找到 cookies 文件")
    return None

def create_session(cookies):
    """创建会话"""
    session = requests.Session()
    session.cookies.update(cookies)
    session.headers.update({
        'Content-Type': 'application/json',
        'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        'Origin': 'https://www.xiaohongshu.com',
        'Referer': 'https://www.xiaohongshu.com/'
    })
    return session

def get_album_list(session):
    """获取专辑列表"""
    try:
        resp = session.get(f"{API_BASE}/folder/list", timeout=10)
        data = resp.json()
        if data.get('success') or data.get('code') == 0:
            folders = data.get('data', {}).get('folders', [])
            print(f"[INFO] 获取到 {len(folders)} 个专辑")
            return folders
        else:
            print(f"[ERROR] 获取专辑列表失败：{data}")
            return []
    except Exception as e:
        print(f"[ERROR] 获取专辑列表异常：{e}")
        return []

def create_album(session, name):
    """创建专辑"""
    try:
        resp = session.post(
            f"{API_BASE}/folder",
            json={
                'name': name,
                'type': 'collect'
            },
            timeout=10
        )
        data = resp.json()
        if data.get('success') or data.get('code') == 0:
            folder_data = data.get('data', {})
            folder_id = folder_data.get('id') or folder_data.get('folder_id')
            print(f"[SUCCESS] 专辑创建成功：{name} (ID: {folder_id})")
            return folder_id
        else:
            print(f"[ERROR] 创建专辑失败 {name}: {data}")
            return None
    except Exception as e:
        print(f"[ERROR] 创建专辑异常 {name}: {e}")
        return None

def add_notes_to_album(session, album_id, note_ids):
    """添加笔记到专辑"""
    try:
        resp = session.post(
            f"{API_BASE}/note/collect/batch",
            json={
                'folder_id': album_id,
                'note_ids': note_ids
            },
            timeout=30
        )
        data = resp.json()
        if data.get('success') or data.get('code') == 0:
            return True
        else:
            print(f"[ERROR] 添加笔记失败：{data}")
            return False
    except Exception as e:
        print(f"[ERROR] 添加笔记异常：{e}")
        return False

def load_categories(file_path):
    """加载分类结果"""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        print(f"[SUCCESS] 分类文件加载成功：{data.get('total', 0)} 条笔记")
        return data
    except Exception as e:
        print(f"[ERROR] 加载分类文件失败：{e}")
        return None

def sync_albums(session, categories_data):
    """执行专辑同步"""
    categories = categories_data.get('categories', {})
    total_notes = categories_data.get('total', 0)
    
    sync_results = {
        'total': total_notes,
        'albums': [],
        'success': 0,
        'failed': 0,
        'timestamp': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
        'status': 'completed'
    }
    
    # 过滤掉"其他"分类
    albums_to_sync = [
        (name, data) for name, data in categories.items()
        if name != '其他' and isinstance(data, dict) and data.get('count', 0) > 0
    ]
    
    # 按笔记数量排序（从多到少）
    albums_to_sync.sort(key=lambda x: x[1]['count'], reverse=True)
    
    print(f"\n[INFO] 开始同步 {len(albums_to_sync)} 个专辑...\n")
    
    # 先获取现有专辑列表
    print("[INFO] 获取现有专辑列表...")
    existing_albums = get_album_list(session)
    album_name_to_id = {
        album.get('name'): album.get('id') or album.get('folder_id')
        for album in existing_albums
    }
    
    for album_name, album_data in albums_to_sync:
        count = album_data['count']
        items = album_data.get('items', [])
        
        print(f"\n📁 同步专辑：{album_name} ({count} 条)")
        print("-" * 50)
        
        # 提取笔记 ID
        note_ids = []
        for item in items:
            if isinstance(item, dict) and 'feed_id' in item:
                note_ids.append(item['feed_id'])
        
        if not note_ids:
            print("[ERROR]   没有有效的笔记 ID")
            sync_results['albums'].append({
                'name': album_name,
                'count': count,
                'success': False,
                'message': '没有有效的笔记 ID'
            })
            sync_results['failed'] += 1
            continue
        
        # 获取或创建专辑
        album_id = album_name_to_id.get(album_name)
        if album_id:
            print(f"[INFO]   使用现有专辑：{album_name} (ID: {album_id})")
        else:
            print(f"[INFO]   创建新专辑：{album_name}")
            album_id = create_album(session, album_name)
            if album_id:
                album_name_to_id[album_name] = album_id
                # 等待专辑创建生效
                time.sleep(2)
        
        if not album_id:
            print(f"[ERROR]   专辑创建失败")
            sync_results['albums'].append({
                'name': album_name,
                'count': count,
                'success': False,
                'message': '专辑创建失败'
            })
            sync_results['failed'] += 1
            continue
        
        # 批量添加笔记（每批 20 条）
        batch_size = 20
        success_count = 0
        failed_count = 0
        
        for i in range(0, len(note_ids), batch_size):
            batch = note_ids[i:i + batch_size]
            batch_num = i // batch_size + 1
            total_batches = (len(note_ids) + batch_size - 1) // batch_size
            
            print(f"\r  批次 {batch_num}/{total_batches}: ", end='', flush=True)
            
            if add_notes_to_album(session, album_id, batch):
                success_count += len(batch)
                print(f"成功 {len(batch)} 条", end='', flush=True)
            else:
                failed_count += len(batch)
                print(f"失败 {len(batch)} 条", end='', flush=True)
            
            # 避免请求过快
            time.sleep(2)
        
        print()  # 换行
        
        album_success = success_count > len(note_ids) * 0.8  # 80% 成功率即认为成功
        
        if album_success:
            print(f"[SUCCESS]   完成：{success_count}/{len(note_ids)} 条笔记")
            sync_results['success'] += 1
        else:
            print(f"[ERROR]   失败：{failed_count}/{len(note_ids)} 条笔记")
            sync_results['failed'] += 1
        
        sync_results['albums'].append({
            'name': album_name,
            'count': count,
            'album_id': album_id,
            'success': album_success,
            'message': f'成功{success_count}条，失败{failed_count}条',
            'note_ids': note_ids
        })
        
        # 专辑之间等待
        time.sleep(3)
    
    return sync_results

def save_report(sync_results, report_file):
    """保存同步报告"""
    try:
        # 保存完整报告
        with open(report_file, 'w', encoding='utf-8') as f:
            json.dump(sync_results, f, ensure_ascii=False, indent=2)
        print(f"[SUCCESS] 同步报告已保存：{report_file}")
        
        # 保存摘要
        summary_file = report_file.replace('.json', '_摘要.json')
        summary = {
            'total': sync_results['total'],
            'success': sync_results['success'],
            'failed': sync_results['failed'],
            'timestamp': sync_results['timestamp'],
            'status': sync_results['status']
        }
        with open(summary_file, 'w', encoding='utf-8') as f:
            json.dump(summary, f, ensure_ascii=False, indent=2)
        print(f"[SUCCESS] 同步摘要已保存：{summary_file}")
        
    except Exception as e:
        print(f"[ERROR] 保存报告失败：{e}")

def print_summary(sync_results):
    """打印摘要"""
    print("\n" + "=" * 70)
    print("📊 同步结果摘要")
    print("=" * 70)
    print(f"\n总专辑数：{len(sync_results['albums'])}")
    print(f"成功：{sync_results['success']} 个")
    print(f"失败：{sync_results['failed']} 个")
    print(f"总笔记：{sync_results['total']} 条")
    print(f"时间：{sync_results['timestamp']}")
    
    print("\n专辑详情:")
    for album in sync_results['albums']:
        status = "✅" if album['success'] else "❌"
        print(f"  {status} {album['name']}: {album['count']} 条 - {album['message']}")
    
    print("\n" + "=" * 70)
    
    if sync_results['failed'] == 0:
        print("🎉 所有专辑同步成功！")
    else:
        print(f"⚠️  有 {sync_results['failed']} 个专辑同步失败")
    print("=" * 70)

def main():
    print("=" * 70)
    print("🚀 小红书收藏专辑自动同步工具 (直接 API 版本)")
    print("=" * 70)
    print()
    
    # 1. 加载 cookies
    print("[INFO] 加载 cookies...")
    cookies = get_cookies()
    if not cookies:
        print("[ERROR] 无法加载 cookies，请确保 MCP 服务器已运行并登录")
        sys.exit(1)
    
    # 2. 创建会话
    session = create_session(cookies)
    
    # 3. 测试连接
    print("\n[INFO] 测试 API 连接...")
    test_albums = get_album_list(session)
    if test_albums is None:
        print("[ERROR] API 连接失败，请检查 cookies 是否有效")
        sys.exit(1)
    print(f"[SUCCESS] API 连接成功，现有 {len(test_albums)} 个专辑")
    
    # 4. 加载分类
    categories_file = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json'
    print(f"\n[INFO] 加载分类文件：{categories_file}")
    categories_data = load_categories(categories_file)
    if not categories_data:
        sys.exit(1)
    
    # 5. 显示同步计划
    print("\n📋 同步计划:\n")
    categories = categories_data.get('categories', {})
    for name, data in sorted(categories.items(), key=lambda x: x[1]['count'] if isinstance(x[1], dict) else 0, reverse=True):
        if name == '其他':
            continue
        if isinstance(data, dict):
            count = data.get('count', 0)
            if count > 0:
                print(f"   📁 {name}: {count} 条")
    
    print("\n" + "=" * 70)
    print("开始同步...\n")
    
    # 6. 执行同步
    sync_results = sync_albums(session, categories_data)
    
    # 7. 保存报告
    report_file = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/专辑同步报告_直接 API.json'
    save_report(sync_results, report_file)
    
    # 8. 打印摘要
    print_summary(sync_results)
    
    # 9. 退出码
    sys.exit(0 if sync_results['failed'] == 0 else 1)

if __name__ == '__main__':
    main()
