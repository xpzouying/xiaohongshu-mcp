# HTTP REST API 使用文档

## 新增功能

本次更新添加了互动操作的 HTTP REST API 端点，无需通过 MCP 协议即可调用。

---

## 📌 收藏笔记 API

### 端点
```
POST /api/v1/feeds/favorite
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | ✅ | 笔记 ID |
| `xsec_token` | string | ✅ | 安全令牌 |
| `unfavorite` | boolean | ❌ | `true`=取消收藏，`false`=收藏（默认） |

### 请求示例

```bash
# 收藏笔记
curl -X POST http://localhost:18060/api/v1/feeds/favorite \
  -H "Content-Type: application/json" \
  -d '{
    "feed_id": "6970638b000000002203ba9e",
    "xsec_token": "ABCI6jKtSJIe5Y3ARKIzTrmeoc2bFHSmhYZwkhUGv3rhU="
  }'

# 取消收藏
curl -X POST http://localhost:18060/api/v1/feeds/favorite \
  -H "Content-Type: application/json" \
  -d '{
    "feed_id": "6970638b000000002203ba9e",
    "xsec_token": "ABCI6jKtSJIe5Y3ARKIzTrmeoc2bFHSmhYZwkhUGv3rhU=",
    "unfavorite": true
  }'
```

### 响应示例

**成功：**
```json
{
  "success": true,
  "data": {
    "feed_id": "6970638b000000002203ba9e",
    "success": true,
    "message": "收藏成功",
    "unfavorite": false
  },
  "message": "收藏成功"
}
```

**失败：**
```json
{
  "error": "收藏失败：context deadline exceeded",
  "code": "FAVORITE_FAILED",
  "details": "context deadline exceeded"
}
```

---

## 👍 点赞笔记 API

### 端点
```
POST /api/v1/feeds/like
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `feed_id` | string | ✅ | 笔记 ID |
| `xsec_token` | string | ✅ | 安全令牌 |
| `unlike` | boolean | ❌ | `true`=取消点赞，`false`=点赞（默认） |

### 请求示例

```bash
# 点赞笔记
curl -X POST http://localhost:18060/api/v1/feeds/like \
  -H "Content-Type: application/json" \
  -d '{
    "feed_id": "6970638b000000002203ba9e",
    "xsec_token": "ABCI6jKtSJIe5Y3ARKIzTrmeoc2bFHSmhYZwkhUGv3rhU="
  }'

# 取消点赞
curl -X POST http://localhost:18060/api/v1/feeds/like \
  -H "Content-Type: application/json" \
  -d '{
    "feed_id": "6970638b000000002203ba9e",
    "xsec_token": "ABCI6jKtSJIe5Y3ARKIzTrmeoc2bFHSmhYZwkhUGv3rhU=",
    "unlike": true
  }'
```

---

## ⚠️ 注意事项

### 1. 超时设置
互动操作需要打开浏览器并模拟点击，耗时较长（通常 30-60 秒）。建议：
- 设置 HTTP 客户端超时时间 ≥ 120 秒
- 使用异步调用或后台任务

### 2. 登录状态
- 必须先登录（通过扫码或 cookies）
- 登录状态保存在 `~/.config/xiaohongshu-mcp/cookies.json`

### 3. 智能检测
- 如果笔记已收藏，再次收藏会跳过（返回"收藏成功或已收藏"）
- 如果笔记未收藏，取消收藏会跳过（返回"取消收藏成功或未收藏"）

---

## 📋 完整 API 列表

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/api/v1/login/status` | GET | 检查登录状态 |
| `/api/v1/login/qrcode` | GET | 获取登录二维码 |
| `/api/v1/login/cookies` | DELETE | 删除 cookies |
| `/api/v1/publish` | POST | 发布图文笔记 |
| `/api/v1/publish_video` | POST | 发布视频笔记 |
| `/api/v1/feeds/list` | GET | 获取推荐列表 |
| `/api/v1/feeds/search` | GET/POST | 搜索笔记 |
| `/api/v1/feeds/detail` | POST | 获取笔记详情 |
| `/api/v1/feeds/favorite` | POST | **收藏/取消收藏** ✨ |
| `/api/v1/feeds/like` | POST | **点赞/取消点赞** ✨ |
| `/api/v1/feeds/comment` | POST | 发表评论 |
| `/api/v1/feeds/comment/reply` | POST | 回复评论 |
| `/api/v1/user/profile` | POST | 获取用户主页 |
| `/api/v1/user/me` | GET | 获取当前用户信息 |

---

## 🔧 批量收藏示例脚本

```bash
#!/bin/bash
# 批量收藏笔记

FEEDS=(
  "6970638b000000002203ba9e:ABCI6jKtSJIe5Y3ARKIzTrmeoc2bFHSmhYZwkhUGv3rhU="
  "68da6c79000000001300949f:ABB3DuPfV0yU_3XAcg1-mEmQXmG90aDzat-6i7nTv_qTg="
  "68eb28a60000000007021107:ABcbH2KdSk-Am-Q4-lfsENOe9ehBp892VQVaNZJ-0QQXM="
)

API_URL="http://localhost:18060/api/v1/feeds/favorite"

for feed in "${FEEDS[@]}"; do
  IFS=':' read -r feed_id token <<< "$feed"
  
  echo "收藏：$feed_id"
  curl -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "{\"feed_id\": \"$feed_id\", \"xsec_token\": \"$token\"}" \
    --max-time 120 \
    -w "\nHTTP 状态码：%{http_code}\n"
  
  sleep 2  # 避免请求过快
done
```

---

## 📝 更新日志

**2026-03-08**
- ✅ 新增 `/api/v1/feeds/favorite` - 收藏/取消收藏 API
- ✅ 新增 `/api/v1/feeds/like` - 点赞/取消点赞 API
- ✅ 智能检测已收藏/已点赞状态
- ✅ 统一的错误处理和响应格式

---

**作者：** Jari (for Lord)  
**GitHub:** https://github.com/lkisme/xiaohongshu-mcp
