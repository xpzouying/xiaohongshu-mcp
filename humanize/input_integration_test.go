//go:build integration

package humanize

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
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

	input := page.MustElement("#a")
	if err := Type(ctx, input, text); err != nil {
		t.Fatalf("输入框输入失败: %v", err)
	}
	assertOneInputPerRune(t, "<input>", countEvents(t, page, "a"), text)
	if got := input.MustProperty("value").Str(); got != text {
		t.Errorf("<input> 文本不符: 期望 %q，实际 %q", text, got)
	}

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
<div data-n="opacitylow"  style="position:absolute;left:100px;top:320px;width:96px;height:40px;opacity:0.001">x</div>
<div data-n="opacitydim"  style="position:absolute;left:100px;top:380px;width:96px;height:40px;opacity:0.6">x</div>
<div data-n="ancestordim" style="position:absolute;left:100px;top:440px;opacity:0.001"><div data-n="inancestor" style="width:96px;height:40px;opacity:1">x</div></div>
<div data-n="offleft"     style="position:absolute;left:-9999px;top:100px;width:96px;height:40px">x</div>
<div data-n="belowfold"   style="position:absolute;left:100px;top:5000px;width:96px;height:40px">x</div>
<div data-n="pointernone" style="position:absolute;left:300px;top:100px;width:96px;height:40px;pointer-events:none">x</div>
<script>
window.HIT = [];
document.querySelectorAll('[data-n]').forEach(el =>
  el.addEventListener('click', () => window.HIT.push(el.dataset.n), true));
</script></body>`

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
		blocked bool
		reason  string
	}{
		{"normal", false, "正常元素"},
		{"display", true, "拿不到可点区域"},
		{"visibility", true, "不可命中"},
		{"opacity0", true, "不可见"},
		{"opacitylow", true, "低于可见下限"},
		{"opacitydim", false, "肉眼可见，须放行"},
		{"inancestor", true, "祖先透明"},
		{"offleft", true, "落点在视口之外"},
		{"belowfold", true, "落点在视口之外"},
		{"pointernone", false, "可穿透但仍应放行"},
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

// 移动途中目标被挪走：应当放弃点击，而不是打在移动前算好的坐标上。
const movingTargetHTML = `<body style="margin:0;width:800px;height:600px">
<div id="target" style="position:absolute;left:600px;top:400px;width:96px;height:40px">x</div>
<div id="behind" style="position:absolute;left:600px;top:400px;width:96px;height:40px"></div>
<script>
window.HIT = [];
document.addEventListener('click', e => window.HIT.push(e.target.id), true);
let n = 0;
document.addEventListener('mousemove', () => {
  if (++n === 3) { document.getElementById('target').style.left = '40px'; }
}, true);
</script></body>`

func TestClickAbortsWhenTargetMoves(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	if err != nil {
		t.Skipf("SKIP: 浏览器不可用: %v", err)
	}

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank")
	page.MustWaitLoad()
	page.MustSetDocumentContent(movingTargetHTML)

	err = ClickNoWait(page.MustElement("#target"))
	if err == nil {
		t.Fatal("目标已挪走，点击不应当发出")
	}

	hit := page.MustEval(`() => JSON.stringify(window.HIT)`).Str()
	if hit != "[]" {
		t.Errorf("不该有任何点击落地，实际: %s", hit)
	}
}

// 限制范围后，移动途中的落点不应超出该矩形。
func TestMoveStaysInsideBounds(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	if err != nil {
		t.Skipf("SKIP: 浏览器不可用: %v", err)
	}

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank")
	page.MustWaitLoad()
	page.MustSetDocumentContent(`<body style="margin:0;width:800px;height:600px">
	<script>
	window.PTS = [];
	document.addEventListener('mousemove', e => window.PTS.push([e.clientX, e.clientY]), true);
	</script></body>`)

	bounds := Rect{Left: 100, Top: 100, Right: 700, Bottom: 300}
	if err := page.Mouse.MoveTo(proto.Point{X: 650, Y: 280}); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => { window.PTS = [] }`)

	if err := moveMouseCurvedWithin(page.Mouse, proto.Point{X: 150, Y: 120}, &bounds); err != nil {
		t.Fatal(err)
	}

	var pts [][]float64
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.PTS)`).Str()), &pts); err != nil {
		t.Fatal(err)
	}
	if len(pts) < 5 {
		t.Fatalf("采样点太少: %d", len(pts))
	}
	for _, p := range pts {
		if p[0] < bounds.Left || p[0] > bounds.Right || p[1] < bounds.Top || p[1] > bounds.Bottom {
			t.Errorf("落点 (%.0f,%.0f) 超出限制范围", p[0], p[1])
		}
	}
}
