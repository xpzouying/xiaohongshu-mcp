//go:build integration

// 集成测试：起无头浏览器验证输入行为，本地页面、不联网、不登录。
// 手动跑：go test -tags integration ./humanize/ -run TestTypeEventSequence -v
package humanize

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

const typeProbeBody = `<input id="a"><div id="b" contenteditable="true"></div>`

const typeProbeInstall = `() => {
  window.__ev = [];
  for (const t of ['input','change']) {
    window.addEventListener(t, e => {
      window.__ev.push({type: e.type, target: e.target.id || ''});
    }, true);
  }
  return true;
}`

type typedEvent struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

// countEvents 统计指定元素上各类事件的次数，并清空记录。
func countEvents(t *testing.T, page *rod.Page, target string) map[string]int {
	t.Helper()

	raw := page.MustEval(`() => { const e = window.__ev; window.__ev = []; return JSON.stringify(e) }`).Str()

	var evs []typedEvent
	if err := json.Unmarshal([]byte(raw), &evs); err != nil {
		t.Fatalf("解析事件失败: %v", err)
	}

	counts := map[string]int{}
	for _, e := range evs {
		if e.Target == target {
			counts[e.Type]++
		}
	}
	return counts
}

// assertOneInputPerRune 每个字符应恰好产生一次 input，且不应产生 change。
func assertOneInputPerRune(t *testing.T, label string, counts map[string]int, text string) {
	t.Helper()

	want := len([]rune(text))
	if counts["input"] != want {
		t.Errorf("%s: input 事件 %d 次，期望 %d 次", label, counts["input"], want)
	}
	if counts["change"] != 0 {
		t.Errorf("%s: 不应产生 change 事件，实际 %d 次", label, counts["change"])
	}
}

func TestTypeEventSequence(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	if err != nil {
		t.Skipf("SKIP: 浏览器不可用: %v", err)
	}

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank")
	page.MustWaitLoad()
	page.MustSetDocumentContent(typeProbeBody)
	if !page.MustEval(typeProbeInstall).Bool() {
		t.Fatal("安装事件监听器失败")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const text = "你好abc"

	// 普通输入框
	input := page.MustElement("#a")
	if err := Type(ctx, input, text); err != nil {
		t.Fatalf("输入框输入失败: %v", err)
	}
	assertOneInputPerRune(t, "<input>", countEvents(t, page, "a"), text)
	if got := input.MustProperty("value").Str(); got != text {
		t.Errorf("<input> 文本不符: 期望 %q，实际 %q", text, got)
	}

	// contenteditable：评论框、发布编辑器都是这种形态
	editable := page.MustElement("#b")
	if err := Type(ctx, editable, text); err != nil {
		t.Fatalf("contenteditable 输入失败: %v", err)
	}
	assertOneInputPerRune(t, "contenteditable", countEvents(t, page, "b"), text)
	if got := editable.MustText(); got != text {
		t.Errorf("contenteditable 文本不符: 期望 %q，实际 %q", text, got)
	}
}
