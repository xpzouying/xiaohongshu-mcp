# GUI 镜像验证

2026-11-01，在本地 macOS / OrbStack 和 GitHub Actions Ubuntu 上验证 Linux/amd64 容器。自动化使用空白数据与本地测试页。

## 覆盖范围

| 项目 | 检查内容 |
| --- | --- |
| 默认镜像 | 无 GUI，启动命令保持不变 |
| 桌面输入 | 英文、Shift、Caps Lock、Rime 简体拼音与 Ctrl+Space 切换 |
| 素材与数据 | 宿主新增文件的系统窗口选择、素材只读、其他 UID 的 data 和旧输入法配置迁移 |
| 访问限制 | MCP 鉴权、回环端口、拒绝跨站及缺失 Origin 的桌面连接 |
| 进程恢复 | 组件提前退出、MCP 退出码、正常重启及 SIGKILL 后显示锁恢复 |
| 安全扫描 | GUI 相对基线新增或严重度升级的 High/Critical 告警为 0 |

安全门禁不代表整款镜像没有漏洞；已有浏览器、Go 及依赖库告警后续单独处理。真实账号的登录、发布和互动不在上述自动化验证范围内。

## 运行检查

构建本地 GUI 镜像后，在项目根目录执行：

```bash
python3 docker/novnc/smoke_test.py xiaohongshu-mcp-novnc:local
```

需要 Docker 和 Python 3。测试使用隔离容器、临时数据和随机回环端口，结束后清理。

[Docker CI](../../../.github/workflows/docker-test.yml) 自动构建两款镜像、执行相同检查，并用固定版本 Grype 比较漏洞告警；完整报告保存在 CI artifact 中。参见 [通过的 CI 记录](https://github.com/xpzouying/xiaohongshu-mcp/actions/runs/41320463382)。

## 原始演示

以下为原始投稿截图，输入法对应切换 Rime 之前的版本，仅供操作效果参考：

- 输入与文件选择：[输入结果](demo-input.png)、[拼音候选](demo-pinyin.png)、[文件窗口](demo-files.png)。
- 小红书公开浏览：[首页](demo-xhs-logged-in.png)、[中文搜索联想](demo-xhs-search-input.png)、[搜索结果](demo-xhs-search-results.png)。
