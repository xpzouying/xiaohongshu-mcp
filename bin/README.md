# xiaohongshu-mcp 进程管理（本仓自治）

**业务模块（AStockOS 等）不负责启动本服务。** 生命周期只在本项目目录管理。

## 浏览器底层：Camoufox

本服务的浏览器是 [Camoufox](https://github.com/daijro/camoufox)（Firefox 反检测构建），
经 playwright-go（Juggler 协议）驱动。**运行时不下载任何浏览器或驱动组件**；
首次使用先跑一次性的安装器，把固定版本、经 SHA-256 校验的二进制落到本仓：

```bash
# 安装 Camoufox（固定版本，下载后强制 SHA-256 校验）+ playwright 驱动（node + playwright-core）
go run ./cmd/camoufox-setup
```

落点（均已 gitignore）：
- Camoufox：`bin/camoufox/Camoufox.app/...`（版本由 `browser.CamoufoxVersion` 固定）
- 驱动：`.playwright-driver/package`（playwright-core，与 go.mod 的 playwright-go 对应）

也可不跑安装器，改用环境变量指向已有组件：
- `XHS_CAMOUFOX_BIN`：Camoufox 可执行文件 / `.app` 包 / 其上层目录
- `PLAYWRIGHT_DRIVER_PATH`：含 `package/cli.js` 的驱动目录
- `PLAYWRIGHT_NODEJS_PATH`：node 可执行文件（缺省取驱动目录自带或 PATH 中的 node）

## 启动 / 停止

```bash
# 在任意 cwd 均可（脚本会 cd 到项目根）
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh start
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh stop
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh status
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh health
```

- 监听：`127.0.0.1:18060`（MCP：`http://localhost:18060/mcp`）
- 启动参数：`-headless=false -port 127.0.0.1:18060`
- **cwd 固定为本仓根**（`cookies.json` 相对路径）
- pid/log：默认 `~/.xiaohongshu-mcp/{pids,logs}/`（不进 git）
- 可选 env：
  - `XHS_CAMOUFOX_BIN`、`PLAYWRIGHT_DRIVER_PATH`、`PLAYWRIGHT_NODEJS_PATH`（见上）
  - `XHS_FP_SEED`（固定指纹 seed，缺省取 cookies.json 里的 seed）、`XHS_PROXY`、`COOKIES_PATH`
  - `XHS_RISK_STREAK_LIMIT`（墙信号熔断，默认 3，0=关）
  - 找不到 Camoufox 或驱动时服务直接退出并提示安装方式（fail closed，绝不回退自动下载）

## 登录（扫码）

```bash
go run ./cmd/login   # 有头弹出 Camoufox，扫码后把 cookie 写回 cookies.json
```

## 业务侧

只做 health-check / 调 MCP。不通时提示本脚本 `start`，禁止业务仓 `nohup` 本二进制。
