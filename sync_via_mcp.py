#!/usr/bin/env python3
"""通过 MCP API 同步专辑（带重试和超时优化）"""

import requests
import json
import time
from datetime import datetime

BASE_URL = "http://localhost:18060"

def log(msg):
    print(f"[{datetime.now().strftime('%H:%M:%S')}] {msg}")

def create_album(name, max_retries=3, timeout=300):
    """创建专辑（带重试）"""
    for attempt in range(max_retries):
        try:
            log(f"  创建专辑：{name} (尝试 {attempt+1}/{max_retries})")
            resp = requests.post(
                f"{BASE_URL}/api/v1/albums/create",
                json={"name": name},
                timeout=timeout
            )
            data = resp.json()
            if data.get('success'):
                album_id = data.get('data', {}).get('id', name)
                log(f"  ✅ 创建成功：{name} (ID: {album_id})")
                return album_id
            else:
                log(f"  ⚠️  创建失败：{data.get('message', 'Unknown')}")
                return name  # 使用名称作为 ID 继续
        except requests.exceptions.Timeout:
            log(f"  ⏱️  超时，重试中...")
            time.sleep(5)
        except Exception as e:
            log(f"  ❌ 错误：{e}")
            if attempt == max_retries - 1:
                return name
            time.sleep(2)
    return name

def add_notes_to_album(album_name, note_ids, timeout=300):
    """添加笔记到专辑"""
    try:
        log(f"  添加 {len(note_ids)} 条笔记到 {album_name}")
        resp = requests.post(
            f"{BASE_URL}/api/v1/albums/add_notes",
            json={"album_name": album_name, "note_ids": note_ids},
            timeout=timeout
        )
        data = resp.json()
        if data.get('success'):
            log(f"  ✅ 添加成功")
            return len(note_ids), 0
        else:
            log(f"  ❌ 添加失败：{data.get('message', 'Unknown')}")
            return 0, len(note_ids)
    except Exception as e:
        log(f"  ❌ 错误：{e}")
        return 0, len(note_ids)

def main():
    log("=" * 70)
    log("🚀 通过 MCP API 同步专辑")
    log("=" * 70)
    
    # 检查服务器
    log("检查 MCP 服务器...")
    try:
        resp = requests.get(f"{BASE_URL}/health", timeout=5)
        if not resp.json().get('success'):
            log("❌ 服务器未就绪")
            return
        log("✅ 服务器正常")
    except Exception as e:
        log(f"❌ 无法连接服务器：{e}")
        return
    
    # 加载分类
    log("加载分类文件...")
    with open('/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json', 'r', encoding='utf-8') as f:
        data = json.load(f)
    
    total = data.get('total', 0)
    categories = data.get('categories', {})
    log(f"✅ 加载成功：{total} 条笔记")
    
    # 显示计划
    log("\n📋 同步计划:")
    for name, cat in sorted(categories.items(), key=lambda x: x[1]['count'] if isinstance(x[1], dict) else 0, reverse=True):
        if name == '其他' or not isinstance(cat, dict):
            continue
        count = cat.get('count', 0)
        if count > 0:
            log(f"   📁 {name}: {count} 条")
    
    log("\n" + "=" * 70)
    log("开始同步...\n")
    
    # 同步
    results = []
    success_count = 0
    failed_count = 0
    
    for category, cat_data in categories.items():
        if category == '其他' or not isinstance(cat_data, dict):
            continue
        
        count = cat_data.get('count', 0)
        items = cat_data.get('items', [])
        if count == 0:
            continue
        
        log(f"\n📁 {category} ({count} 条)")
        log("-" * 50)
        
        # 提取笔记 ID
        note_ids = [item['feed_id'] for item in items if isinstance(item, dict) and 'feed_id' in item]
        if not note_ids:
            log("  ⚠️  无有效笔记 ID")
            continue
        
        # 创建专辑
        album_id = create_album(category)
        
        # 添加笔记
        success, failed = add_notes_to_album(album_id, note_ids)
        
        album_success = success > len(note_ids) * 0.8
        if album_success:
            success_count += 1
            log(f"  ✅ 完成：成功 {success}/{len(note_ids)} 条")
        else:
            failed_count += 1
            log(f"  ❌ 失败：成功 {success}/{len(note_ids)} 条")
        
        results.append({
            'name': category,
            'count': count,
            'success': album_success,
            'success_count': success,
            'failed_count': failed
        })
        
        # 等待
        time.sleep(3)
    
    # 结果
    log("\n" + "=" * 70)
    log("📊 同步结果")
    log("=" * 70)
    log(f"总专辑：{len(results)}")
    log(f"成功：{success_count}")
    log(f"失败：{failed_count}")
    
    for r in results:
        status = "✅" if r['success'] else "❌"
        log(f"  {status} {r['name']}: {r['success_count']}/{r['count']}")
    
    log("=" * 70)
    
    # 保存报告
    report = {
        'total': total,
        'success': success_count,
        'failed': failed_count,
        'albums': results,
        'timestamp': datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    }
    
    with open('/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/专辑同步报告_MCP.json', 'w', encoding='utf-8') as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    
    log(f"\n📄 报告已保存：专辑同步报告_MCP.json")
    
    if failed_count == 0:
        log("\n🎉 所有专辑同步成功！")
    else:
        log(f"\n⚠️  有 {failed_count} 个专辑同步失败")

if __name__ == '__main__':
    main()
