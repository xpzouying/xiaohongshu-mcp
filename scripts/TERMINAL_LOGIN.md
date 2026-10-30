<!-- Copyright 2026 Marvelooous. SPDX-License-Identifier: Apache-2.0 -->
# Linux 终端扫码与验证码输入

这是本分支新增的本机终端助手，不是上游官方登录工具。它调用已有 MCP 的
`get_login_qrcode`，然后连接同一台 Linux 主机上该浏览器的本地调试端口。
验证码由用户手动输入和提交；助手不绕过平台验证。

## 运行条件

- Linux，能够访问 `/proc`，并与 MCP 使用同一个系统用户运行。
- Node.js 22 或更高版本，推荐 Node.js 24；使用内置 `fetch` 和 `WebSocket`，无需 npm 依赖。
- 已启动本仓库的 MCP 服务，默认地址 `http://127.0.0.1:18060/mcp`。
- 本机安装 `chafa`、`qrencode`、`zbarimg`。Ubuntu/Debian 可安装：

  ```bash
  sudo apt-get install --no-install-recommends chafa qrencode zbar-tools
  ```

仅运行一个 MCP 实例和一个登录助手，避免同时刷新二维码导致登录会话被替换。
该助手不会自动启动 MCP；在另一台电脑运行它，无法发现服务器上的浏览器。

## 开始登录

先按上游说明安装浏览器依赖、构建并启动本分支的 MCP。例如在仓库根目录：

```bash
go build -o bin/xiaohongshu-mcp .
./bin/xiaohongshu-mcp -port 127.0.0.1:18060
```

保持服务运行，在同一主机的另一个交互式终端执行：

```bash
node scripts/xiaohongshu-login.mjs
```

1. 等待生成二维码。二维码由专用工具重新编码，保留四周白边，不使用通用图片缩放。
2. 窗口太小时，缩小终端字体或放大窗口，然后按回车。二维码会保持显示，等待扫码。
3. 用小红书 App 扫码。若收到短信验证码，按回车进入当前网页的操作界面。
4. 根据打印出的标签，使用 `i 编号` 填写对应输入框；输入内容不会在终端回显。
5. 使用 `b 编号` 点击需要的按钮，例如验证码表单的“确认”或“登录”。
6. 按回车刷新网页截图和控件编号；输入 `q` 退出。控件编号以最近一次刷新为准。

例如输入框 1 是验证码、按钮 3 是确认时：

```text
操作 > i 1
输入框 1 的内容（隐藏输入）:
操作 > b 3
```

重复嵌套的同名按钮会尽量合并，但不同位置仍可能有同名按钮。
应结合页面文字和截图选择，不要把所有“登录”按钮依次点击。

成功登录后，MCP 负责保存登录会话并关闭浏览器。助手检测到浏览器关闭时会再次
调用 MCP 检查登录状态；浏览器关闭本身不作为登录成功的凭据。
如果已经登录，获取二维码会直接显示 MCP 返回的状态。

## 地址与鉴权

使用其他本地端口时：

```bash
XHS_MCP_URL=http://127.0.0.1:18061/mcp node scripts/xiaohongshu-login.mjs
```

若服务开启鉴权，通过环境变量 `XHS_MCP_AUTH_TOKEN` 传入 Token；未设置时读取
`AUTH_TOKEN`。例如 Bash 中可隐藏输入，再启动助手：

```bash
read -r -s -p 'MCP Token: ' XHS_MCP_AUTH_TOKEN
printf '\n'
export XHS_MCP_AUTH_TOKEN
node scripts/xiaohongshu-login.mjs
unset XHS_MCP_AUTH_TOKEN
```

不要把真实 Token、Cookie 或验证码写进源码或提交记录。

## 显示与使用限制

- 二维码逐格使用半块字符显示，需要等宽字体；非标准字符比例仍可能导致视觉变形。
- 高清二维码和网页截图保存在权限受限的临时目录，退出时删除。扫码后会显示高清原图路径，
  必要时可以在自己的图像查看器中打开；不要提交或公开这些图片。
- 滑块、拖动验证和跨域 iframe 中的输入框不在当前助手支持范围内，需要使用可交互浏览器完成。
- 登录二维码有服务端有效期。过期时退出后重新运行；不要反复创建多个扫码会话。
- 该分支等待关键 UI/资料数据而不是整页资源完成，但浏览器仍可能启动或渲染较慢。
  只有登录检查和资料读取成功，才能确认当前会话可用。

## 测试

无需真实账号的助手测试：

```bash
node --test scripts/xiaohongshu-login.test.mjs
```

默认 Go 测试不会自动下载或启动浏览器。要验证“图片一直不返回也不阻塞关键元素”，
可显式指定已安装的浏览器执行回归测试：

```bash
XHS_TEST_BROWSER=/absolute/path/to/chrome \
  go test ./xiaohongshu -run TestNavigateToElementWithSlowResources -count=1 -v
```
