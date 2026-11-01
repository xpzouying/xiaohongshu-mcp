# 可视化部署实测

测试环境：Windows 宿主、Docker Desktop 的 Linux/amd64 容器；基础镜像为官方 `v2.5.5`（digest `sha256:5bc690d0c7907f069886ddeb6b37abbe4bdd37bd431975b914c7c27435b1b602`）。

测试实例使用独立空白 data 与只读素材目录。先调用原服务的获取登录二维码接口，让 **MCP 自己启动浏览器**；未扫码登录，再经 noVNC 将该窗口导航到容器内的本地测试页。截图里的输入框是隔离测试页，不是小红书发布页。

## 输入和文件选择

真实 noVNC JavaScript 客户端通过 WebSocket/RFB 发键盘和鼠标事件；没有向远端输入框注入文字。测试页记录实际 input/change 事件：

- Caps Lock 开：`AI`；关：`ai`。
- Shift 大写：`AI`。
- Ctrl+Space 切换英文→拼音→英文，`nihao` + 空格得到“你好”。
- 容器启动后在宿主新增 `newest-cover.txt`；点击系统文件窗口的 **images** 书签，选中文件并点“打开”，网页收到该文件名。
- 容器尝试写入 `/app/images` 被只读挂载拒绝。

![真实输入与文件选择结果](demo-input.png)

![拼音候选窗口](demo-pinyin.png)

![系统文件窗口可见宿主新增文件](demo-files.png)

## 启停与访问

- Compose 配置校验、镜像构建、启动脚本 `bash -n` 通过。
- `/health` 返回 v2.5.5；noVNC 页面及真实 RFB 连接通过。
- MCP、noVNC 发布端口均只绑定宿主回环；原始 VNC 未发布。
- 设置测试 AUTH_TOKEN 后，未带 token 的 MCP API 返回 401。该 token 不保护 noVNC。
- 正常重启后，MCP、显示器、输入法恢复；素材书签没有重复追加。
- 对测试容器执行 SIGKILL 后重新启动通过：陈旧 X 锁被清理。另起第二个显示启动脚本时拒绝占用仍在使用的显示，原服务保持正常。
- 单独结束 x11vnc 时，容器以非零码退出；重新启动后桌面与 API 恢复。

没有改动原 Dockerfile、原 docker-compose.yml 或 Go 服务。没有运行真实账号的登录/发布/互动，也没有实测 Linux 或 macOS 宿主、ARM 镜像、跨机器 SSH 转发；不将上述局部验证写成完整发布流程已通过。

## 重现方式

按 README 在独立测试目录启动此 Compose；通过获取登录二维码工具打开 MCP 浏览器，但不扫码。将该窗口导航到自己托管的本地 HTML（包含普通文本输入框和 `<input type="file">`），从 noVNC 依次输入上述字符并选择挂载文件即可复核。

重启检查使用 `docker compose restart`；强制结束与组件故障测试只对独立测试容器进行。停止后使用原 data 再次启动，应恢复桌面并保留输入法配置和素材书签。
