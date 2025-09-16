# Windows 安装指南

在 Windows 部署过程，如果遇到问题，那么可以先参考本手册。

## 开发环境安装指南

本项目需要使用 **Go** 和 **npx**，因此需要安装 **Go** 和 **Node.js**。  
请不要直接通过官网下载的安装包进行安装，否则可能会因为环境变量配置问题，导致后续运行失败。  

推荐使用 **Windows 包管理工具 `winget`** 来完成安装，操作简单，且可自动配置环境变量。  

---

### 使用 Windows 命令行

如果你不熟悉 Windows 命令行（CMD），可以参考以下步骤：

打开 CMD：
  - 同时按下 `Win + R`，在弹出的“运行”窗口输入 `cmd`，然后按回车；  
  - 或者在开始菜单搜索 `命令提示符` 并点击打开。  


   ```cmd
   winget -v
   ```

### 确保已安装 winget

- 操作系统：Windows 10 或 Windows 11  
- 已安装 **winget**（Windows 包管理工具）  
  - 检查方式：在命令行运行：
  - 
<img width="446" height="191" alt="检查 WinGet 版本" src="https://github.com/user-attachments/assets/84389b75-5998-411a-ba26-e86885750e8c" />


    ```bash
    winget -v
    ```

  - 如果能输出版本号，说明已安装  
  - 若提示找不到命令，请先更新系统或安装 [App Installer](https://apps.microsoft.com/store/detail/9NBLGGH4NNS1)  

---

### 安装 Go

在命令行（PowerShell 或 CMD）中执行以下命令：

<img width="682" height="226" alt="安装 Go" src="https://github.com/user-attachments/assets/877b1b4c-3312-4f06-9c47-c676f2f823d6" />


```bash
winget install GoLang.Go
```

安装完成后，可以运行以下命令确认安装成功：

<img width="687" height="191" alt="检查 Go 版本" src="https://github.com/user-attachments/assets/97c814a3-a250-4e4a-b07e-3254160c510b" />


```bash
go version
```

### 安装 Node.JS（LTS）

在命令行中执行以下命令：

<img width="651" height="234" alt="安装 Node.JS" src="https://github.com/user-attachments/assets/45a9297d-0fe6-442b-8af6-643a9c2995da" />


```bash
winget install OpenJS.NodeJS.LTS
```
winget install OpenJS.NodeJS

安装完成后，可以运行以下命令确认 NPX 命令已经正常：

<img width="464" height="195" alt="确认 NPX 版本" src="https://github.com/user-attachments/assets/dfd6ef98-6993-42b2-94a6-f274dcace2ca" />


```bash
npx -v
```

祝大家使用 xiaohongshu-mcp 愉快~~~

