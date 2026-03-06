---
name: debug-with-evidence
description: evidence-first 调试流程，先复现再修复
---

# Debug with Evidence

## Checklist

1. 复现：最小输入 + 稳定触发步骤
2. 采集：错误日志、堆栈、超时点
3. 定位：先排除外部因素，再定位代码
4. 修复：最小修复，不顺带重构
5. 证明：复现脚本从 FAIL 变 PASS

## Required Artifacts

- 最小复现脚本/命令
- 修复前后对比输出
- 新增回归测试（若可自动化）
