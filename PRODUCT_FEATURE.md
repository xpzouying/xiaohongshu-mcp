# 商品笔记发布功能

## 功能概述

新增了在发布小红书图文笔记时添加商品的能力。用户可以在调用 MCP 的 `publish_content` 工具时传入商品名称列表，服务会自动在发布页面中打开商品选择弹窗、搜索匹配的商品并完成多选保存。

## 新增参数

| 参数名 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `products` | `[]string` | 否 | 商品名称关键词列表，系统按包含关系匹配，可多选（最多18个） |

调用示例：

```json
{
  "title": "春季养生茶推荐",
  "content": "分享几款好喝的养生茶～",
  "images": ["/path/to/tea-1.jpg", "/path/to/tea-2.jpg"],
  "tags": ["养生", "茶饮"],
  "products": ["蒲菊枸杞决明子茶", "正宗特级湘西莓茶"]
}
```

## 自动化流程概述

1. 点击发布页的「添加商品」按钮：
   - 优先查找 `div.multi-good-select-empty-btn button`
   - 回退到 `div.multi-good-select-add-btn button`
   - 若仍未找到，使用文本匹配方式定位包含“添加商品”的按钮。
2. 等待弹出 `div.multi-goods-selector-modal` 弹窗。
3. 对每个商品关键词依次执行：
   - 在 `input[placeholder='搜索商品ID 或 商品名称']` 输入框中注入关键词并触发输入事件。
   - 遍历 `.good-card-container` 列表，通过 `.sku-name` 文本做不区分大小写的包含匹配。
   - 点击 `.d-checkbox-main` 复选框，确保对应商品处于选中状态。
4. 全部商品处理完成后，点击弹窗底部的「保存」按钮并等待弹窗关闭。

## 重要细节

- **最大数量**：小红书限制一次最多选择18个商品，代码会依次处理传入的关键词列表，不做额外截断，需由调用侧保证数量合法。
- **匹配策略**：使用包含匹配（`strings.Contains(strings.ToLower(name), strings.ToLower(keyword))`），即商品名称中包含关键词即可选中。
- **状态校验**：通过执行 `checkboxInput.Eval("() => this.checked")` 并读取 `res.Value.Bool()` 确认复选框状态，确保选中成功。
- **弹窗收尾**：点击保存后循环使用 `page.Has("div.multi-goods-selector-modal")` 检查弹窗是否关闭，避免立即进入下一步导致失败。

## 受影响的代码模块

- `mcp_server.go`：`PublishContentArgs` 增加 `Products` 字段，并在工具调用时传递。
- `mcp_handlers.go`：解析 `products` 参数并写入 `PublishRequest`。
- `service.go`：`PublishRequest` 与 `PublishImageContent` 增加 `Products` 字段。
- `xiaohongshu/publish.go`：
  - 在发布流程中新增 `addProducts` 调用。
  - 实现 `addProducts` 及其辅助函数，负责商品弹窗的操作与校验。

## 使用提示

- 商品必须已在小红书上架，否则无法在弹窗中检索到。
- 若某个关键词未匹配到商品，会直接返回错误并终止发布流程。
- 若页面结构调整导致选择器失效，需要更新对应的查找逻辑。
