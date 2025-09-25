## 后台运行小红书 MCP 的解决方案 - Mac 端

通过此方法你可以：通过系统进程管理小红书 MCP

### 快速开始

#### 1. 安装配置

1. 打开当前目录下 xhsmcp.plist
   1. 必须：替换 {二进制路径} 为你的小红书 MCP 二进制路径
   2. 必须：替换 {工作路径} 为你的小红书 MCP 工作路径，必须在有 cookies.json 文件的目录才能正常工作
   3. 可选：修改默认日志路径 StandardOutPath
   4. 可选：修改默认错误日志路径 StandardErrorPath
   5. 可选：修改错误退出的行为是否重启 KeepAlive
   6. 可选：修改是否开机自动重启 RunAtLoad
2. 安装配置
   1. ln -s {你编辑后的 plist} ~/Library/LaunchAgents/xhsmcp.plist
   2. launchctl load ~/Library/LaunchAgents/xhsmcp.plist

至此就完成了配置安装

#### 2. 使用配置

启动小红书 MCP 服务

```bash
launchctl start xhsmcp
```

关闭小红书 MCP 服务

```bash
launchctl stop xhsmcp
```

查看服务状态，输出有进程 ID 则为运行中，也可以通过 curl 检查服务运行状态

```bash
launchctl list | grep xhsmcp
```

### Shell 脚本管理 （进阶用法）

如果你使用 fish shell，可以安装该目录下的 xhsmcp.fish，实现类似这样的效果：

``` bash
~/home
> launchctl list | grep 

-	0	xhsmcp

~/home
> xhsmcp_status

✗ xhsmcp 未运行
是否启动服务? (yes/其他): yes
✓ 服务启动成功 (PID: 76061)

~/home
> launchctl list | grep 
76061	0	xhsmcp
```
