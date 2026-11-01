# GUI 部署（noVNC）

在网页中查看和辅助操作小红书浏览器。不需要桌面时，使用 [无界面部署](../README.md)。

## 启动

先安装并启动 Docker，下载本项目。

**1. 配置阿里云镜像（可选）**

默认使用 Docker Hub，可跳过这一步。使用阿里云时，在 `docker/novnc/` 中新建文本文件 `.env`，与 `compose.yml` 放在一起，写入：

```dotenv
XHS_NOVNC_IMAGE=crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp:novnc
```

文件名不要带 `.txt`；已有 `.env` 时，只修改这一项。

**2. 启动服务**

在项目根目录打开终端，执行：

```bash
cd docker/novnc
docker compose pull
docker compose up -d
```

**3. 连接使用**

- 桌面：[点击打开](http://127.0.0.1:6080/vnc.html?autoconnect=true&resize=scale)。
- MCP：`http://127.0.0.1:18060/mcp`，按 [客户端配置](../../README.md#2-mcp-客户端接入) 接入 AI。

桌面空白时，让 AI“获取小红书登录二维码”即可出现浏览器。普通工具调用结束后窗口通常关闭，人工操作时避免与自动化同时输入或点击。

## 常用操作

- 中文输入：**Ctrl+Space** 切换拼音，输入 `nihao` 后按空格得到“你好”。
- 发布素材：文件放入 `docker/novnc/images/`，告诉 AI 容器内路径，例如 `/app/images/cover.jpg`。桌面文件窗口的 **images** 书签也可找到，素材挂载为只读。
- 更新：在 `docker/novnc/` 执行 `docker compose pull`，再执行 `docker compose up -d`。

**桌面没有密码，仅限本机或 SSH 转发访问，不要直接暴露公网。** `AUTH_TOKEN` 只保护 MCP API。

<details>
<summary>其他配置、数据迁移与远程访问</summary>

以下配置可写入同一份 `.env`，每行一项；保存后执行 `docker compose up -d`。

| 配置项 | 用途 | 默认值 |
| --- | --- | --- |
| `XHS_NOVNC_IMAGE` | 镜像地址；固定版本可用 `vX.Y.Z-novnc` 标签 | `xpzouying/xiaohongshu-mcp:novnc` |
| `XHS_DATA_DIR` | 登录数据目录 | `./data` |
| `XHS_MEDIA_DIR` | 素材目录，例如 `D:/images` 或 `/home/user/images` | `./images` |
| `XHS_NOVNC_PORT` | 桌面端口 | `6080` |
| `XHS_MCP_PORT` | MCP 端口 | `18060` |

默认目录位于 `docker/novnc/` 下。从已有部署切换时，先停止旧实例，将 `XHS_DATA_DIR` 设为原 `data` 的完整路径，保留登录状态；不要同时运行两套实例操作同一账号。修改端口后，访问地址也要相应修改。

远程访问示例：`ssh -L 6080:127.0.0.1:6080 user@server`，然后打开本机桌面地址。转发端口应与 `XHS_NOVNC_PORT` 一致；不一致时，在服务端配置 `NOVNC_ALLOWED_ORIGINS` 为浏览器实际使用的本机来源，例如 `http://127.0.0.1:16081`。不要将容器加入不可信的 Docker 网络。

</details>

<details>
<summary>维护者：本地构建与验证</summary>

在 `docker/novnc/` 执行：

```bash
docker compose -f compose.yml -f compose.build.yml up -d --build
```

生成 `xiaohongshu-mcp-novnc:local`，可通过 `MCP_IMAGE` 指定基础镜像。测试方法见 [验证记录](docs/verification.md)。

</details>
