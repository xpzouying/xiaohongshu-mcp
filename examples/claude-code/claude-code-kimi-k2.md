# Claude Code With kimi-k2

由于 Claude Code 的各种限制，对于普通用户来说门槛太高，不推荐普通用户使用，不过推荐一种替代方案，可以让 Claude Code 接入国内 kimi-k2 的模型，实现同样的功能。使用国内的其他支持 Claude Code 的模型厂商都大同小异，这里以 kimi-k2 为例。


## 1. 申请 API Key。

前往Kimi开放平台申请API Key。

点击前往：[Kimi开放平台](https://platform.moonshot.cn/) - 点击 [控制台](https://platform.moonshot.cn/console)

<img width="1024" height="781" alt="image" src="https://github.com/user-attachments/assets/1cdd8bb7-f198-48f8-b5b0-ca42c671a3ae" />

点击进入 [API Key管理]，新建一个新的 API Key，保存下来 API Key，后面会用到。

<img width="2048" height="543" alt="image" src="https://github.com/user-attachments/assets/a3fd3226-2f91-4616-8a1e-10a0a8755b93" />

## 2. 一键安装

直接参考开源项目：[LLM-Red-Team/kimi-cc](https://github.com/LLM-Red-Team/kimi-cc)

**重点说明：**

- 准备好上一步骤的 API Key，安装过程中会要求你输出 API Key（隐藏式的）

一键安装脚本：

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/LLM-Red-Team/kimi-cc/refs/heads/main/install.sh)"
```

安装过程中，会“暂停”要求输入 API Key，直接复制进去，然后回车即可。

<img width="934" height="342" alt="image" src="https://github.com/user-attachments/assets/a1776764-a577-4e90-9354-5ae6162c8b13" />

成功安装后，完成。

<img width="925" height="533" alt="image" src="https://github.com/user-attachments/assets/0b69c6d2-2ed9-40b0-acbd-521585160675" />

安装成功后，一定要重启 SHELL 环境或者重新加载对应的环境变量，按照日志中，输入即可。

<img width="824" height="337" alt="image" src="https://github.com/user-attachments/assets/26129b99-3e68-43b9-b86d-8a5a74bae718" />

按照提示，输入：

<img width="588" height="122" alt="image" src="https://github.com/user-attachments/assets/c28bf3d6-5e53-44d6-885e-10ecec64fb46" />

再次运行 [claude] 后，确认是否是自己输入的 API Key，确认后，选择 YES！

<img width="593" height="259" alt="image" src="https://github.com/user-attachments/assets/262dbf1c-5c79-4b8e-8108-a2749ab8c36c" />

然后可能会让你确定一些协议，点击 YES 后，正式打开了 Claude Code，不过此时已经为你接上 Kimi-K2 的模型了。

<img width="786" height="526" alt="image" src="https://github.com/user-attachments/assets/50986c96-6f45-4074-b1c4-c8858e914c0e" />

注意这里的 API-Key 是 Kimi API Key，API Base URL 是 moonshot.cn 域名下的 URL，表示连接到 Kimi 的 API 了。

## 3. 下载 MCP 程序

从 [Release](https://github.com/xpzouying/xiaohongshu-mcp/releases) 中下载对应的二进制后启动。（以 Ubuntu 系统为例）


## 4. 接入 MCP

参考 [README 文档 - 接入 MCP 章节](https://github.com/xpzouying/xiaohongshu-mcp/tree/add-claude-code-kimi-k2-examples?tab=readme-ov-file#22-%E6%94%AF%E6%8C%81%E7%9A%84%E5%AE%A2%E6%88%B7%E7%AB%AF)

