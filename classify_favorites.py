#!/usr/bin/env python3
"""
小红书收藏笔记 AI 分类工具
自动将收藏笔记分类到不同的专辑
"""

import json
import re
from collections import defaultdict

# 定义分类规则
CATEGORIES = {
    '美食烹饪': {
        'keywords': [
            '吃', '菜', '汤', '早餐', '煎', '美食', '做饭', '炒', '炖', '煮', '蒸', '烤', 
            '烘焙', '食谱', '做法', '好吃', '美味', '口蘑', '牛肉', '芥蓝', '虾滑', 
            '南瓜', '鸡翅', '黄焖鸡', '米饭', '粉丝', '茶餐厅', '咸蛋黄', '巨好喝', 
            '复刻', '绝绝子', '配饭', '咸香', '下饭', '鲜', '爆汁', 'q 弹', '焗', '炸',
            '煲仔饭', '绿豆饼', '土豆', '酸菜', '肉末', '米粉', '凉拌', '手撕鸡',
            '咕噜肉', '酱油', '烧肉', '番茄', '鸡蛋', '滑蛋', '盖饭', '红烧', '排骨',
            '手撕包菜', '小龙虾', '蒜蓉', '蒸蛋', '板栗', '粥', '酸爽', '开胃', '料理',
            '费米', '出师', '女孩子', '晚餐', '先放', '蛋', '包菜', '孩子'
        ],
        'weight': 2,
    },
    '育儿母婴': {
        'keywords': [
            '宝宝', '育儿', '婴儿', '儿童', '孩子', '母婴', '剃头', '发型', '月龄', 
            '胎发', '咳嗽', '排痰', '推拿', '鼻塞', '哄睡', '自主入睡', '省妈', 
            '崔玉涛', '顺产', '孕晚期', '待产包', '新生儿', '孕妈', '分娩', '产后', 
            '一胎', '二胎', '宝妈', '带娃', '养护', '蛇宝宝', '遇秋则贵', '特种婴', 
            '抱睡', '奶睡', '辅食', '奶粉', '便秘', '怀孕', '证件', '建档', '老公',
            '注意事项', '产科', '老婆', '孕期', '催', '偏', '亲测', '有效'
        ],
        'weight': 2,
    },
    '汽车交通': {
        'keywords': [
            '车', '汽车', '提车', '博越', '车牌', '驾驶', '停车', '侧方', '驾照', 
            '贴膜', '4s 店', '车主', '吉利', '领克', '畅悦', '功能', '闲置', '必看',
            '砍价', '销售', '星越', '坦克', '绿牌', '选号', '心仪', '男友', '思路',
            '非刚需', '规划', '海', '青春', '热水器'
        ],
        'weight': 2,
    },
    '股票理财': {
        'keywords': [
            '股票', '炒股', '基金', '理财', '涨停', '跌停', '集合竞价', '抓涨停', 
            'a 股', '交易', '选号', '自编', '财经', '赚钱', '投资', 'etf', '场内',
            '小白', '入门', '知识', '账户', '权限', '开通', '规则', '板块', '招商',
            '证券', '开户', '保险', '惠民', '报销', '补贴', '尿酸', '饮食'
        ],
        'weight': 2,
    },
    '电商创业': {
        'keywords': [
            '拼多多', '电商', '无货源', '店', '信息差', '创业', '副业', '开店', 
            '新手', '一周', '干货', '复盘', '变现'
        ],
        'weight': 2,
    },
    '健康医疗': {
        'keywords': [
            '健康', '医生', '心理咨询', '医院', '看病', '治疗', '症状', '药物', 
            '疫苗', '体检', '心理', '小时', '咨询', '眼睛', '胀痛', '疲劳', '缓解',
            '中药', '煎服', '方法', '耳前', '瘘管', '发炎', '开掉', '螨虫', '咬',
            '判断', '高尿酸', '建议'
        ],
        'weight': 2,
    },
    '家居生活': {
        'keywords': [
            '床', '洞洞板', '家居', '装修', '软装', '收纳', '整理', '软床', '竹席', 
            '说明书', '好用', '变硬'
        ],
        'weight': 2,
    },
    '技能学习': {
        'keywords': [
            '教程', '学习', '技巧', '方法', '指南', '必看', '学会', '教学', '攻略', 
            '新手', '十秒', '分钟', '搞定', '轻松', '一篇', '清楚', '知道', '告诉'
        ],
        'weight': 1,
    },
    '娱乐搞笑': {
        'keywords': ['搞笑', '幽默', '笑死', '哈哈', '🤣', '😂', '实力', '不用多说'],
        'weight': 1,
    },
}

