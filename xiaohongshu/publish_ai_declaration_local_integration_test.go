//go:build integration

package xiaohongshu

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 本地页面复用发布页的触发器和浮层结构。点击次数用于区分幂等跳过与重复操作。
const aiDeclarationMockPage = `
<style>
	.d-select-wrapper, .d-option-content { width: 240px; padding: 8px }
</style>
<div class="d-select-wrapper"><div class="d-select-content">公开可见</div></div>
<div id="control" class="d-select-wrapper" lass="declaration-wrapper">
	<div class="d-select-content">添加内容类型声明</div>
</div>
<div id="menu" class="declaration-drop-down" hidden>
	<div class="d-option-content">笔记含AI合成内容说明</div>
	<div class="d-option-content" id="ai">笔记含AI合成内容</div>
</div>`

const bindAIDeclarationMock = `mode => {
	const wrapper = document.querySelector('#control');
	const label = wrapper.querySelector('.d-select-content');
	const menu = document.querySelector('#menu');
	const option = document.querySelector('#ai');
	window.clicks = {trigger: 0, option: 0};

	switch (mode) {
	case 'switch':
		const hidden = wrapper.cloneNode(true);
		hidden.removeAttribute('id');
		hidden.firstElementChild.textContent = option.textContent;
		hidden.hidden = true;
		wrapper.before(hidden);
		wrapper.removeAttribute('lass');
		label.textContent = '内容包含营销广告';
		break;
	case 'selected':
		label.textContent = option.textContent;
		break;
	case 'open':
		label.textContent = '其他内容声明';
		menu.hidden = false;
		option.remove();
		setTimeout(() => menu.append(option), 350);
		break;
	case 'blocked':
		const overlay = document.createElement('div');
		overlay.style.cssText = 'position:fixed;inset:0;z-index:10';
		document.body.appendChild(overlay);
		break;
	case 'missing':
		option.remove();
		break;
	}

	label.onclick = () => {
		window.clicks.trigger++;
		if (mode !== 'closed') menu.hidden = !menu.hidden;
	};
	menu.onclick = e => {
		window.clicks.option++;
		if (mode !== 'stays-open') menu.hidden = true;
		if (mode === 'switch') {
			const current = wrapper.cloneNode(true);
			current.firstElementChild.textContent = e.target.textContent;
			wrapper.replaceWith(current);
			return;
		}
		label.textContent = e.target.textContent;
		if (mode === 'reverted') {
			const current = wrapper.cloneNode(true);
			current.firstElementChild.textContent += '（待确认）';
			wrapper.replaceWith(current);
		}
	};
}`

func TestSetAIDeclarationLocal(t *testing.T) {
	b := newMentionFixtureBrowser(t)
	for _, tc := range []struct {
		name, mode, wantErr string
		triggers, options   int
	}{
		{"切换已有声明并忽略隐藏副本", "switch", "", 1, 1},
		{"已选中且菜单关闭时幂等跳过", "selected", "", 0, 0},
		{"菜单已打开时等待异步选项", "open", "", 0, 1},
		{"浮层遮挡触发器", "blocked", "点击内容类型声明下拉框失败", 0, 0},
		{"菜单未展开", "closed", "等待并选择可见的AI内容声明选项失败", 1, 0},
		{"菜单缺少精确匹配的选项", "missing", "等待并选择可见的AI内容声明选项失败", 1, 0},
		{"重绘后只有旧节点精确命中", "reverted", "AI内容声明未生效", 1, 1},
		{"显示已选中但菜单未关闭", "stays-open", "内容类型声明下拉菜单未关闭", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := b.MustPage("about:blank")
			defer page.Close()
			page.MustSetDocumentContent(aiDeclarationMockPage)
			page.MustEval(bindAIDeclarationMock, tc.mode)

			limit := 4 * time.Second
			if tc.mode == "blocked" {
				// 让生产代码的 10 秒上限先到期。
				limit = 30 * time.Second
			}
			action := page.Timeout(limit)
			defer action.CancelTimeout()
			start := time.Now()
			err := setAIDeclaration(action)
			if tc.mode == "blocked" {
				require.Less(t, time.Since(start), 15*time.Second)
				require.ErrorIs(t, err, context.DeadlineExceeded)
			}
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, "笔记含AI合成内容", page.MustElement("#control .d-select-content").MustText())
				require.False(t, page.MustElement("#menu").MustVisible())
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
			require.Equal(t, tc.triggers, page.MustEval("() => window.clicks.trigger").Int())
			require.Equal(t, tc.options, page.MustEval("() => window.clicks.option").Int())
			if tc.mode == "closed" || tc.mode == "missing" {
				require.Equal(t, tc.mode == "missing", page.MustElement("#menu").MustVisible())
			}
		})
	}
}
