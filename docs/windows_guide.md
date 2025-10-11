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

# xiaohongshu-mcp Windows11快速搭建

## 1.  下载最新构建版本

[github.com](https://github.com/xpzouying/xiaohongshu-mcp/releases)

如果当前系统为Windows 则选择 xiaohongshu-mcp-windows-amd64.zip 下载

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_597379_Dw_WBLdYI-KsFlXm_1760067122?w=1137&h=633&type=image/png)

下载完解压文件

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_806026_wozodlNLyXADgJzQ_1760067150?w=1097&h=437&type=image/png)

在当前文件夹中右键打开终端

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_24479_igFOK7Lf332tlvkM_1760067218?w=1090&h=622&type=image/png)

先运行登录命令程序

```
./xiaohongshu-login-windows-amd64.exe
```

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_557435_MEWWz-JeHubKmkhc_1760067518?w=1709&h=810&type=image/png)

等待下载完

## 2.  解决Windows 11 报病毒问题

在运行之前的程序后会报病毒，如下图

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_79147_lDOh7CnkzJEWiROM_1760067634?w=1761&h=518&type=image/png)

这时候我们需要打开Windows 安全中心（Windows 11 版本演示）

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_436678__HrwxQPD57zZvW5h_1760067781?w=1424&h=932&type=image/png)

点击进入管理设置后，查看最下方的排除项

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_936924_6OPZpwjyICV7NlGc_1760067974?w=1166&h=916&type=image/png)

把之前的错误程序的路径添加进去，如下图

要改成你当前报错的实际路径

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_871687_NBwGzTWJ1RHTQgBQ_1760068159?w=1901&h=439&type=image/png)

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_710523_eExonqwWf2gSc5RD_1760068191?w=1838&h=658&type=image/png)

总结解决路径办法

解决步骤：

1. 打开 Windows 安全中心（Windows Security）。

2. 点击 病毒和威胁防护（Virus & threat protection）。

3. 在“病毒和威胁防护设置”下，点击 管理设置（Manage settings）。

4. 向下滚动，找到并点击 添加或删除排除项（Add or remove exclusions）。

5. 点击 添加排除项（Add an exclusion）。

6. 选择 文件夹（Folder）。

7. 导航到以下路径并选择该文件夹：

```
C:\Users\你的用户(当前电脑)\AppData\Local\Temp\leakless-amd64-adb80298fa6a3af7ced8b1c9b5f18007
```

8.  . 确认添加排除项。

## 3.  启动程序

```
./xiaohongshu-login-windows-amd64.exe
```

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_986235_Vn-u3F7LZXOsYE6c_1760078263?w=1118&h=346&type=image/png)

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_215347_jIpS7bT7J6nQPIDs_1760078324?w=901&h=830&type=image/png)

登录小红书

启动MCP服务

```
./xiaohongshu-mcp-windows-amd64.exe
```

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_66988_0r6LHv0FuL9Aidlv_1760094345?w=970&h=291&type=image/png)

## 4.  MCP 验证

```
npx @modelcontextprotocol/inspector
```

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_861647_Lo0xw1oXyLKD5A2Y_1760165693?w=1074&h=452&type=image/png)

![](https://wdcdn.qpic.cn/MTY4ODg1NTIyMTY1ODI2NQ_260079_5FFeEfMTVXaLGXoz_1760165797?w=1905&h=937&type=image/png)