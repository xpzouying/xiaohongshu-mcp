//go:build integration

// 真实页面探测用例：只填表单、不点「发布」，不产生笔记。
//
// 用途：把小红书编辑器里 @ 的联想容器、候选行、插入后的提及节点结构原样打出来，
// 用于校准 publish.go 里的 mentionItemContainers / mentionItemNameSelectors / mentionNodeQuery。
//
//	go test -tags integration ./xiaohongshu/ -run TestProbeMentionDOM -v -timeout 15m
//
// 可选环境变量：
//
//	XHS_TEST_BROWSER   浏览器路径，不填走 launcher.LookPath() / 常见安装路径
//	XHS_PROBE_HEADLESS 1 = 无头（默认有头，首次需要扫码登录）
//	XHS_TEST_IMAGE     上传用的本地图片，不填用 assets/publish_result.jpeg
//	XHS_AT_PROBE_NAME  要 @ 的昵称；不填则从联想列表里自动挑第一个
//	XHS_PROBE_LOGIN_WAIT 等待扫码的秒数，默认 300
//	XHS_TEST_SHOT      结束时把编辑器截图存到这个路径
package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// ---------- 浏览器 ----------

func probeBrowserPath(t *testing.T) string {
	t.Helper()
	if bin := strings.TrimSpace(os.Getenv("XHS_TEST_BROWSER")); bin != "" {
		return bin
	}
	for _, p := range []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if bin, ok := launcher.LookPath(); ok {
		return bin
	}
	t.Skip("未找到本地浏览器，请用 XHS_TEST_BROWSER 指定 chrome.exe 路径")
	return ""
}

func launchProbeBrowser(t *testing.T, headless bool) *rod.Browser {
	t.Helper()

	l := launcher.New().
		Bin(probeBrowserPath(t)).
		Headless(headless).
		// Windows Defender 会拦 rod 释放的 leakless.exe，本地起浏览器统一关掉。
		Leakless(false).
		Set("window-size", "1440,900").
		Set("lang", "zh-CN")

	controlURL, err := l.Launch()
	require.NoError(t, err, "启动浏览器失败")

	b := rod.New().ControlURL(controlURL).NoDefaultDevice()
	require.NoError(t, b.Connect())

	t.Cleanup(func() {
		_ = b.Close()
		l.Cleanup()
	})
	return b
}

// ---------- cookie ----------

type probeCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// loadProbeCookies 把生产用的 cookies 文件注入页面，复用已有登录态。
func loadProbeCookies(page *rod.Page) {
	path := cookies.GetCookiesFilePath()
	c := cookies.NewLoadCookie(path)
	raw, err := c.LoadCookies()
	if err != nil {
		return
	}

	var list []probeCookie
	if err := json.Unmarshal(raw, &list); err != nil {
		return
	}
	params := make([]*proto.NetworkCookieParam, 0, len(list))
	for _, ck := range list {
		p := &proto.NetworkCookieParam{
			Name:     ck.Name,
			Value:    ck.Value,
			Domain:   ck.Domain,
			Path:     ck.Path,
			HTTPOnly: ck.HTTPOnly,
			Secure:   ck.Secure,
		}
		if ck.Expires > 0 {
			p.Expires = proto.TimeSinceEpoch(ck.Expires)
		}
		switch ck.SameSite {
		case "Strict":
			p.SameSite = proto.NetworkCookieSameSiteStrict
		case "Lax":
			p.SameSite = proto.NetworkCookieSameSiteLax
		case "None":
			p.SameSite = proto.NetworkCookieSameSiteNone
		}
		params = append(params, p)
	}
	if len(params) > 0 {
		_ = page.SetCookies(params)
	}
}

func saveProbeCookies(t *testing.T, b *rod.Browser) {
	t.Helper()
	cks, err := b.GetCookies()
	if err != nil {
		t.Logf("读取浏览器 cookies 失败: %v", err)
		return
	}
	data, err := json.Marshal(cks)
	if err != nil {
		t.Logf("序列化 cookies 失败: %v", err)
		return
	}
	path := cookies.GetCookiesFilePath()
	if err := cookies.NewLoadCookie(path).SaveCookies(data); err != nil {
		t.Logf("写入 cookies 失败: %v", err)
		return
	}
	abs, _ := filepath.Abs(path)
	t.Logf("登录态已保存: %s（后续运行直接复用）", abs)
}

