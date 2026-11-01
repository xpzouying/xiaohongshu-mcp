# Docker 可视化浏览器（noVNC，可选）

需要查看 Docker 里的实际浏览器、辅助处理登录页面或手动选取本机素材时，使用此部署。它在官方 MCP 镜像上增加虚拟显示器、窗口管理器、noVNC 和 Fcitx5 拼音，MCP 仍启动自己的内置浏览器。

原来的 Dockerfile 和 docker-compose.yml 保持不变；不需要桌面的用户继续原来的部署。

## 启动

在本目录运行：

```bash
docker compose up -d --build
```

打开 [noVNC](http://127.0.0.1:6080/vnc.html?autoconnect=true&resize=scale)。MCP 地址仍为 `http://127.0.0.1:18060/mcp`。首次打开可能只有空桌面：随后通过原有 MCP 工具（例如获取登录二维码）触发浏览器，窗口才会出现。

这是另一种部署方式。迁移已有实例时，先停止原实例，再将 `XHS_DATA_DIR` 指向原来的 data 目录；不要同时启动两个实例操作同一账号或共享同一运行目录。示例默认使用本目录下独立的 data，**不会自动发现另一套部署的登录数据**。

默认继承官方 `v2.5.5`。可以通过 `MCP_IMAGE` 指定自己已验证的官方版本或本地构建镜像，不在启动时自动升级。自编译主分支时，先在仓库根目录执行 `docker build -t xhs-mcp:local .`，再以该镜像作为 `MCP_IMAGE` 构建本桌面层。

## 中文、大小写和素材

- **Ctrl+Space**：切换英文与拼音。输入 `nihao` 后按空格可得到“你好”。
- **Caps Lock**：切换英文大小写；Shift 也可以临时输入大写。VNC 服务启用了 `-capslock`，避免浏览器端已发大写字符而远端仍输出小写。
- 系统文件窗口里的 **images** 书签指向 `/app/images`。默认映射宿主的 `./images`，只读；宿主新增或更新文件会直接可见，不需复制进容器。
- 要换成已有素材目录，设置 `XHS_MEDIA_DIR`。只挂所需目录，不需要暴露整块硬盘。noVNC 是远程桌面，网页中的“上传”会打开容器文件窗口，不能直接打开宿主的系统文件窗口。

例如在 PowerShell 中：

```powershell
$env:XHS_MEDIA_DIR = 'D:/MyProject/deliveries'
$env:XHS_NOVNC_PORT = '16081'
docker compose up -d --build
```

Linux/macOS 可用 `XHS_MEDIA_DIR=/path/to/deliveries docker compose up -d --build`。已有 data 的路径通过 `XHS_DATA_DIR` 传入；修改挂载路径需要重建容器，先等当前工具调用结束。

## 人工操作的边界

noVNC 显示的是 MCP 启动的同一个浏览器，没有另开一套账号登录。它**不增加常驻浏览器、暂停/恢复自动化或保存浏览器 profile 的接口**：

- 浏览器的创建和关闭仍由原服务控制。普通工具调用完成后窗口通常关闭；二维码窗口只在原登录等待期限内存在。
- 不要在自动化正在输入或点击时同时操作。需要长期手动编辑已发布笔记的场景，还需要独立的生命周期/人工接管设计，单靠本 PR 无法提供。
- Cookie 的保存仍按 MCP 原流程进行；任意手动导航或输入不等于已保存会话。
- 中文输入支持使用开源 Fcitx5 拼音，不依赖微信输入法。

## 访问与回退

Compose 只向宿主 `127.0.0.1` 发布 MCP 和 noVNC 端口，原始 VNC 只在容器回环监听。**noVNC 拥有桌面操作权限且没有额外密码**；`AUTH_TOKEN` 只保护 MCP API，不保护桌面。不要把 6080 直接映射到公网；跨机器访问可用 SSH 本地转发。不要把此容器加入不可信的共享 Docker 网络。

关闭桌面部署执行 `docker compose down`，数据卷对应的本机目录保留。回到原无界面部署时使用同一 data 目录并启动原 Compose；无需改 MCP 代码或迁移 Cookie 格式。

## 验证

本功能只修改部署层，测试使用独立 data 和本地测试页，无真实账号、Cookie 或发布操作。验证内容及真实桌面演示见 [验证记录](verification.md)。

修改启动脚本后至少检查：

1. 构建后 `/health`、`vnc.html` 可访问，MCP 工具启动的窗口出现在 noVNC。
2. 在浏览器输入框实测 Caps Lock 开/关、Shift、拼音与 Ctrl+Space。
3. 从系统文件窗口的 images 书签选取宿主文件，检查宿主新增文件实时可见且容器不能写入。
4. 重启容器后显示器、输入法及 MCP 恢复；结束桌面组件时容器退出，避免只剩 API 可用而桌面已死。
