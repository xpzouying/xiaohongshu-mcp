# Docker 可视化浏览器（noVNC，可选）

需要查看 Docker 里的实际浏览器、辅助处理登录页面或手动选取本机素材时，使用此部署。它在官方 MCP 镜像上增加虚拟显示器、窗口管理器、noVNC 和 Fcitx5 Rime 拼音，MCP 仍启动自己的内置浏览器。

原来的 Dockerfile 和 docker-compose.yml 保持不变；不需要桌面的用户继续原来的部署。

## 使用官方可选镜像

默认无界面镜像的 `latest`、`vX.Y.Z` 标签保持不变。每次正式发版另外发布 `novnc`、`vX.Y.Z-novnc` 标签，均同步到 Docker Hub 和阿里云；桌面镜像使用同一次发版的无界面镜像 digest 构建，不混用旧版 MCP。

这些标签从包含本功能的首次正式发版开始提供；发版前可使用下方的本地构建方式。

在本目录运行：

```bash
docker compose pull
docker compose up -d
```

国内用户可选择阿里云的同款镜像：

```bash
XHS_NOVNC_IMAGE=crpi-hocnvtkomt7w9v8t.cn-beijing.personal.cr.aliyuncs.com/xpzouying/xiaohongshu-mcp:novnc docker compose up -d
```

需要固定版本时，将 `XHS_NOVNC_IMAGE` 设置为对应仓库的 `vX.Y.Z-novnc` 标签。

打开 [noVNC](http://127.0.0.1:6080/vnc.html?autoconnect=true&resize=scale)。MCP 地址仍为 `http://127.0.0.1:18060/mcp`。首次打开可能只有空桌面：随后通过原有 MCP 工具（例如获取登录二维码）触发浏览器，窗口才会出现。

这是另一种部署方式。迁移已有实例时，先停止原实例，再将 `XHS_DATA_DIR` 指向原来的 data 目录；不要同时启动两个实例操作同一账号或共享同一运行目录。示例默认使用本目录下独立的 data，**不会自动发现另一套部署的登录数据**。

## 本地构建

```bash
docker compose -f compose.yml -f compose.build.yml up -d --build
```

本地构建默认继承官方 `v2.5.5`，生成 `xiaohongshu-mcp-novnc:local`，不会覆盖官方镜像标签。可以通过 `MCP_IMAGE` 指定自己已验证的官方版本或本地构建镜像。自编译主分支时，先在仓库根目录执行 `docker build -t xhs-mcp:local .`，再以该镜像作为 `MCP_IMAGE` 构建本桌面层。

## 中文、大小写和素材

- **Ctrl+Space**：切换英文与拼音。输入 `nihao` 后按空格可得到“你好”。
- **Caps Lock**：切换英文大小写；Shift 也可以临时输入大写。VNC 服务启用了 `-capslock`，避免浏览器端已发大写字符而远端仍输出小写。
- 系统文件窗口里的 **images** 书签指向 `/app/images`。默认映射宿主的 `./images`，只读；宿主新增或更新文件会直接可见，不需复制进容器。
- 要换成已有素材目录，设置 `XHS_MEDIA_DIR`。只挂所需目录，不需要暴露整块硬盘。noVNC 是远程桌面，网页中的“上传”会打开容器文件窗口，不能直接打开宿主的系统文件窗口。

例如在 PowerShell 中：

```powershell
$env:XHS_MEDIA_DIR = 'D:/MyProject/deliveries'
$env:XHS_NOVNC_PORT = '16081'
docker compose up -d
```

Linux/macOS 可用 `XHS_MEDIA_DIR=/path/to/deliveries docker compose up -d`。已有 data 的路径通过 `XHS_DATA_DIR` 传入；修改挂载路径需要重建容器，先等当前工具调用结束。

## 人工操作的边界

noVNC 显示的是 MCP 启动的同一个浏览器，没有另开一套账号登录。它**不增加常驻浏览器、暂停/恢复自动化或保存浏览器 profile 的接口**：

- 浏览器的创建和关闭仍由原服务控制。普通工具调用完成后窗口通常关闭；二维码窗口只在原登录等待期限内存在。
- 不要在自动化正在输入或点击时同时操作。需要长期手动编辑已发布笔记的场景，还需要独立的生命周期/人工接管设计，单靠本 PR 无法提供。
- Cookie 的保存仍按 MCP 原流程进行；任意手动导航或输入不等于已保存会话。
- 中文输入使用开源 Fcitx5 Rime 简体拼音，词库在镜像内离线提供。

## 访问与回退

Compose 只向宿主 `127.0.0.1` 发布 MCP 和 noVNC 端口，原始 VNC 只在容器回环监听。**noVNC 拥有桌面操作权限且没有额外密码**；`AUTH_TOKEN` 只保护 MCP API，不保护桌面。不要把 6080 直接映射到公网；跨机器访问可用 SSH 本地转发。不要把此容器加入不可信的共享 Docker 网络。

桌面层只增加 Ubuntu 软件包仓库中的组件及本目录的启动脚本和输入法配置，不下载额外安装脚本，不包含账号、Cookie 或密钥。使用 Rime 避免引入 Qt WebEngine，关闭推荐包安装并清理 apt 索引。没有安装云拼音或 SSH 服务。

Compose 移除额外 Linux capabilities 并设置 `no-new-privileges`，素材目录只读。容器沿用基础镜像的运行用户；这些限制不能替代桌面鉴权或公网访问控制。

WebSocket 只接受本机页面来源，拒绝其他网站及没有 Origin 的桌面连接。Compose 会根据 `XHS_NOVNC_PORT` 配置来源；手动 `docker run` 映射其他端口时同时设置 `NOVNC_PORT`。SSH 转发使用不同本地端口时，可通过 `NOVNC_ALLOWED_ORIGINS` 指定浏览器实际访问的本机来源（空格分隔）；这不会增加密码认证。

关闭桌面部署执行 `docker compose down`，数据卷对应的本机目录保留。回到原无界面部署时使用同一 data 目录并启动原 Compose；无需改 MCP 代码或迁移 Cookie 格式。

## 验证

本功能只修改部署层，测试使用独立 data 和本地测试页，无真实账号、Cookie 或发布操作。验证内容及真实桌面演示见 [验证记录](verification.md)。

修改启动脚本后至少检查：

1. 构建后 `/health`、`vnc.html` 可访问，MCP 工具启动的窗口出现在 noVNC。
2. 在浏览器输入框实测 Caps Lock 开/关、Shift、拼音与 Ctrl+Space。
3. 从系统文件窗口的 images 书签选取宿主文件，检查宿主新增文件实时可见且容器不能写入。
4. 重启容器后显示器、输入法及 MCP 恢复；结束桌面组件时容器退出，避免只剩 API 可用而桌面已死。

CI 会从同一份代码构建无界面与 GUI 两款镜像，并执行不使用账号的容器检查。也可以在本地运行：

```bash
python3 docker/novnc/smoke_test.py xiaohongshu-mcp-novnc:local
```

该命令在仓库根目录执行，要求本机有 Docker 和 Python 3；测试使用独立容器、临时素材、回环随机端口，并在结束时清理。

CI 另用校验过 SHA256 的固定版本 Grype 扫描两款镜像，保存完整报告，并阻止 GUI 相对基线新增或严重度升高的 High/Critical 告警。基础镜像原有告警单独跟进；通过此检查不代表整款镜像没有已知漏洞。
