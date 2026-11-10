//go:build integration

// 本地页面集成测试：不需要登录、不触网，用 mock DOM 覆盖 @ 提及的选择与降级分支。
//
//	go test -tags integration ./xiaohongshu/ -run TestInputMentionLocal -v
package xiaohongshu

import (
	"fmt"
	"html"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/require"
)

// newMentionFixtureBrowser 起一个只用于本地 mock 页面的无头浏览器。
// 关掉 leakless：Windows Defender 会把 rod 释放的 leakless.exe 当成可疑程序拦掉。
func newMentionFixtureBrowser(t *testing.T) *rod.Browser {
	t.Helper()

	bin := os.Getenv("XHS_TEST_BROWSER")
	if bin == "" {
		var ok bool
		bin, ok = launcher.LookPath()
		if !ok {
			t.Skip("未找到本地浏览器，可用 XHS_TEST_BROWSER 指定路径")
		}
	}

	l := launcher.New().Bin(bin).Headless(true).Leakless(false)
	controlURL, err := l.Launch()
	require.NoError(t, err)

	b := rod.New().ControlURL(controlURL).NoDefaultDevice()
	require.NoError(t, b.Connect())

	t.Cleanup(func() {
		_ = b.Close()
		l.Cleanup()
	})
	return b
}

// mentionMockPage 按真机探测到的结构造的 mock（2026-11-06 probe）：
// 候选行是 div.item，昵称是夹在头像 img 之间的纯文本节点，没有 .name 类名；
// 点击后插入 <a class="tiptap-at" data-user='{"userId":"...","nickname":"..."}'>。
const mentionMockPage = `
<div id="editor" contenteditable="true" style="width:400px;height:200px"></div>
<div id="creator-editor-at-container" class="items" style="margin:40px;width:240px">
	<div class="item is-selected" style="height:40px"><img src="x.png"> 小明同学 <img class="verify-icon" style="display:none"></div>
	<div class="item" style="height:40px"><img src="y.png"> 小明 <img class="verify-icon" style="display:none"></div>
</div>`

// jsMentionPick 点击候选时把昵称写进 body.picked，并往编辑器插一个真实结构的提及节点。
const jsMentionPick = `() => {
	const list = document.getElementById('creator-editor-at-container');
	list.onclick = e => {
		const item = e.target.closest('.item');
		const name = item.innerText.trim().split('\n')[0].trim();
		document.body.dataset.picked = name;
		const a = document.createElement('a');
		a.className = 'tiptap-at';
		a.contentEditable = 'false';
		a.dataset.user = JSON.stringify({userId: 'u-' + name, nickname: name});
		a.textContent = '@' + name;
		document.getElementById('editor').append(a);
	};
}`

func TestInputMentionLocal(t *testing.T) {
	b := newMentionFixtureBrowser(t)

	t.Run("精确匹配昵称而不是第一个候选", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		page.MustEval(jsMentionPick)

		editor := page.MustElement("#editor")
		require.NoError(t, inputMentions(page.GetContext(), editor, []string{"@小明"}))
		require.Equal(t, "小明", page.MustEval("() => document.body.dataset.picked || ''").String())
		require.Equal(t, 1, countMentionNodes(editor))
	})

	t.Run("没有同名候选时退成纯文本且不报错", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		page.MustEval(`() => {
			document.getElementById('creator-editor-at-container').innerHTML =
				'<div class="item" style="height:40px"><img src="x.png"> 小明同学 <img class="verify-icon" style="display:none"></div>';
		}`)

		editor := page.MustElement("#editor")
		require.NoError(t, inputMentions(page.GetContext(), editor, []string{"小明"}))
		require.Equal(t, 0, countMentionNodes(editor), "不应插入提及节点")
		require.Contains(t, editor.MustProperty("textContent").String(), "@小明")
	})

	t.Run("候选被重绘后仍能插入", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		page.MustEval(jsMentionPick)
		page.MustEval(`() => {
			const list = document.getElementById('creator-editor-at-container');
			document.addEventListener('mousemove', () => {
				const item = list.lastElementChild;
				item.replaceWith(item.cloneNode(true));
				document.body.dataset.replaced = '1';
			}, {once: true});
		}`)

		editor := page.MustElement("#editor")
		require.NoError(t, inputMentions(page.GetContext(), editor, []string{"小明"}))
		require.Equal(t, "小明", page.MustEval("() => document.body.dataset.picked || ''").String())
		require.Equal(t, "1", page.MustEval("() => document.body.dataset.replaced || ''").String())
	})

	t.Run("点中候选但节点结构未识别时阻断", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		page.MustEval(`() => {
			const list = document.getElementById('creator-editor-at-container');
			list.onclick = e => {
				const item = e.target.closest('.item');
				document.body.dataset.picked = item.innerText.trim().split('\n')[0].trim();
			};
		}`)

		editor := page.MustElement("#editor")
		require.Error(t, inputMentions(page.GetContext(), editor, []string{"小明"}),
			"未确认插入结果时必须阻断发布")
		require.Equal(t, "小明", page.MustEval("() => document.body.dataset.picked || ''").String())
	})

	t.Run("插入节点的昵称与请求不一致时阻断", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		// 模拟点中后插入的是另一个人（data-user 的 nickname 对不上）
		page.MustEval(`() => {
			const list = document.getElementById('creator-editor-at-container');
			list.onclick = e => {
				document.body.dataset.picked = e.target.closest('.item').innerText.trim();
				const a = document.createElement('a');
				a.className = 'tiptap-at';
				a.contentEditable = 'false';
				a.dataset.user = JSON.stringify({userId: 'u-x', nickname: '小明同学'});
				a.textContent = '@小明同学';
				document.getElementById('editor').append(a);
			};
		}`)

		editor := page.MustElement("#editor")
		require.ErrorContains(t, inputMentions(page.GetContext(), editor, []string{"小明"}),
			"插入的@节点与请求不一致")
		require.Equal(t, "小明", page.MustEval("() => document.body.dataset.picked || ''").String())
	})

	t.Run("同名但ID不匹配时阻断", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		page.MustEval(jsMentionPick)

		err := inputMentions(page.GetContext(), page.MustElement("#editor"),
			[]string{"小明|0123456789abcdef01234567"})
		require.ErrorContains(t, err, "插入的@节点与请求不一致")
	})

	t.Run("只给 user_id 时用 id 搜索并取第一个候选", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)
		page.MustEval(`() => {
			const list = document.getElementById('creator-editor-at-container');
			list.onclick = e => {
				const item = e.target.closest('.item');
				const name = item.innerText.trim().split('\n')[0].trim();
				document.body.dataset.picked = name;
				const a = document.createElement('a');
				a.className = 'tiptap-at';
				a.contentEditable = 'false';
				a.dataset.user = JSON.stringify({userId: '0123456789abcdef01234567', nickname: name});
				a.textContent = '@' + name;
				document.getElementById('editor').append(a);
			};
		}`)

		editor := page.MustElement("#editor")
		require.NoError(t, inputMentions(page.GetContext(), editor, []string{"0123456789abcdef01234567"}))
		require.Equal(t, 1, countMentionNodes(editor), "应取第一个候选插入")
		require.Equal(t, "小明同学", page.MustEval("() => document.body.dataset.picked || ''").String())
		require.Equal(t, "0123456789abcdef01234567",
			page.MustEval(`() => JSON.parse(document.querySelector('a.tiptap-at').getAttribute('data-user')).userId`).String())
	})

	t.Run("空昵称被跳过", func(t *testing.T) {
		page := b.MustPage("about:blank").Timeout(20 * time.Second)
		defer page.Close()
		page.MustSetDocumentContent(mentionMockPage)

		editor := page.MustElement("#editor")
		require.NoError(t, inputMentions(page.GetContext(), editor, []string{"", "  ", "@"}))
		require.Equal(t, "", strings.TrimSpace(editor.MustProperty("textContent").String()))
	})
}

