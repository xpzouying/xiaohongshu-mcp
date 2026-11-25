//go:build integration

package xiaohongshu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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
<div id="menu-container">
	<div id="menu" class="declaration-drop-down" hidden>
		<div class="d-option-content">笔记含AI合成内容说明</div>
		<div class="d-option-content" id="ai">笔记含AI合成内容</div>
	</div>
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
	case 'hidden-label':
		const stale = label.cloneNode(true);
		stale.textContent = option.textContent;
		stale.hidden = true;
		label.before(stale);
		break;
	case 'hidden-only':
		label.textContent = option.textContent;
		label.hidden = true;
		break;
	case 'selected':
		label.textContent = option.textContent;
		break;
	case 'transparent-menu':
		label.textContent = option.textContent;
		menu.hidden = false;
		menu.parentElement.style.opacity = '0';
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
		if (mode === 'transparent-after-click') {
			menu.parentElement.style.opacity = '0';
		} else if (mode !== 'stays-open') {
			menu.hidden = true;
		}
		if (mode === 'switch') {
			const current = wrapper.cloneNode(true);
			current.firstElementChild.textContent = e.target.textContent;
			wrapper.replaceWith(current);
			return;
		}
		if (mode === 'hidden-after-click') {
			const stale = label.cloneNode(true);
			stale.textContent = e.target.textContent;
			stale.hidden = true;
			label.before(stale);
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
		{"跳过排在可见标签前的隐藏旧标签", "hidden-label", "", 1, 1},
		{"只有隐藏标签时拒绝确认声明", "hidden-only", "未找到内容类型声明下拉框", 0, 0},
		{"点击后只有隐藏标签显示声明已选中", "hidden-after-click", "AI内容声明未生效", 1, 1},
		{"已选中且菜单关闭时幂等跳过", "selected", "", 0, 0},
		{"菜单已打开时等待异步选项", "open", "", 0, 1},
		{"已选中的声明跳过透明菜单", "transparent-menu", "", 0, 0},
		{"点击后透明菜单视为已关闭", "transparent-after-click", "", 1, 1},
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
				require.Equal(t, "笔记含AI合成内容", page.MustElement("#control .d-select-content:not([hidden])").MustText())
				if tc.mode == "transparent-menu" || tc.mode == "transparent-after-click" {
					require.True(t, page.MustElement("#menu").MustVisible(), "透明菜单仍占据布局空间")
					require.Equal(t, "0", page.MustEval("() => getComputedStyle(document.querySelector('#menu-container')).opacity").Str())
				} else {
					require.False(t, page.MustElement("#menu").MustVisible())
				}
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

// 在首个鼠标命令发送时取消请求，确认 CDP 收到同一个上下文。
func TestAIDeclarationClickUsesDeadline(t *testing.T) {
	for _, tc := range []struct {
		mode, wantErr string
	}{
		{"normal", "点击内容类型声明下拉框失败"},
		{"open", "等待并选择可见的AI内容声明选项失败"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var input *declarationInputProbe
			browser := newSearchFixtureBrowser(t, func(client rod.CDPClient) rod.CDPClient {
				input = &declarationInputProbe{CDPClient: client, cancel: cancel}
				return input
			})
			page := browser.MustPage("about:blank")
			defer page.Close()
			page.MustSetDocumentContent(aiDeclarationMockPage)
			page.MustEval(bindAIDeclarationMock, tc.mode)

			start := time.Now()
			err := setAIDeclaration(page.Context(ctx))
			require.True(t, input.called)
			require.WithinRange(t, input.deadline, start.Add(10*time.Second), start.Add(11*time.Second))
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

type declarationInputProbe struct {
	rod.CDPClient
	cancel   context.CancelFunc
	deadline time.Time
	called   bool
}

func (p *declarationInputProbe) Call(ctx context.Context, sessionID, method string, params interface{}) ([]byte, error) {
	switch method {
	case (proto.InputDispatchMouseEvent{}).ProtoReq(), (proto.InputSynthesizeTapGesture{}).ProtoReq():
		p.called = true
		p.deadline, _ = ctx.Deadline()
		p.cancel()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("输入命令的上下文没有被取消")
	default:
		return p.CDPClient.Call(ctx, sessionID, method, params)
	}
}
