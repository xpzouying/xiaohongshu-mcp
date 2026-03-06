# third_party

本目录用于放置运行时依赖（不直接提交大体积源码/模型）。

当前约定（历史兼容）：

- `whisper.cpp` 安装路径：`third_party/whisper.cpp`
- 安装命令：`bash scripts/setup_whisper_cpp.sh medium`

`whisper.cpp` 目录已在 `.gitignore` 中忽略，避免将编译产物与模型提交到仓库。

> 说明：`transcribe_feed_video` 已默认切换到 GLM API 链路，不再依赖本地 whisper。  
> 该目录保留是为了兼容旧版本环境。
