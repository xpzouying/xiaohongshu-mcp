package xiaohongshu

import (
	"fmt"

	"github.com/go-rod/rod"
)

// DumpPublishDOM 诊断用：抓取发布页里包含关键字的文本节点，向上取两级父元素，
// 返回其 outerHTML（每条截断到 800 字符，最多 limit 条）。仅读取 DOM，不做任何交互。
func DumpPublishDOM(page *rod.Page, keyword string, limit int) (string, error) {
	js := `(kw, lim) => {
		const out = [];
		const seen = new Set();
		const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
		while (walker.nextNode()) {
			const node = walker.currentNode;
			if (!node.textContent || !node.textContent.includes(kw)) continue;
			let el = node.parentElement;
			for (let i = 0; i < 2 && el && el.parentElement; i++) el = el.parentElement;
			if (!el || seen.has(el)) continue;
			seen.add(el);
			out.push(el.outerHTML.slice(0, 800));
			if (out.length >= lim) break;
		}
		return out.join('\n=====\n');
	}`

	res, err := page.Eval(js, keyword, limit)
	if err != nil {
		return "", fmt.Errorf("执行 DOM 抓取失败: %w", err)
	}
	return res.Value.Str(), nil
}
