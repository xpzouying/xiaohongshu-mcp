# GUI 部署（noVNC，可选）

在网页中查看和操作 MCP 启动的浏览器，支持中文拼音和素材选择。日常自动化可继续使用更小的 [无界面镜像](../README.md)。

## 启动

在本目录执行：

```bash
docker compose pull
docker compose up -d
```

- 浏览器桌面：[打开 noVNC](http://127.0.0.1:6080/vnc.html?autoconnect=true&resize=scale)
- MCP 地址：`http://127.0.0.1:18060/mcp`

首次可能只有空桌面，通过 MCP 工具触发浏览器后才会出现窗口。普通调用完成后窗口通常关闭，不提供长时间手动接管；避免与自动化同时输入或点击。

## 中文输入和素材

- **Ctrl+Space** 切换英文与简体拼音，输入 `nihao` 后按空格得到“你好”。
- 系统文件窗口的 **images** 书签对应宿主 `./images`，容器内路径为 `/app/images`，只读；宿主新增文件会直接可见。

## 常用配置

| 环境变量 | 用途 | 默认值 |
| --- | --- | --- |
| `XHS_NOVNC_IMAGE` | GUI 镜像，可固定为 `vX.Y.Z-novnc` | `xpzouying/xiaohongshu-mcp:novnc` |
| `XHS_DATA_DIR` | 登录状态和运行数据目录 | `./data` |
| `XHS_MEDIA_DIR` | 宿主素材目录 | `./images` |
| `XHS_NOVNC_PORT` | 桌面端口 | `6080` |
| `XHS_MCP_PORT` | MCP 端口 | `18060` |

目录默认相对于本目录。迁移已有实例时，先停止旧实例，再把 `XHS_DATA_DIR` 指向原 data 的绝对路径，避免同时运行两套实例。

例如指定素材目录：

```bash
# Linux / macOS
XHS_MEDIA_DIR=/path/to/images docker compose up -d
```

```powershell
# Windows PowerShell
$env:XHS_MEDIA_DIR = 'D:/images'
docker compose up -d
```

使用阿里云时，将 `XHS_NOVNC_IMAGE` 设为 `crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp:novnc`。

## 安全访问

桌面没有独立密码，`AUTH_TOKEN` 只保护 MCP API。默认端口仅绑定本机，**不要直接暴露公网或加入不可信的 Docker 网络**。

跨机器使用 SSH 转发，并保持本地端口与 `XHS_NOVNC_PORT` 一致。例如默认端口：

```bash
ssh -L 6080:127.0.0.1:6080 user@server
```

然后打开本机的 noVNC 地址。需要使用不同本地端口时，在服务端启动容器前，将 `NOVNC_ALLOWED_ORIGINS` 设为浏览器实际访问的本机来源，例如 `http://127.0.0.1:16081`。

## 本地构建（可选）

```bash
docker compose -f compose.yml -f compose.build.yml up -d --build
```

生成 `xiaohongshu-mcp-novnc:local`；如需指定基础 MCP 镜像，设置 `MCP_IMAGE`。

维护者的测试方法和覆盖范围见 [验证记录](verification.md)。
