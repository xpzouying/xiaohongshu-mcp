#!/usr/bin/env python3
"""
小红书收藏专辑自动同步工具 - API 版本
直接调用 MCP 服务器 API，无需浏览器自动化

使用方法:
    python3 auto_sync_api.py

依赖:
    - MCP 服务器运行中 (localhost:18060)
    - 收藏分类结果.json
"""

import json
import requests
import time
import sys
from datetime import datetime

BASE_URL = "http://localhost:18060"

def log_info(msg):
    print(f"[INFO] {msg}")

def log_success(msg):
    print(f"[SUCCESS] {msg}")

def log_error(msg):
    print(f"[ERROR] {msg}")

def log_progress(current, total, album_name):
    """显示进度"""
    percent = (current / total * 100) if total > 0 else 0
    print(f"\r  进度：{current}/{total} ({percent:.1f}%) - {album_name}", end='', flush=True)

def check_server_health():
    """检查服务器健康状态"""
    try:
        resp = requests.get(f"{BASE_URL}/health", timeout=5)
        data = resp.json()
        if data.get('success') and data.get('data', {}).get('status') == 'healthy':
            log_success("MCP 服务器运行正常")
            return True
        else:
            log_error("MCP 服务器状态异常")
            return False
    except Exception as e:
        log_error(f"无法连接 MCP 服务器：{e}")
        return False

def load_categories(file_path):
    """加载分类结果"""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        log_success(f"分类文件加载成功：{data.get('total', 0)} 条笔记")
        return data
    except Exception as e:
        log_error(f"加载分类文件失败：{e}")
        return None

def get_album_list():
    """获取专辑列表"""
    try:
        resp = requests.get(f"{BASE_URL}/api/v1/albums/list", timeout=10)
        data = resp.json()
        if data.get('success'):
            return data.get('data', [])
        return []
    except Exception as e:
        log_error(f"获取专辑列表失败：{e}")
        return []

def create_album(name):
    """创建专辑"""
    try:
        resp = requests.post(
            f"{BASE_URL}/api/v1/albums/create",
            json={"name": name},
            timeout=120  # 增加超时到 120 秒
        )
        data = resp.json()
        if data.get('success'):
            album_data = data.get('data', {})
            log_success(f"专辑创建成功：{name} (ID: {album_data.get('id', 'N/A')})")
            return album_data.get('id')
        else:
            log_error(f"创建专辑失败：{data.get('message', 'Unknown error')}")
            return None
    except Exception as e:
        log_error(f"创建专辑异常：{e}")
        return None

def add_notes_to_album(album_id, note_ids):
    """添加笔记到专辑"""
    try:
        resp = requests.post(
            f"{BASE_URL}/api/v1/albums/add_notes",
            json={"album_id": album_id, "note_ids": note_ids},
            timeout=120  # 增加超时到 120 秒
        )
        data = resp.json()
        if data.get('success'):
            return True
        else:
            log_error(f"添加笔记失败：{data.get('message', 'Unknown error')}")
            return False
    except Exception as e:
        log_error(f"添加笔记异常：{e}")
        return False

def sync_albums(categories_data):
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
    
    log_info(f"开始同步 {len(albums_to_sync)} 个专辑...\n")
    
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
            log_error("  没有有效的笔记 ID")
            sync_results['albums'].append({
                'name': album_name,
                'count': count,
                'success': False,
                'message': '没有有效的笔记 ID'
            })
            sync_results['failed'] += 1
            continue
        
        # 创建专辑
        album_id = create_album(album_name)
        if not album_id:
            # 尝试使用专辑名称作为 ID（如果 API 返回占位符）
            album_id = f"album_{album_name}"
        
        # 批量添加笔记（每批 20 条）
        batch_size = 20
        success_count = 0
        failed_count = 0
        
        for i in range(0, len(note_ids), batch_size):
            batch = note_ids[i:i + batch_size]
            batch_num = i // batch_size + 1
            total_batches = (len(note_ids) + batch_size - 1) // batch_size
            
            print(f"\r  批次 {batch_num}/{total_batches}: ", end='', flush=True)
            
            if add_notes_to_album(album_id, batch):
                success_count += len(batch)
                print(f"成功 {len(batch)} 条", end='', flush=True)
            else:
                failed_count += len(batch)
                print(f"失败 {len(batch)} 条", end='', flush=True)
            
            # 避免请求过快
            time.sleep(2)
        
        print()  # 换行
        
        album_success = success_count > len(note_ids) * 0.8  # 80% 成功率
        
        if album_success:
            log_success(f"  完成：{success_count}/{len(note_ids)} 条笔记")
            sync_results['success'] += 1
        else:
            log_error(f"  失败：{failed_count}/{len(note_ids)} 条笔记")
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
        log_success(f"同步报告已保存：{report_file}")
        
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
        log_success(f"同步摘要已保存：{summary_file}")
        
    except Exception as e:
        log_error(f"保存报告失败：{e}")

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
    print("🚀 小红书收藏专辑自动同步工具 (API 版本)")
    print("=" * 70)
    print()
    
    # 1. 检查服务器
    log_info("检查 MCP 服务器状态...")
    if not check_server_health():
        log_error("MCP 服务器未运行或无法连接")
        log_info("请先启动 MCP 服务器：./xiaohongshu-mcp-local")
        sys.exit(1)
    
    # 2. 加载分类
    categories_file = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json'
    log_info(f"加载分类文件：{categories_file}")
    categories_data = load_categories(categories_file)
    if not categories_data:
        sys.exit(1)
    
    # 3. 显示同步计划
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
    
    # 4. 执行同步
    sync_results = sync_albums(categories_data)
    
    # 5. 保存报告
    report_file = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/专辑同步报告_API.json'
    save_report(sync_results, report_file)
    
    # 6. 打印摘要
    print_summary(sync_results)
    
    # 7. 退出码
    sys.exit(0 if sync_results['failed'] == 0 else 1)

if __name__ == '__main__':
    main()
