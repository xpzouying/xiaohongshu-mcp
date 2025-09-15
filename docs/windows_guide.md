# Windows 安装指南（避免环境变量问题）

在 Windows 部署过程，如果遇到问题，那么可以先参考本手册。

可以参考这里 https://github.com/xpzouying/xiaohongshu-mcp/issues/56

由于 xiaohongshu-mcp 采用的是 Go，NPX 则依赖 Node.JS。为了*避免后续遇到的环境变量等问题*，建议使用 Winget 来安装 Go 和 Node.JS，因为使用 Winget 安装后，Windows 会自动配置好对应的环境变量。

## 打开命令行
<img width="981" height="851" alt="打开命令行" src="https://github.com/user-attachments/assets/1170e4b4-5a47-41ae-9beb-6ca9bd896ede" />

1. Windows 搜索框中输入 CMD
2. 选择以管理员身份运行

## 安装 Go 
在*命令行*中使用以下命令安装 Go (截图如下）
<img width="762" height="164" alt="安装 Go" src="https://github.com/user-attachments/assets/621752cf-a757-41e6-9b14-45ff559537f3" />

```bash
 winget install GoLang.Go
```

## 安装 Node.JS
继续在*命令行*中使用以下命令安装 Node.JS (截图如下）
<img width="665" height="178" alt="安装 Node.JS" src="https://github.com/user-attachments/assets/e09f33cb-f6dc-46f1-824a-ed3c7929658f" />


```bash
 winget install OpenJS.NodeJS.LTS
```

祝大家使用 xiaohongshu-mcp 服务愉快哦~
