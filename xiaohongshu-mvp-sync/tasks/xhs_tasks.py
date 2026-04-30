"""同步任务定义：分类数据解析 + Agent 任务指令生成"""
import json
from pathlib import Path
from typing import Dict, List


def load_classification_data(data_path: Path) -> Dict[str, List[str]]:
    """读取 收藏分类结果.json，返回 {分类名: [note_ids]}

    支持三种格式：
    1. {"分类A": [{"id": "..."}, ...], ...}
    2. {"分类": ["note_id_1", ...], ...}
    3. {"total": N, "categories": {"分类A": {"count": N, "items": [{"feed_id": "..."}, ...]}}}
    """
    with open(data_path, "r", encoding="utf-8") as f:
        raw = json.load(f)

    result = {}

    # 格式 3: 嵌套 categories 结构
    if "categories" in raw:
        for category, cat_data in raw["categories"].items():
            items = cat_data.get("items", []) if isinstance(cat_data, dict) else cat_data
            if items and isinstance(items[0], dict):
                result[category] = [
                    item.get("feed_id", item.get("id", item.get("note_id", "")))
                    for item in items
                ]
            else:
                result[category] = items
        return result

    # 格式 1 & 2: 顶层就是分类
    for category, items in raw.items():
        if not isinstance(items, list):
            continue
        if items and isinstance(items[0], dict):
            result[category] = [
                item.get("id", item.get("note_id", item.get("feed_id", "")))
                for item in items
            ]
        else:
            result[category] = items
    return result


SYNC_TASK_TEMPLATE = """你现在是小红书网页版的自动化助手，已登录到 www.xiaohongshu.com。

任务：创建专辑并移动笔记

步骤：
1. 导航到我的收藏页面：https://www.xiaohongshu.com/user/favorite
2. 找到"创建专辑"按钮，创建一个名为「{album_name}」的专辑
   - 如果专辑已存在，直接使用已有的
3. 将以下笔记移入这个专辑：
{note_list}
4. 逐一操作，完成后确认所有笔记都已移入
5. 报告最终结果：成功移动 X 条，失败 Y 条

注意：
- 每步操作后等待页面加载
- 遇到弹窗点击确认或关闭
- 用中文报告结果
"""


def build_sync_task(album_name: str, note_ids: list[str]) -> str:
    """生成 Agent 任务指令"""
    note_list = "\n".join(f"   - ID: {nid}" for nid in note_ids[:20])  # 最多显示20条
    if len(note_ids) > 20:
        note_list += f"\n   ... 共 {len(note_ids)} 条"
    return SYNC_TASK_TEMPLATE.format(
        album_name=album_name,
        note_list=note_list,
    )