// ---------- 登录 ----------

func probeEnsureLogin(t *testing.T, page *rod.Page, outDir string) {
	t.Helper()

	check := func() bool {
		if err := page.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
			return false
		}
		_ = page.WaitLoad()
		time.Sleep(2 * time.Second)
		ok, _, err := page.Has(`.main-container .user .link-wrapper .channel`)
		return err == nil && ok
	}

	if check() {
		t.Log("登录态有效，跳过扫码")
		return
	}

	shot := filepath.Join(outDir, "01-login-qrcode.png")
	if bin, err := page.Screenshot(false, nil); err == nil {
		_ = os.WriteFile(shot, bin, 0o644)
		t.Logf("登录二维码截图: %s", shot)
	}

	wait := 300
	if v := strings.TrimSpace(os.Getenv("XHS_PROBE_LOGIN_WAIT")); v != "" {
		if n, err := time.ParseDuration(v + "s"); err == nil {
			wait = int(n.Seconds())
		}
	}
	t.Logf("未登录：请在打开的浏览器窗口里扫码，最多等待 %d 秒 ...", wait)

	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for time.Now().Before(deadline) {
		if ok, _, _ := page.Has(`.main-container .user .link-wrapper .channel`); ok {
			t.Log("扫码登录成功")
			time.Sleep(2 * time.Second)
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("等待扫码超时（%d 秒），未取得登录态", wait)
}

// ---------- DOM dump ----------

// jsSnapshot 记录当前所有节点，用于后续只挑「新出现」的节点。
const jsSnapshot = `() => { window.__probeSnap = new Set(Array.from(document.querySelectorAll('*'))); return window.__probeSnap.size }`

const jsEditorHTML = `() => {
	const el = document.querySelector('div[role="textbox"][contenteditable="true"]')
		|| document.querySelector('div.tiptap[contenteditable="true"]')
		|| document.querySelector('.ql-editor')
		|| document.querySelector('div[contenteditable="true"]');
	return el ? el.innerHTML : '';
}`

// jsDumpFresh 把「输入之后新出现」的节点连同层级路径打出来。
const jsDumpFresh = `() => {
	const before = window.__probeSnap || new Set();
	const vis = el => { const r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; };
	const fresh = Array.from(document.querySelectorAll('*')).filter(el => !before.has(el) && vis(el));
	const path = el => { const p = []; let n = el; for (let i = 0; i < 4 && n; i++, n = n.parentElement) {
		p.unshift(n.tagName.toLowerCase() + (n.id ? '#' + n.id : '') +
			(typeof n.className === 'string' && n.className.trim() ? '.' + n.className.trim().split(/\s+/).join('.') : ''));
	} return p.join(' > '); };
	const info = el => ({
		tag: el.tagName.toLowerCase(),
		id: el.id,
		cls: typeof el.className === 'string' ? el.className : '',
		role: el.getAttribute('role') || '',
		attrs: Array.from(el.attributes).map(a => a.name + '="' + a.value + '"').join(' '),
		rect: (r => [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)])(el.getBoundingClientRect()),
		text: (el.innerText || '').trim().slice(0, 100),
	});
	// 最外层的新增节点：父节点是旧节点，避免把整棵子树都倒出来
	const tops = fresh.filter(el => !(el.parentElement && !before.has(el.parentElement)));
	return JSON.stringify({
		freshCount: fresh.length,
		paths: Array.from(new Set(fresh.map(path))),
		tops: tops.map(el => Object.assign(info(el), { html: el.outerHTML.slice(0, 4000) })),
		editorHTML: (document.querySelector('div[role="textbox"][contenteditable="true"]')
			|| document.querySelector('div.tiptap[contenteditable="true"]')
			|| document.querySelector('div[contenteditable="true"]') || {}).innerHTML || '',
	}, null, 1);
}`

// jsPickName 从新出现的联想列表里猜一个昵称：取最内层、有文字、不是纯数字/粉丝数的节点首行。
const jsPickName = `() => {
	const before = window.__probeSnap || new Set();
	const vis = el => { const r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; };
	const fresh = Array.from(document.querySelectorAll('*')).filter(el => !before.has(el) && vis(el));
	const leaves = fresh.filter(el => (el.innerText || '').trim() && !Array.from(el.children).some(c => (c.innerText || '').trim()));
	for (const el of leaves) {
		const t = (el.innerText || '').trim().split('\n')[0].trim();
		if (!t || t.startsWith('@') || t.length > 30) continue;
		if (/^\d+([\u4e07]?粉丝|个关注|关注|粉丝)$/.test(t)) continue;
		return t;
	}
	return '';
}`

// jsListCandidates 列出当前联想框里的候选昵称与候选数量。
const jsListCandidates = `() => {
	const items = Array.from(document.querySelectorAll('#creator-editor-at-container .item'));
	return JSON.stringify({
		count: items.length,
		names: items.map(el => (el.innerText || '').trim().split('\n')[0].trim()),
	});
}`

// jsInsertedMentions 读出编辑器里已插入的提及节点（昵称 + userId）。
const jsInsertedMentions = `() => {
	return JSON.stringify(Array.from(document.querySelectorAll('a.tiptap-at')).map(el => {
		let u = {};
		try { u = JSON.parse(el.getAttribute('data-user') || '{}'); } catch (e) {}
		return {text: (el.innerText || '').trim(), nickname: u.nickname || '', userId: u.userId || ''};
	}));
}`

// writeProbeDump 落盘并把关键信息打到日志，方便直接对比。
func writeProbeDump(t *testing.T, outDir, name, content string) string {
	t.Helper()
	path := filepath.Join(outDir, name+".json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Logf("写 dump 失败 %s: %v", path, err)
		return ""
	}
	t.Logf("DONE %s", path)
	return path
}

// ---------- 用例 ----------

// jsSuggestionState 联想框当前状态：弹层在不在、几个候选、分别是谁。
const jsSuggestionState = `() => {
	const box = document.getElementById('creator-editor-at-container');
	const items = box ? Array.from(box.querySelectorAll('.item')) : [];
	return JSON.stringify({
		hasPopup: !!box,
		count: items.length,
		names: items.map(el => (el.innerText || '').trim().split('\n')[0].trim()),
	});
}`

// TestProbeUserIDSuggestion 对照实验：确认 @ 之后出现的候选是「按输入搜的」还是「默认/最近列表」。
// 关键对照项是那串不存在的字符——它也能出候选，就说明候选与输入无关。
func TestProbeUserIDSuggestion(t *testing.T) {
	headless := strings.TrimSpace(os.Getenv("XHS_PROBE_HEADLESS")) == "1"

	outDir := filepath.Join("..", ".workbuddy", "probe", time.Now().Format("20060102-150405")+"-uid")
	require.NoError(t, os.MkdirAll(outDir, 0o755))
	absOut, _ := filepath.Abs(outDir)
	t.Logf("产物目录: %s", absOut)

	image := filepath.Join("..", "assets", "publish_result.jpeg")
	absImage, err := filepath.Abs(image)
	require.NoError(t, err)

	b := launchProbeBrowser(t, headless)
	page := b.MustPage().Timeout(10 * time.Minute)
	defer page.Close()

	loadProbeCookies(page)
	probeEnsureLogin(t, page, outDir)

	action, err := NewPublishImageAction(page)
	require.NoError(t, err)
	require.NoError(t, uploadImages(action.page, []string{absImage}))

	titleElem, err := action.page.Element("div.d-input input")
	require.NoError(t, err)
	require.NoError(t, humanize.Type(context.Background(), titleElem, "probe-uid"))

	contentElem, err := getContentElement(action.page, contentElemTimeout)
	require.NoError(t, err)
	require.NoError(t, humanize.Type(context.Background(), contentElem, "对照实验。"))
	closeFeatureGuide(action.page)
	tryRefocusTitle(action.page)

	ctx := context.Background()
	require.NoError(t, startEditorTail(contentElem))

	backspace := func(n int) {
		for i := 0; i < n; i++ {
			ka, err := contentElem.KeyActions()
			require.NoError(t, err)
			require.NoError(t, ka.Press(input.Backspace).Do())
			time.Sleep(20 * time.Millisecond)
		}
	}

	// user_id 当查询词的对照实验：默认用占位 id；要验证真实用户时用
	// XHS_AT_PROBE_USER_ID=<24位id> 传入，避免把任何真实账号写进代码。
	target := strings.TrimSpace(os.Getenv("XHS_AT_PROBE_USER_ID"))
	if target == "" {
		target = "0123456789abcdef01234567"
	}
	cases := []struct {
		label string
		text  string
		wait  time.Duration
	}{
		{"只打@", "@", 1500 * time.Millisecond},
		{"乱码", "@zzq9x7v1notexist", 1500 * time.Millisecond},
		{"userid", "@" + target, 1500 * time.Millisecond},
		{"userid久等", "@" + target, 4 * time.Second},
	}

	for i, tc := range cases {
		require.NoError(t, humanize.Type(ctx, contentElem, tc.text))
		time.Sleep(tc.wait)
		state := page.MustEval(jsSuggestionState).String()
		t.Logf("%-10s 输入=%-32q 联想框=%s", tc.label, tc.text, state)
		writeProbeDump(t, outDir, fmt.Sprintf("uid-%d-%s", i+1, tc.label), state)

		backspace(len([]rune(tc.text)))
		time.Sleep(400 * time.Millisecond)
	}

	t.Logf("结论看两处：乱码是否也有候选；user_id 的候选是否与乱码时一致")
	t.Logf("产物目录: %s", absOut)
	fmt.Printf("PROBE_OUT_DIR=%s\n", absOut)
}

func TestProbeMentionDOM(t *testing.T) {
	headless := strings.TrimSpace(os.Getenv("XHS_PROBE_HEADLESS")) == "1"

	outDir := filepath.Join("..", ".workbuddy", "probe", time.Now().Format("20060102-150405"))
	require.NoError(t, os.MkdirAll(outDir, 0o755))
	absOut, _ := filepath.Abs(outDir)
	t.Logf("本次探测产物目录: %s", absOut)

	image := strings.TrimSpace(os.Getenv("XHS_TEST_IMAGE"))
	if image == "" {
		image = filepath.Join("..", "assets", "publish_result.jpeg")
	}
	absImage, err := filepath.Abs(image)
	require.NoError(t, err)
	_, statErr := os.Stat(absImage)
	require.NoError(t, statErr, "图片不存在: %s", absImage)

	b := launchProbeBrowser(t, headless)

	page := b.MustPage().Timeout(15 * time.Minute)
	defer page.Close()

	loadProbeCookies(page)
	probeEnsureLogin(t, page, outDir)
	saveProbeCookies(t, b)

	// 1) 进入发布页并上传一张图（不点发布）
	action, err := NewPublishImageAction(page)
	require.NoError(t, err, "打开发布页失败")

	require.NoError(t, uploadImages(action.page, []string{absImage}), "上传图片失败")

	titleElem, err := action.page.Element("div.d-input input")
	require.NoError(t, err)
	require.NoError(t, humanize.Type(context.Background(), titleElem, "probe-at"))

	contentElem, err := getContentElement(action.page, contentElemTimeout)
	require.NoError(t, err)
	require.NoError(t, humanize.Type(context.Background(), contentElem, "探测@提及结构，不做发布。"))
	closeFeatureGuide(action.page)
	tryRefocusTitle(action.page)

	ctx := context.Background()

	// 2) 光标移到正文末尾新段落，打一个 @，把联想框的 DOM 抓下来
	require.NoError(t, startEditorTail(contentElem))
	time.Sleep(500 * time.Millisecond)

	n := page.MustEval(jsSnapshot).Int()
	t.Logf("基线节点数: %d", n)

	require.NoError(t, humanize.Type(ctx, contentElem, "@"))
	time.Sleep(1500 * time.Millisecond)
	writeProbeDump(t, outDir, "02-after-at", page.MustEval(jsDumpFresh).String())

	// 3) 输入昵称，抓过滤后的候选列表。
	// XHS_AT_PROBE_NAME 支持 "昵称" 与 "昵称|user_id" 两种写法（与 at_users 一致）。
	raw := strings.TrimSpace(os.Getenv("XHS_AT_PROBE_NAME"))
	if raw == "" {
		raw = strings.TrimSpace(page.MustEval(jsPickName).String())
	}
	target := parseAtTarget(raw)
	if raw == "" && target.Name != "" {
		raw = target.Name
	}
	require.NotEmpty(t, target.Name, "没能确定要 @ 的昵称，请用 XHS_AT_PROBE_NAME 指定（支持 昵称|user_id）")
	t.Logf("本次要 @ 的目标: 昵称=%q user_id=%q", target.Name, target.UserID)

	require.NoError(t, humanize.Type(ctx, contentElem, target.Name))
	time.Sleep(1500 * time.Millisecond)
	writeProbeDump(t, outDir, "03-after-name", page.MustEval(jsDumpFresh).String())
	t.Logf("输入 @%s 后的候选: %s", target.Name, page.MustEval(jsListCandidates).String())

	// 4) 模拟你手动的流程：回车选中，看插入后的提及节点长什么样
	ka, err := contentElem.KeyActions()
	require.NoError(t, err)
	require.NoError(t, ka.Press(input.Enter).Do())
	time.Sleep(1200 * time.Millisecond)
	writeProbeDump(t, outDir, "04-after-enter", page.MustEval(jsDumpFresh).String())
	t.Logf("回车后编辑器 HTML:\n%s", page.MustEval(jsEditorHTML).String())

	// 5) 再跑一次线上实现（点击选中），验证它到底有没有命中
	beforeEnter := countMentionNodes(contentElem)
	require.NoError(t, startEditorTail(contentElem))
	err = inputMentions(ctx, contentElem, []string{raw})
	afterEnter := countMentionNodes(contentElem)
	t.Logf("inputMentions 返回: %v；mentionNodeQuery 命中数 %d -> %d", err, beforeEnter, afterEnter)
	time.Sleep(1200 * time.Millisecond)
	writeProbeDump(t, outDir, "05-after-inputMentions", page.MustEval(jsDumpFresh).String())
	inserted := page.MustEval(jsInsertedMentions).String()
	t.Logf("已插入的提及节点: %s", inserted)
	if target.UserID != "" {
		require.Contains(t, inserted, target.UserID,
			"插入节点的 data-user.userId 与请求的 user_id 不一致")
		t.Logf("user_id 校验通过: %s", target.UserID)
	}

	// 6) 只用 user_id 走一遍正式实现：@ + id 搜索 → 取第一个候选 → 校验 userId
	userID := strings.TrimSpace(os.Getenv("XHS_AT_PROBE_USER_ID"))
	if userID != "" {
		require.NoError(t, startEditorTail(contentElem))
		err := inputMentions(ctx, contentElem, []string{userID})
		t.Logf("仅用 user_id 的 inputMentions 返回: %v", err)
		time.Sleep(1500 * time.Millisecond)
		inserted := page.MustEval(jsInsertedMentions).String()
		t.Logf("仅用 user_id 插入后的提及节点: %s", inserted)
		writeProbeDump(t, outDir, "06-after-userid", page.MustEval(jsDumpFresh).String())
		require.NoError(t, err, "只给 user_id 也应能 @ 到人")
		require.Contains(t, inserted, userID, "插入节点的 userId 与请求不一致")
		t.Logf("仅用 user_id 的校验通过: %s", userID)
	}

	// 7) 留档截图
	shot := strings.TrimSpace(os.Getenv("XHS_TEST_SHOT"))
	if shot == "" {
		shot = filepath.Join(outDir, "08-editor.png")
	}
	if bin, err := page.Screenshot(false, nil); err == nil {
		_ = os.WriteFile(shot, bin, 0o644)
		t.Logf("截图: %s", shot)
	}
	t.Logf("全部产物目录: %s", absOut)
	fmt.Printf("PROBE_OUT_DIR=%s\n", absOut)
}
