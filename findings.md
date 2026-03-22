# Findings: xiaohongshu-mcp 项目分析

## Project Analysis
- **Name**: xiaohongshu-mcp
- **Type**: Go MCP Server for Xiaohongshu
- **Language**: Go 1.21+
- **Key Modules**: 
  - handlers_api.go - API 路由处理
  - app_server.go - 应用服务器
  - browser/ - 浏览器自动化
  - cmd/ - 命令行入口
- **Tech Stack**: Go, Rod (浏览器自动化), MCP Protocol, Gin (HTTP 框架)

## 现有功能 (根据 README 和代码)
1. ✅ 登录和检查登录状态
2. ✅ 发布图文/视频内容
3. ✅ 搜索内容
4. ✅ 获取推荐列表
5. ✅ 获取帖子详情
6. ✅ 点赞/取消点赞
7. ✅ 收藏/取消收藏 (`favorite_feed`)
8. ✅ 获取用户主页信息 (`user_profile`)
9. ✅ 评论/回复评论
10. ✅ 获取我的个人主页 (`/api/v1/user/me`)

## 缺失功能
- ❌ 获取自己收藏的笔记列表
- ❌ 收藏夹管理（创建、重命名、删除）
- ❌ 笔记移动到收藏夹
- ❌ 收藏夹自动分类
- ❌ 按标签/提示词筛选收藏笔记

## API 接口分析
根据 routes.go 和 handlers_api.go 分析现有 API 结构：

### API 路由组 `/api/v1`
| 方法 | 路径 | 处理函数 | 功能 |
|------|------|---------|------|
| GET | `/login/status` | checkLoginStatusHandler | 检查登录状态 |
| GET | `/login/qrcode` | getLoginQrcodeHandler | 获取登录二维码 |
| DELETE | `/login/cookies` | deleteCookiesHandler | 删除 cookies |
| POST | `/publish` | publishHandler | 发布图文 |
| POST | `/publish_video` | publishVideoHandler | 发布视频 |
| GET | `/feeds/list` | listFeedsHandler | 获取推荐列表 |
| GET/POST | `/feeds/search` | searchFeedsHandler | 搜索笔记 |
| POST | `/feeds/detail` | getFeedDetailHandler | 获取笔记详情 |
| POST | `/user/profile` | userProfileHandler | 获取用户主页 |
| POST | `/feeds/comment` | postCommentHandler | 发表评论 |
| POST | `/feeds/comment/reply` | replyCommentHandler | 回复评论 |
| GET | `/user/me` | myProfileHandler | 获取我的信息 |

### 服务层结构 (service.go)
- `XiaohongshuService` - 核心服务类
- `withBrowserPage()` - 浏览器页面通用封装
- `newBrowser()` - 创建无头浏览器实例

### 业务层结构 (xiaohongshu/*.go)
- `like_favorite.go` - 点赞/收藏操作（`FavoriteAction`）
- `user_profile.go` - 用户主页信息
- `search.go` - 搜索功能
- `feed_detail.go` - 笔记详情
- `publish.go` - 发布图文
- `publish_video.go` - 发布视频
- `comment_feed.go` - 评论功能

## 收藏夹功能实现思路

### 1. 获取收藏列表
需要创建 `favorite_list.go`：
- 导航到用户主页
- 点击"收藏"标签页
- 滚动加载所有收藏笔记
- 解析笔记列表数据

### 2. 自动分类
- 使用 AI 分析笔记标题、内容、标签
- 生成分类标签（如：美食、旅行、学习、穿搭等）
- 存储分类结果到本地或数据库

### 3. 筛选移动
- 根据标签/关键词过滤收藏笔记
- 调用小红书收藏夹管理 API（需要探索）
- 将笔记移动到指定分类

## Lessons Learned
1. 项目使用 Go 语言，需要保持代码风格一致
2. 浏览器自动化使用 Rod 库
3. API 设计遵循 RESTful 风格
4. 错误处理使用 `myerrors` 包
5. 日志使用 `logrus`
6. 所有操作都需要浏览器页面上下文
