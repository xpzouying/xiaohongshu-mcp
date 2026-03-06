# AI 协作配置架构（通用版）

本文档描述 `xiaohongshu-mcp` 的通用 AI 协作配置架构，目标是提升复杂任务中的可控性、可验证性和并行效率。

## 1. 六层架构

1. Context Layer（入口与路由）
- 根入口：`AGENTS.md`
- 目录级覆盖：`pkg/AGENTS.md`、`xiaohongshu/AGENTS.md`、`docs/AGENTS.md`

2. Rule Layer（规则分层）
- `.codex/rules/*.md`
- 按任务类型按需加载，避免上下文过载

3. Role Layer（专家分工）
- `.codex/agents/planner.md`
- `.codex/agents/implementer.md`
- `.codex/agents/reviewer.md`
- `.codex/agents/debugger.md`

4. Workflow Layer（命令化流程）
- `.codex/commands/plan.sh`
- `.codex/commands/verify.sh`
- `.codex/commands/review.sh`
- `.codex/commands/create-pr.sh`

5. Evidence Layer（证据驱动）
- `.codex/evidence/task-contract.md`
- `.codex/evidence/min-repro.md`
- `.codex/evidence/failure-report.md`

6. Skills Layer（可复用技能）
- `.codex/skills/plan-task/SKILL.md`
- `.codex/skills/add-http-endpoint/SKILL.md`
- `.codex/skills/debug-with-evidence/SKILL.md`

## 2. 推荐执行顺序

```bash
# 1) 先产出计划
./.codex/commands/plan.sh add-user-profile-cache

# 2) 实现并验证
./.codex/commands/verify.sh

# 3) 审查风险
./.codex/commands/review.sh main

# 4) 生成 PR 摘要
./.codex/commands/create-pr.sh main
```

## 3. 关键原则

- 先 evidence，后实现。
- 每个子任务必须有可执行验收标准。
- 并行只用于相互独立的子任务。
- 对发布/登录/账号相关流程优先保证稳定性和可观测性。
