# 小红书收藏专辑自动同步功能

## 📊 功能概述

本功能可以自动将分类后的收藏笔记同步到小红书专辑，实现：
- ✅ 自动创建专辑
- ✅ 批量移动笔记到专辑
- ✅ 智能分类管理

## 📁 文件说明

| 文件 | 说明 |
|------|------|
| `album_manager.go` | Go 语言专辑管理模块 |
| `album_sync_tool.py` | Python 同步工具 |
| `收藏分类结果.json` | AI 分类结果数据 |
| `收藏分类结果_专辑同步清单.md` | 手动同步清单 |

## 🚀 使用方法

### 方法一：自动同步（推荐先预览）

```bash
# 1. 启动服务器
cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp
./xiaohongshu-mcp-local &

# 2. 预览同步计划（不执行实际操作）
python3 album_sync_tool.py

# 3. 执行实际同步
# 修改 album_sync_tool.py 中 DRY_RUN = False
# 然后运行
python3 album_sync_tool.py
```

### 方法二：手动同步（当前最可靠）

由于小红书 API 限制，建议使用手动方式：

1. **打开同步清单**
   ```bash
   cat 收藏分类结果_专辑同步清单.md
   ```

2. **访问收藏页面**
   ```
   https://www.xiaohongshu.com/user/profile/620923cd000000002102474c?tab=fav&subTab=note
   ```

3. **创建专辑**
   - 美食烹饪 (65 条)
   - 育儿母婴 (18 条)
   - 汽车交通 (15 条)
   - 股票理财 (12 条)
   - 健康医疗 (4 条)
   - 家居生活 (1 条)
   - 电商创业 (1 条)

4. **移动笔记**
   - 使用清单中的笔记 ID 搜索定位
   - 批量选择后移动到对应专辑

## 📋 需要同步的专辑

| 专辑名称 | 笔记数 | 优先级 |
|---------|--------|--------|
| 美食烹饪 | 65 条 | ⭐⭐⭐ |
| 育儿母婴 | 18 条 | ⭐⭐⭐ |
| 汽车交通 | 15 条 | ⭐⭐ |
| 股票理财 | 12 条 | ⭐⭐ |
| 健康医疗 | 4 条 | ⭐ |
| 家居生活 | 1 条 | ⭐ |
| 电商创业 | 1 条 | ⭐ |

## ⚠️ 注意事项

1. **API 限制**
   - 小红书网页版 API 可能有频率限制
   - 建议分批操作，每批不超过 20 条
   - 操作间隔建议 2-3 秒

2. **认证问题**
   - 确保 cookies 有效
   - 如 API 调用失败，请使用手动方式

3. **数据备份**
   - 同步前建议备份分类结果
   - 已保存：`收藏分类结果.json`

## 🔧 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/albums/list` | GET | 获取专辑列表 |
| `/api/v1/albums/create` | POST | 创建专辑 |
| `/api/v1/albums/add_notes` | POST | 添加笔记到专辑 |

## 📝 示例请求

### 创建专辑
```bash
curl -X POST http://localhost:18060/api/v1/albums/create \
  -H "Content-Type: application/json" \
  -d '{"name": "美食烹饪"}'
```

### 添加笔记到专辑
```bash
curl -X POST http://localhost:18060/api/v1/albums/add_notes \
  -H "Content-Type: application/json" \
  -d '{
    "album_id": "album_xxx",
    "note_ids": ["65640b3300000000320035b2", "671df5e50000000026037e50"]
  }'
```

## ✅ 完成状态

- [x] 收藏列表获取 (145 条)
- [x] AI 自动分类 (7 个分类)
- [x] 专辑管理模块
- [x] 同步工具脚本
- [x] 手动同步清单
- [ ] 自动同步执行 (需要 API 认证)

## 🎯 下一步

1. **测试 API 调用** - 验证专辑创建和笔记移动功能
2. **完善错误处理** - 处理 API 失败和重试逻辑
3. **添加进度显示** - 实时显示同步进度
4. **生成同步报告** - 同步完成后生成详细报告
