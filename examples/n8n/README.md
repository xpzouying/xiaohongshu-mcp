# N8N 接入 xiaohongshu-mcp 完整指南

## 📋 概述

本文档详细介绍了如何部署汉化版 n8n 工作流平台，并集成 xiaohongshu-mcp 服务，实现自动化小红书内容发布功能。

## 🚀 环境准备

### 前置要求
- Docker 和 Docker Compose 已安装
- xiaohongshu-mcp 服务已正常启动
- 有效的 DeepSeek API 密钥

## 📦 n8n 部署指南

### 1. 下载汉化包

前往 [n8n 汉化包项目](https://github.com/other-blowsnow/n8n-i18n-chinese/releases) 下载最新版本的汉化文件。

**操作步骤：**
1. 下载最新的汉化包压缩文件
2. 解压下载的文件
3. 确保解压后包含 `editor-ui/dist` 文件夹

### 2. Docker Compose 部署（推荐）

创建 `docker-compose.yml` 文件，内容如下：

```yaml
version: '3'

services:
  n8n:
    image: n8nio/n8n
    container_name: n8n
    restart: unless-stopped
    ports:
      - "5678:5678"
    volumes:
      # 运行数据挂载 - 确保工作流数据持久化
      - ./n8n_data:/home/node/.n8n
      # 汉化包挂载 - 替换为你的汉化包路径
      - ./editor-ui/dist:/usr/local/lib/node_modules/n8n/node_modules/n8n-editor-ui/dist
    environment:
      - N8N_HOST=localhost
      - N8N_PORT=5678
      - N8N_PROTOCOL=http
      # 可选：设置基本认证（增强安全性）
      # - N8N_BASIC_AUTH_ACTIVE=true
      # - N8N_BASIC_AUTH_USER=myuser
      # - N8N_BASIC_AUTH_PASSWORD=mypassword
      # 时区设置（亚洲/上海）
      - GENERIC_TIMEZONE=Asia/Shanghai
      # 调试时禁用安全Cookie（方便本地访问）
      - N8N_SECURE_COOKIE=false
      # 设置默认语言为简体中文
      - N8N_DEFAULT_LOCALE=zh-CN
    networks:
      - n8n-network

networks:
  n8n-network:
    driver: bridge
```

**启动服务：**
```bash
docker-compose up -d
```

### 3. Docker 直接部署（备选方案）

创建启动脚本或直接运行命令：

```bash
docker run -it --name n8nChinese \
  -p 5678:5678 \
  -v "/path/to/editor-ui-dist:/usr/local/lib/node_modules/n8n/node_modules/n8n-editor-ui/dist" \
  -v "${HOME}/.n8n:/home/node/.n8n" \
  -e N8N_DEFAULT_LOCALE=zh-CN \
  -e N8N_SECURE_COOKIE=false \
  n8nio/n8n
```

### 4. 访问和初始化

1. 打开浏览器访问：http://localhost:5678
2. 首次访问需要输入邮箱地址进行注册
3. n8n 会向该邮箱发送激活码
4. 按提示输入激活码完成初始化

![初始化界面](./images/image-20250915225901709.png)
![激活界面](./images/image-20250915225950626.png)


## ⚠️ 重要注意事项

- **数据持久化**：务必挂载本地目录保存工作流数据，避免容器重启后数据丢失
- **端口冲突**：如端口 5678 被占用，可修改 `-p` 参数映射其他端口
- **汉化配置**：`N8N_DEFAULT_LOCALE=zh-CN` 环境变量强制设置为简体中文界面
- **安全警告**：生产环境建议启用基本认证和安全Cookie设置

## 🔌 接入 xiaohongshu-mcp 服务

### 前提条件
确保 xiaohongshu-mcp 服务已正常启动并运行

### 配置步骤

#### 步骤 1：创建工作流

在 n8n 控制台中创建新的工作流：

![创建工作流](./images/image-20250915225530994.png)

#### 步骤 2：导入工作流配置

导入本目录中的配置文件：
- 文件名称：`自动发布笔记到小红书.json`
- 操作：点击"导入工作流"选择该文件

![导入工作流](./images/image-20250915230216557.png)

#### 步骤 3：配置大模型节点

1. 选择 AI 大模型节点（支持 DeepSeek、OpenAI 等）
2. 配置大模型连接凭证
3. 以 DeepSeek 为例，需要申请 API 密钥

**DeepSeek API 密钥申请：**
- 访问：[DeepSeek 平台](https://platform.deepseek.com/api_keys)
- 注册账号并获取 API 密钥

![选择大模型](./images/image-20250915230403977.png)
![配置凭证](./images/image-20250915230528047.png)
![完成配置](./images/image-20250915230614246.png)

#### 步骤 4：配置 MCP 服务

1. **双击 MCP 节点进行配置**

![配置MCP节点](./images/image-20250915231537715.png)

2. **修改连接设置**
   - 将 IP 地址修改为你实际的 xiaohongshu-mcp 服务 IP
   - 默认导入所有可用的工具函数
   

   ![修改IP配置](./images/image-20250915231736534.png)


3. **测试连接**
   - 点击"执行步骤"测试连接
   - 选择一个接口进行功能测试
   - 返回成功表示接入正常
   

   ![测试连接](./images/image-20250915232135744.png)
   ![测试成功](./images/image-20250915232246623.png)


## 🎯 开始使用

### 执行工作流

1. 点击"开始执行该步骤"
2. 在聊天框中输入提示词
3. 系统会自动处理并发布内容


![开始执行](./images/image-20250915232457764.png)

### 示例提示词

```
给我发布一篇关于重庆旅游的小红书爆款笔记，配图找"重庆打卡"点赞最高的一张
```

### 效果展示

![测试过程](./images/测试图.png)
![测试结果](./images/测试效果图.jpg)


## 🛠️ 故障排除

### 常见问题

1. **连接失败**：检查 xiaohongshu-mcp 服务是否正常运行
2. **API 密钥错误**：确认 DeepSeek API 密钥有效且未过期
3. **汉化不生效**：检查汉化包路径是否正确挂载
4. **端口冲突**：修改 docker-compose.yml 中的端口映射

### 获取帮助

- 查看 n8n 官方文档：https://docs.n8n.io
- 参考 xiaohongshu-mcp 项目文档
- 检查日志文件排查具体错误

## 📁 项目文件说明

- `docker-compose.yml` - Docker Compose 部署配置文件
- `自动发布笔记到小红书.json` - n8n 工作流配置文件
- `images/` - 说明文档相关截图
- `editor-ui/dist/` - 汉化包文件（需自行下载）

## 🎉 完成部署

通过以上步骤，您已成功部署汉化版 n8n 并集成 xiaohongshu-mcp 服务，可以开始自动化小红书内容发布工作了！