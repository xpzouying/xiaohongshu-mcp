//go:build integration

// 集成测试：起无头浏览器验证输入行为，本地页面、不联网、不登录。
// 手动跑：go test -tags integration ./humanize/ -v
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

const clickProbeBody = `<button id="btn" style="position:absolute;top:120px;left:80px;width:200px;height:60px">click</button>`

const clickProbeInstall = `() => {
  window.__clicks = [];
  const b = document.getElementById('btn');
  b.addEventListener('mousedown', e => window.__clicks.push(
    {t:'down', ts:e.timeStamp, x:e.offsetX, y:e.offsetY}));
  b.addEventListener('mouseup', e => window.__clicks.push(
    {t:'up', ts:e.timeStamp, x:e.offsetX, y:e.offsetY}));
  return true;
}`

type clickRecord struct {
	T  string  `json:"t"`
	TS float64 `json:"ts"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// TestClickTiming 校验 Click 的按下时长与落点分布符合约定。
func TestClickTiming(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	if err != nil {
		t.Skipf("SKIP: 浏览器不可用: %v", err)
	}

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank")
	page.MustWaitLoad()
	page.MustSetDocumentContent(clickProbeBody)
	if !page.MustEval(clickProbeInstall).Bool() {
		t.Fatal("安装点击监听器失败")
	}

	const rounds = 12
	btn := page.MustElement("#btn")
	for i := 0; i < rounds; i++ {
		if err := Click(btn); err != nil {
			t.Fatalf("第 %d 次点击失败: %v", i+1, err)
		}
	}

	var recs []clickRecord
	raw := page.MustEval(`() => JSON.stringify(window.__clicks)`).Str()
	if err := json.Unmarshal([]byte(raw), &recs); err != nil {
		t.Fatalf("解析点击记录失败: %v", err)
	}
	if len(recs) != rounds*2 {
		t.Fatalf("应记录 %d 条 down/up，实际 %d 条", rounds*2, len(recs))
	}

	minHold := DefaultProvider{}.Timing()[ClickHold].Min
	points := map[[2]float64]struct{}{}

	for i := 0; i < len(recs); i += 2 {
		down, up := recs[i], recs[i+1]
		if down.T != "down" || up.T != "up" {
			t.Fatalf("第 %d 组事件顺序异常: %s/%s", i/2+1, down.T, up.T)
		}
		// timeStamp 单位是毫秒；留 10ms 余量给时钟精度
		hold := time.Duration(up.TS-down.TS) * time.Millisecond
		if hold+10*time.Millisecond < minHold {
			t.Errorf("第 %d 次按压时长 %v，短于下限 %v", i/2+1, hold, minHold)
		}

		points[[2]float64{down.X, down.Y}] = struct{}{}
	}

	if len(points) < 2 {
		t.Errorf("%d 次点击落点完全相同（%v），抖动未生效", rounds, points)
	}
	t.Logf("%d 次点击产生 %d 个不同落点", rounds, len(points))
}

const guardHTML = `<body style="margin:0;width:800px;height:600px">
<div data-n="normal"      style="position:absolute;left:100px;top:100px;width:96px;height:40px">x</div>
<div data-n="display"     style="display:none">x</div>
<div data-n="visibility"  style="position:absolute;left:100px;top:200px;width:96px;height:40px;visibility:hidden">x</div>
<div data-n="opacity0"    style="position:absolute;left:100px;top:260px;width:96px;height:40px;opacity:0">x</div>
<div data-n="offleft"     style="position:absolute;left:-9999px;top:100px;width:96px;height:40px">x</div>
<div data-n="belowfold"   style="position:absolute;left:100px;top:5000px;width:96px;height:40px">x</div>
<div data-n="pointernone" style="position:absolute;left:300px;top:100px;width:96px;height:40px;pointer-events:none">x</div>
<script>
window.HIT = [];
document.querySelectorAll('[data-n]').forEach(el =>
  el.addEventListener('click', () => window.HIT.push(el.dataset.n), true));
</script></body>`

// TestClickGuards 校验点击前的落点检查：够不到的目标必须报错，而不是静默落空；
// opacity:0 和 pointer-events:none 则必须放行。
func TestClickGuards(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	if err != nil {
		t.Skipf("SKIP: 浏览器不可用: %v", err)
	}

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank")
	page.MustWaitLoad()
	page.MustSetDocumentContent(guardHTML)

	cases := []struct {
		name    string
		blocked bool // 是否应当被拦下
		reason  string
	}{
		{"normal", false, "正常元素"},
		{"display", true, "display:none 拿不到可点区域"},
		{"visibility", true, "visibility:hidden 不参与命中测试"},
		{"opacity0", false, "opacity:0 仍可命中，须放行"},
		{"offleft", true, "落点在视口左侧之外"},
		{"belowfold", true, "落点在视口下方之外"},
		{"pointernone", false, "pointer-events:none 会穿透，须放行"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			el := page.MustElement(`[data-n="` + c.name + `"]`)
			page.MustEval(`() => { window.HIT = [] }`)

			err := ClickNoWait(el)
			if c.blocked {
				if err == nil {
					t.Errorf("%s（%s）应被拦下，实际放行了", c.name, c.reason)
				}
				return
			}
			if err != nil {
				t.Errorf("%s（%s）不应被拦下，实际报错: %v", c.name, c.reason, err)
			}
		})
	}
}