// 没有候选时会重试两次，每次只能删除刚输入的 @ 和查询词。
func TestInputMentionRetryLocal(t *testing.T) {
	b := newMentionFixtureBrowser(t)
	page := b.MustPage("about:blank").Timeout(45 * time.Second)
	defer page.Close()
	page.MustSetDocumentContent(`<div id="editor" contenteditable="true"></div>`)
	editor := page.MustElement("#editor")
	require.NoError(t, editor.Input("原文X"))
	const userID = "0123456789abcdef01234567"

	err := inputMentions(page.GetContext(), editor, []string{userID})
	require.ErrorContains(t, err, "没能搜到候选用户")
	require.Equal(t, "原文X@"+userID, editor.MustText(), "重试不得删除已有正文")
}

// 静态页面覆盖身份和数量校验；TestInputMentionLocal 验证错误传回输入入口。
func TestVerifyInsertedMentionIdentityLocal(t *testing.T) {
	b := newMentionFixtureBrowser(t)
	const userID = "0123456789abcdef01234567"
	tests := []struct {
		name    string
		target  string
		attr    string
		count   int
		wantErr bool
	}{
		{"正确ID允许昵称变化", "小明|" + userID, `{"userId":"0123456789abcdef01234567","nickname":"新昵称"}`, 1, false},
		{"同名错误ID", "小明|" + userID, `{"userId":"aaaaaaaaaaaaaaaaaaaaaaaa","nickname":"小明"}`, 1, true},
		{"只有ID时选到错误用户", userID, `{"userId":"aaaaaaaaaaaaaaaaaaaaaaaa","nickname":"小明"}`, 1, true},
		{"同名但缺少ID", "小明|" + userID, `{"nickname":"小明"}`, 1, true},
		{"同名但空ID", "小明|" + userID, `{"userId":"","nickname":"小明"}`, 1, true},
		{"同名但属性损坏", "小明|" + userID, `not-json`, 1, true},
		{"同名但属性缺失", "小明|" + userID, "", 1, true},
		{"额外插入节点", "小明|" + userID, `{"userId":"0123456789abcdef01234567","nickname":"小明"}`, 2, true},
		{"只给昵称匹配", "小明", `{"userId":"0123456789abcdef01234567","nickname":"小明"}`, 1, false},
		{"只给昵称不匹配", "别人", `{"userId":"0123456789abcdef01234567","nickname":"小明"}`, 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page := b.MustPage("about:blank").Timeout(20 * time.Second)
			defer page.Close()
			attr := ""
			if tc.attr != "" {
				attr = fmt.Sprintf(` data-user="%s"`, html.EscapeString(tc.attr))
			}
			node := fmt.Sprintf(`<a class="tiptap-at" contenteditable="false"%s>@小明</a>`, attr)
			page.MustSetDocumentContent(`<div id="editor">` + strings.Repeat(node, tc.count) + `</div>`)

			err := verifyInsertedMention(page.MustElement("#editor"), 0, parseAtTarget(tc.target))
			if tc.wantErr {
				require.Error(t, err, "身份或节点数量未确认时必须阻断发布")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
