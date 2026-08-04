# xiaohongshu-mcp 进程管理（本仓自治）

**业务模块（AStockOS 等）不负责启动本服务。** 生命周期只在本项目目录管理。

## 启动 / 停止

```bash
# 在任意 cwd 均可（脚本会 cd 到项目根）
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh start
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh stop
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh status
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh health
~/Documents/Xiaohongshu/xiaohongshu-mcp/bin/xhs-mcp-daemon.sh cleanup-rod   # 用完后清 rod 孤儿 Chrome
```

- 监听：`127.0.0.1:18060`（MCP：`http://localhost:18060/mcp`）
- 启动参数：`-headless=false -port 127.0.0.1:18060`（可用 `XHS_HEADLESS=true` 覆盖）
- **cwd 固定为本仓根**（`cookies.json` 相对路径）
- pid/log：默认 `~/.xiaohongshu-mcp/{pids,logs}/`（不进 git）
- 可选 env：`XHS_READ_ONLY=1`（禁写）、`XHS_RISK_STREAK_LIMIT`（墙信号熔断，默认 3，0=关）、`XHS_FP_SEED`、`XHS_PROXY`、`COOKIES_PATH`

## 可选 LaunchAgent

见 `../deploy/macos/readme.md`（`xhsmcp.plist`）。`WorkingDirectory` 必须设为本仓根。

## 业务侧

只做 health-check / 调 MCP。不通时提示本脚本 `start`，禁止业务仓 `nohup` 本二进制。
