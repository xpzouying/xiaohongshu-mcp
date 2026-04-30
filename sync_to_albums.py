#!/usr/bin/env python3
"""
小红书收藏专辑同步工具
将分类后的收藏笔记同步到小红书专辑
"""

import json
import requests
import time

# 小红书 API 端点
BASE_URL = "https://edith.xiaohongshu.com/api/sns/web/v1"

class XHSAlbumManager:
    """小红书专辑管理器"""
    
    def __init__(self, cookies_file='/tmp/cookies.json'):
        """初始化"""
        self.cookies = self._load_cookies(cookies_file)
        self.session = requests.Session()
        self._setup_session()
    
    def _load_cookies(self, cookies_file):
        """加载 cookies"""
        try:
            with open(cookies_file) as f:
                cookies_list = json.load(f)
            # 转换为 requests 格式
            return {c['name']: c['value'] for c in cookies_list}
        except Exception as e:
            print(f"❌ 加载 cookies 失败：{e}")
            return {}
    
    def _setup_session(self):
        """设置会话"""
        self.session.cookies.update(self.cookies)
        self.session.headers.update({
            'accept': 'application/json, text/plain, */*',
            'content-type': 'application/json',
            'referer': 'https://www.xiaohongshu.com/',
            'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        })
    
    def get_album_list(self, user_id):
        """获取专辑列表"""
        url = f"{BASE_URL}/folder/list"
        params = {
            'user_id': user_id,
            'type': 'collect'
        }
        
        try:
            resp = self.session.get(url, params=params, timeout=30)
            data = resp.json()
            
            if data.get('success'):
                albums = data.get('data', {}).get('folders', [])
                print(f"✅ 获取到 {len(albums)} 个专辑")
                return albums
            else:
                print(f"❌ 获取专辑列表失败：{data.get('error')}")
                return []
        except Exception as e:
            print(f"❌ 请求失败：{e}")
            return []
    
    def create_album(self, name, user_id):
        """创建专辑"""
        url = f"{BASE_URL}/folder"
        data = {
            'name': name,
            'user_id': user_id,
            'type': 'collect'
        }
        
        try:
            resp = self.session.post(url, json=data, timeout=30)
            result = resp.json()
            
            if result.get('success'):
                album_id = result.get('data', {}).get('id')
                print(f"✅ 创建专辑成功：{name} (ID: {album_id})")
                return album_id
            else:
                print(f"❌ 创建专辑失败：{result.get('error')}")
                return None
        except Exception as e:
            print(f"❌ 请求失败：{e}")
            return None
    
    def add_notes_to_album(self, album_id, note_ids):
        """批量添加笔记到专辑"""
        url = f"{BASE_URL}/note/collect/batch"
        data = {
            'folder_id': album_id,
            'note_ids': note_ids
        }
        
        try:
            resp = self.session.post(url, json=data, timeout=30)
            result = resp.json()
            
            if result.get('success'):
                print(f"✅ 成功添加 {len(note_ids)} 条笔记到专辑")
                return True
            else:
                print(f"❌ 添加笔记失败：{result.get('error')}")
                return False
        except Exception as e:
            print(f"❌ 请求失败：{e}")
            return False
    
    def sync_categories_to_albums(self, categories_file, user_id, dry_run=True):
        """
        将分类结果同步到专辑
        
        Args:
            categories_file: 分类结果 JSON 文件路径
            user_id: 用户 ID
            dry_run: 是否仅预览不实际执行
        """
        # 加载分类结果
        with open(categories_file) as f:
            data = json.load(f)
        
        categories = data.get('categories', {})
        
        print("=" * 70)
        print("📊 专辑同步预览")
        print("=" * 70)
        
        # 获取现有专辑
        print("\n📁 获取现有专辑...")
        existing_albums = self.get_album_list(user_id)
        existing_names = {album['name']: album['id'] for album in existing_albums}
        
        # 规划需要同步的分类
        print("\n📋 同步计划:\n")
        
        sync_plan = []
        for category, cat_data in categories.items():
            if category == '其他':
                continue  # 跳过"其他"类
            
            count = cat_data['count']
            if count == 0:
                continue
            
            # 检查专辑是否已存在
            if category in existing_names:
                album_id = existing_names[category]
                action = "更新"
            else:
                album_id = None
                action = "创建"
            
            sync_plan.append({
                'category': category,
                'count': count,
                'action': action,
                'album_id': album_id,
                'items': cat_data.get('items', [])
            })
            
            status = "✅ 已存在" if album_id else "🆕 待创建"
            print(f"  {status} 【{category}】{count}条笔记")
        
        print("\n" + "=" * 70)
        
        if dry_run:
            print("⚠️  预览模式（dry_run=True），未执行实际操作")
            print("   设置 dry_run=False 执行实际同步")
            return
        
        # 执行同步
        print("\n🚀 开始同步...\n")
        
        for plan in sync_plan:
            category = plan['category']
            count = plan['count']
            items = plan['items']
            
            print(f"【{category}】({count}条)")
            
            # 创建或获取专辑
            if plan['album_id']:
                album_id = plan['album_id']
            else:
                album_id = self.create_album(category, user_id)
                if not album_id:
                    print(f"  ❌ 创建专辑失败，跳过\n")
                    continue
                time.sleep(1)  # 避免请求过快
            
            # 提取笔记 ID
            note_ids = [item['feed_id'] for item in items if item.get('feed_id')]
            
            if not note_ids:
                print(f"  ⚠️  没有有效的笔记 ID，跳过\n")
                continue
            
            # 批量添加（每次最多 20 条）
            batch_size = 20
            for i in range(0, len(note_ids), batch_size):
                batch = note_ids[i:i+batch_size]
                print(f"  批次 {i//batch_size + 1}: 添加 {len(batch)} 条笔记...")
                
                success = self.add_notes_to_album(album_id, batch)
                if not success:
                    print(f"  ⚠️  部分失败，继续下一批\n")
                
                time.sleep(2)  # 避免请求过快
            
            print(f"  ✅ 完成\n")
            time.sleep(3)  # 每个分类之间等待
        
        print("=" * 70)
        print("✅ 同步完成！")
        print("=" * 70)


def main():
    """主函数"""
    # 配置
    CATEGORIES_FILE = '/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json'
    USER_ID = '620923cd000000002102474c'  # 你的用户 ID
    DRY_RUN = True  # 先预览，确认无误后改为 False
    
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
    
    # 创建管理器
    manager = XHSAlbumManager()
    
    # 执行同步
    manager.sync_categories_to_albums(
        categories_file=CATEGORIES_FILE,
        user_id=USER_ID,
        dry_run=DRY_RUN
    )


if __name__ == '__main__':
    main()
