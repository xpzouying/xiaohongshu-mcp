//go:build integration

package xiaohongshu

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 本地页面可在鼠标移动时替换或删除候选，稳定复现真实编辑器的重绘时序。
func TestInputTagLocal(t *testing.T) {
	b := newSearchFixtureBrowser(t)
	for _, tc := range []struct {
		name       string
		change     string
		wantPicked string
	}{
		{"相似话题排在前面", "", "#散步"},
		{"候选被替换", "replace", "#散步"},
		{"同一候选行原位改名", "rename", "#散步日记"},
		{"候选消失", "remove", ""},
		{"没有联想框", "absent", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := b.MustPage("about:blank").Timeout(12 * time.Second)
			defer page.Close()
			page.MustSetDocumentContent(`
				<div id="editor" contenteditable="true"></div>
				<div id="creator-editor-topic-container" style="margin:80px;width:240px">
					<div class="item"><span class="name">#散步日记</span><span>12万浏览</span></div>
					<div class="item"><span class="name">#散步</span><span>8万浏览</span></div>
				</div>`)
			page.MustEval(`change => {
				const list = document.getElementById('creator-editor-topic-container');
				if (change === 'absent') { list.remove(); return; }
				const editor = document.getElementById('editor');
				const appendTopic = name => {
					const a = document.createElement('a');
					a.className = 'tiptap-topic';
					a.dataset.topic = JSON.stringify({name: name.slice(1)});
					a.textContent = name;
					editor.append(a);
				};
				if (change === 'rename') appendTopic('#散步');
				list.onclick = e => {
					const name = e.target.closest('.item').querySelector('.name').textContent;
					document.body.dataset.picked = name;
					setTimeout(() => appendTopic(name), 120);
				};
				if (!change) return;
				list.firstElementChild.remove();
				document.addEventListener('mousemove', () => {
					const item = list.lastElementChild;
					if (change === 'rename') item.querySelector('.name').textContent = '#散步日记';
					else if (change === 'replace') item.replaceWith(item.cloneNode(true));
					else item.remove();
					document.body.dataset.changed = change;
				}, {once: true});
			}`, tc.change)
			editor := page.MustElement("#editor")
			err := inputTag(page.GetContext(), editor, "散步")
			if tc.change == "rename" {
				require.ErrorContains(t, err, "插入的话题与请求不符")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantPicked, page.MustEval("() => document.body.dataset.picked || ''").String())
			if tc.change == "replace" || tc.change == "remove" {
				require.Equal(t, tc.change, page.MustEval("() => document.body.dataset.changed").String())
			}
			if tc.wantPicked == "" {
				got := editor.MustProperty("textContent").String()
				require.Equal(t, "#散步 ", strings.ReplaceAll(got, "\u00a0", " "))
			}
		})
	}
	t.Run("调用方超时", func(t *testing.T) {
		page := b.MustPage("about:blank")
		defer page.Close()
		selected, err := selectTopicItem(page.Timeout(100*time.Millisecond), "散步")
		require.False(t, selected)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
