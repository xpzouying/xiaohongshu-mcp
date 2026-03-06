# Browser Automation Rules

- 首选 go-rod 元素操作：点击、输入、等待、状态检查。
- 减少 `Eval/JavaScript` 注入；若必须使用，需说明原因与风险。
- 关键步骤失败时输出可追踪上下文（页面阶段、选择器、超时）。
- 涉及登录态逻辑时，必须说明 cookies/profile 影响范围。
