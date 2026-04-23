# xiaohongshu-mcp (扩展版)

> **说明：** 本项目 fork 自 [xpzouying/xiaohongshu-mcp](https://github.com/xpzouying/xiaohongshu-mcp)，因原项目 PR 未被接受，故在此继续维护扩展版本。

MCP for 小红书 / xiaohongshu.com。让你的 AI 助手直接访问小红书数据。

---

## 🔧 主要修改

### 1. Docker 配置优化

**问题：** 原项目默认使用阿里云 Docker 镜像源，非阿里云服务器用户无法使用。

**解决：** 
- 修改默认镜像名称，从公共仓库切换为私有仓库
- 重构 Dockerfile 的 Chrome 安装流程，提升构建稳定性
- **现在支持本地打包，适用于任何云服务器或本地环境**

### 2. 新增 MCP 工具

| 工具名 | 功能 | 参数 |
|--------|------|------|
| `get_my_profile` | 获取当前登录用户的个人信息 | 无 |
| `edit_profile` | 编辑个人资料（昵称和简介） | `nickname`（可选）、`description`（可选） |

---

## 🚀 快速开始

### 本地 Docker 构建（推荐）

```bash
# 1. 克隆项目
git clone https://github.com/Moriarty0909/xiaohongshu-mcp.git
cd xiaohongshu-mcp

# 2. 构建镜像
docker build -t my-xiaohongshu-mcp:latest .

# 3. 启动服务
docker-compose up -d
```

> 💡 **说明：** 本项目未上传 Docker Hub，需先在本地构建镜像。

---

## 📋 可用 MCP 工具

### 用户相关

- `get_my_profile` - 获取当前登录用户的个人信息
  - 返回：昵称、简介、头像、粉丝数、获赞数等
  
- `edit_profile` - 编辑个人资料
  - `nickname`: 新昵称（可选），不超过 20 个字符
  - `description`: 新简介（可选），不超过 100 个字符
  - 至少提供一个参数

### 其他工具

> 其他工具与原项目保持一致，请参考 [原项目文档](https://github.com/xpzouying/xiaohongshu-mcp)

---

## 💡 使用示例

**获取当前用户信息：**
```
帮我查看当前小红书账号的个人信息
```

**编辑个人资料：**
```
帮我把小红书账号的昵称改为「旅行达人」，简介改为「分享旅途中的美好瞬间」
```

---

## ⚠️ 注意事项

1. **Docker 镜像：** 本项目未上传 Docker Hub，需本地构建
2. **适用人群：** 适用于非阿里云服务器用户，或需要自定义 Docker 源的用户
3. **原项目功能：** 如需使用原项目的完整功能，请参考 [xpzouying/xiaohongshu-mcp](https://github.com/xpzouying/xiaohongshu-mcp)

---

## 📝 更新日志

### 2026-04-11
- ✅ 新增 `edit_profile` 功能（编辑个人资料）
- ✅ 优化 Docker 配置，支持本地打包

### 2026-04-08
- ✅ 新增 `get_my_profile` 功能（获取当前用户信息）

---

## 🙏 致谢

感谢原项目作者 [@xpzouying](https://github.com/xpzouying) 的优秀工作。

---

## 📄 许可证

与原项目保持一致
