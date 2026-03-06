# Codex 协作入口（xiaohongshu-mcp）

本文件是仓库级入口配置，目标是让 AI 协作从“单次对话”升级为“可复用工程流程”。

## 默认工作流

1. `/plan`：先拆任务，再定义验证标准（evidence）
2. `/implement`：按子任务实现，单次只做一件事
3. `/verify`：运行格式化、测试、回归检查
4. `/review`：执行结构化审查（风险分级）
5. `/create-pr`：生成 PR 描述并提交人工复核

命令脚本见：`.codex/commands/`

## 路由规则（按需加载）

- 全局约束：`.codex/rules/global.md`
- Go 后端改动：`.codex/rules/go-backend.md`
- 小红书浏览器自动化：`.codex/rules/browser-automation.md`
- 文档/API 改动：`.codex/rules/documentation.md`
- 验证与回归：`.codex/rules/testing.md`
- MCP 工具调用：`.codex/skills/xhs-mcp-tools-playbook/SKILL.md`
- 评论回复优化：`.codex/skills/xhs-humanized-comment-reply/SKILL.md`
- 账号起号与养号：`.codex/skills/xhs-account-bootstrapping/SKILL.md`

## 证据优先（Evidence-First）

- 任何改动前必须定义：输入、输出、验收命令、失败信号。
- Bug 修复必须先给最小复现（MRE），模板见 `.codex/evidence/min-repro.md`。
- 无法提供可执行 evidence 的任务，不进入实现阶段。

## 角色分工（多 Session/多 Agent）

- Planner：拆解任务与验收标准（`.codex/agents/planner.md`）
- Implementer：按约束编码（`.codex/agents/implementer.md`）
- Reviewer：结构化审查（`.codex/agents/reviewer.md`）
- Debugger：基于日志与复现定位（`.codex/agents/debugger.md`）

## 底线约束

- 保持最小改动，不做无关重构。
- 优先复用现有模式，不引入过度设计。
- 浏览器操作优先 go-rod API，避免大量 JS 注入。
- 最终交付前必须通过 `/.codex/commands/verify.sh` 对应的验证流程。
