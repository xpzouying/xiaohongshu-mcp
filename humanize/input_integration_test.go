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
//
// 这两条一起把输入的事件序列钉死：任何人把 Type 换成一次性写入、或换成
// elem.Input（它在 CDP 插入之外还会多派发一轮 input/change），这里都会红。
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

// TestClickPressAndScatter 校验 Click 的两条约定：
//   - 按下与抬起之间有停留（rod 原生 Mouse.Click 是 Down 紧接 Up，没有间隔）
//   - 同一元素多次点击的落点不固定（rod 返回的可点位置是常量）
func TestClickPressAndScatter(t *testing.T) {
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
