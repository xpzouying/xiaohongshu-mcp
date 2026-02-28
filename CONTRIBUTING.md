# 贡献指南 | Contributing Guide

感谢你对本项目的关注！为了保证代码质量和 Review 效率，请在提交 PR 前仔细阅读以下规范。

Thank you for your interest! Please read this guide carefully before submitting a PR.

---

## 基本流程 | Basic Workflow

1. Fork 本仓库并创建功能分支
2. 在本地完成开发和测试
3. 提交 PR 并填写清晰的描述

---

## PR 提交规范 | PR Requirements

### 1. 一个 PR 只做一件事 | One PR, One Feature

每个 PR 只包含 **一个功能或一个修复**。多个功能请拆分为多个 PR。

Each PR should contain **only one feature or one fix**. Split multiple features into separate PRs.

### 2. 必须经过验证 | Must Be Verified

**即使代码是 AI 生成的，也必须在本地运行并验证功能正确。** 未经验证的 PR 将直接关闭。

**Even if the code is AI-generated, you must run and verify it locally.** Unverified PRs will be closed.

### 3. 提供演示截图/视频 | Provide Demo

PR 中请附上功能演示的 **截图或录屏**，让 Reviewer 快速理解改动效果。

Please attach **screenshots or screen recordings** to demonstrate the feature.

> **隐私提醒：演示中务必对自己的账号信息进行打码处理！**
>
> **Privacy: Always blur/mask your account info in demos!**

### 4. 禁止大量 JS 注入 | No Excessive JS Injection

本项目使用 [go-rod](https://go-rod.github.io/) 进行浏览器自动化。**严禁通过大量注入 JavaScript 的方式操作页面元素**，应使用 go-rod 提供的 API 操作元素。

违反此规则的 PR **一律不予合并**。

This project uses [go-rod](https://go-rod.github.io/) for browser automation. **Do NOT manipulate page elements by injecting large amounts of JavaScript.** Use go-rod's API instead.

PRs violating this rule **will NOT be merged**.

### 5. 代码规范 | Code Style

- Go 代码需要格式化（`gofmt`）
- 注释使用中文，专业术语可用英文
- 不要过度设计，保持简洁

---

## 提交 Checklist | PR Checklist

提交前请确认：

- [ ] 代码已在本地运行并验证通过
- [ ] 一个 PR 仅包含一个功能/修复
- [ ] 附上演示截图或录屏（账号信息已打码）
- [ ] 没有大量 JS 注入，使用 go-rod API 操作元素
- [ ] 代码已格式化，注释清晰

---

感谢你的贡献！🎉 | Thanks for contributing!
