# Codex Configuration Architecture Design

**Goal:** 将可复用的 AI 协作方法集成到仓库，形成“分层规则 + 角色分工 + 命令流程 + evidence 驱动”的工程化工作流。

## Scope

- 新增仓库级入口与目录级约束
- 新增 `.codex/` 六层配置骨架
- 新增使用文档与执行命令

## Non-goals

- 不修改业务功能逻辑
- 不引入新的运行时依赖

## Validation

- 验证脚本存在且可执行
- 文档路径完整、交叉引用可用
- 仓库测试可正常执行