def classify_note(title, desc=''):
    """
    对单条笔记进行分类
    返回：(分类名称，置信度)
    """
    text = (title + ' ' + desc).lower()
    title_lower = title.lower()
    
    scores = defaultdict(float)
    
    for category, config in CATEGORIES.items():
        score = 0
        matched_keywords = []
        
        for keyword in config['keywords']:
            if keyword in text:
                score += config['weight']
                matched_keywords.append(keyword)
        
        # 标题中的关键词权重更高
        for keyword in config['keywords']:
            if keyword in title_lower:
                score *= 1.3
        
        # 如果匹配到多个关键词，额外加分
        if len(matched_keywords) >= 2:
            score *= 1.5
        if len(matched_keywords) >= 3:
            score *= 1.5
        
        if score > 0:
            scores[category] = score
    
    if not scores:
        return '其他', 0.0
    
    # 返回得分最高的分类
    best_category = max(scores, key=scores.get)
    confidence = scores[best_category]
    
    # 归一化置信度到 0-1
    max_possible = max(len(CATEGORIES[cat]['keywords']) * CATEGORIES[cat]['weight'] * 2 
                       for cat in CATEGORIES)
    normalized_confidence = min(1.0, confidence / (max_possible * 0.15))
    
    return best_category, normalized_confidence

def main():
    # 加载收藏数据
    with open('/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/favorites_145.json') as f:
        data = json.load(f)
    
    items = data['data']['items']
    
    # 分类统计
    categorized = defaultdict(list)
    uncategorized = []
    
    print("🔍 开始 AI 分类...\n")
    
    for i, item in enumerate(items, 1):
        title = item.get('title', '')
        desc = item.get('desc', '')
        
        category, confidence = classify_note(title, desc)
        
        result = {
            **item,
            'category': category,
            'confidence': round(confidence, 2),
        }
        
        if category == '其他' or confidence < 0.15:
            uncategorized.append(result)
        else:
            categorized[category].append(result)
        
        if i % 20 == 0:
            print(f"  已处理 {i}/{len(items)} 条...")
    
    # 输出分类结果
    print("\n📊 分类结果:\n")
    print(f"{'分类':<15} {'数量':>6} {'占比':>8}")
    print("-" * 35)
    
    total = len(items)
    for category in sorted(categorized.keys(), key=lambda x: len(categorized[x]), reverse=True):
        count = len(categorized[category])
        pct = count / total * 100
        print(f"{category:<15} {count:>6} {pct:>7.1f}%")
    
    if uncategorized:
        pct = len(uncategorized) / total * 100
        print(f"{'其他':<15} {len(uncategorized):>6} {pct:>7.1f}%")
    
    print("-" * 35)
    print(f"{'总计':<15} {total:>6} {100.0:>7.1f}%")
    
    # 保存分类结果
    output = {
        'total': total,
        'categories': {},
        'items': [],
    }
    
    for category, items_list in categorized.items():
        output['categories'][category] = {
            'count': len(items_list),
            'items': items_list,
        }
    
    if uncategorized:
        output['categories']['其他'] = {
            'count': len(uncategorized),
            'items': uncategorized,
        }
    
    # 保存为 JSON
    with open('/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json', 'w', encoding='utf-8') as f:
        json.dump(output, f, ensure_ascii=False, indent=2)
    
    print(f"\n✅ 分类结果已保存到：/root/.openclaw/workspace-pm/projects/xiaohongshu-mcp/收藏分类结果.json")
    
    # 显示每个分类的前 3 条
    print("\n📁 各分类示例:\n")
    for category in sorted(categorized.keys(), key=lambda x: len(categorized[x]), reverse=True)[:5]:
        print(f"【{category}】({len(categorized[category])}条)")
        for i, item in enumerate(categorized[category][:3], 1):
            title = item.get('title', '无标题')[:50]
            conf = item.get('confidence', 0)
            print(f"  {i}. {title} (置信度：{conf:.2f})")
        print()

if __name__ == '__main__':
    main()
