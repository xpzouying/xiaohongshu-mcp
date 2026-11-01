package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// Board 表示用户收藏的专辑（小红书 board）
type Board struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NoteCount int    `json:"note_count"`
}

// BoardAction 负责专辑相关交互
type BoardAction struct {
	page *rod.Page
}

func NewBoardAction(page *rod.Page) *BoardAction {
	return &BoardAction{page: page.Timeout(60 * time.Second)}
}

const boardScrollRounds = 30

// ===== 读：走 __INITIAL_STATE__，与仓库其它模块一致 =====

// gotoPage 导航并等页面加载完
func gotoPage(page *rod.Page, url string) error {
	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("打开页面失败: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("等待页面加载失败: %w", err)
	}
	return nil
}

// evalString 执行只读脚本并取字符串结果
func evalString(page *rod.Page, js string, args ...interface{}) (string, error) {
	obj, err := page.Eval(js, args...)
	if err != nil {
		return "", fmt.Errorf("读取页面状态失败: %w", err)
	}
	return obj.Value.Str(), nil
}

// waitLoggedInUserID 等到页面注水出真实登录态，返回当前登录用户 id。
//
// 页面刚打开的几秒里状态里还是游客，这时候拿到的 user_id 是游客的，
// 用它去查专辑会"成功"返回 0 个专辑 —— 静默查错账号，所以必须等。
func waitLoggedInUserID(page *rod.Page) (string, error) {
	const js = `() => {
		const unwrap = (v) => (v && typeof v === 'object' && 'value' in v) ? v.value : v;
		const u = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user;
		if (!u) return "";
		if (!unwrap(u.loggedIn)) return "";
		const info = unwrap(u.userInfo);
		if (!info) return "";
		return info.userId || info.user_id || "";
	}`

	deadline := time.Now().Add(30 * time.Second)
	for {
		uid, err := evalString(page, js)
		if err == nil && uid != "" {
			return uid, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("未检测到登录态，请先登录并确认 cookies 有效")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ListBoards 获取用户的收藏专辑列表。
// 走 收藏→专辑 页面的 __INITIAL_STATE__.board.userBoardList，一次拿全，不用翻页。
func (a *BoardAction) ListBoards(ctx context.Context, userID string) ([]Board, error) {
	page := a.page.Context(ctx)

	if err := gotoPage(page, "https://www.xiaohongshu.com/explore"); err != nil {
		return nil, err
	}
	loginUserID, err := waitLoggedInUserID(page)
	if err != nil {
		return nil, err
	}
	if userID == "" {
		userID = loginUserID
	}

	url := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?tab=fav&subTab=board", userID)
	if err := gotoPage(page, url); err != nil {
		return nil, err
	}

	const js = `() => {
		const unwrap = (v) => (v && typeof v === 'object' && 'value' in v) ? v.value : v;
		const b = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.board;
		if (!b) return "";
		const list = unwrap(b.userBoardList);
		if (!Array.isArray(list)) return "";
		return JSON.stringify(list.map((x) => ({ id: x.id, name: x.name, total: x.total })));
	}`

	var raw string
	deadline := time.Now().Add(30 * time.Second)
	for {
		raw, err = evalString(page, js)
		if err == nil && raw != "" {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("页面状态里没有专辑列表，可能未登录或页面结构已变化")
		}
		time.Sleep(500 * time.Millisecond)
	}

	var list []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("解析专辑列表失败: %w", err)
	}

	boards := make([]Board, 0, len(list))
	for _, b := range list {
		boards = append(boards, Board{ID: b.ID, Name: b.Name, NoteCount: b.Total})
	}

	logrus.Infof("获取专辑列表成功，共 %d 个专辑", len(boards))
	return boards, nil
}

// noteInBoard 查专辑里有没有这条笔记，用来确认移动是否真的生效。
// 读专辑页的 __INITIAL_STATE__.board.boardFeedsMap；笔记多时滚动加载。
func noteInBoard(page *rod.Page, boardID, noteID string) (bool, error) {
	url := fmt.Sprintf("https://www.xiaohongshu.com/board/%s", boardID)
	if err := gotoPage(page, url); err != nil {
		return false, err
	}

	const js = `(boardId) => {
		const unwrap = (v) => (v && typeof v === 'object' && 'value' in v) ? v.value : v;
		const b = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.board;
		if (!b) return "";
		const map = unwrap(b.boardFeedsMap);
		if (!map) return "";
		const entry = unwrap(map[boardId]);
		if (!entry) return "";
		return JSON.stringify({
			hasMore: !!entry.hasMore,
			notes: (entry.notes || []).map((n) => n.noteId || n.id),
		});
	}`

	deadline := time.Now().Add(30 * time.Second)
	for round := 0; round < boardScrollRounds; round++ {
		raw, err := evalString(page, js, boardID)
		if err != nil {
			return false, err
		}
		if raw == "" {
			if time.Now().After(deadline) {
				return false, fmt.Errorf("页面状态里没有专辑 %s 的笔记列表", boardID)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var feeds struct {
			HasMore bool     `json:"hasMore"`
			Notes   []string `json:"notes"`
		}
		if err := json.Unmarshal([]byte(raw), &feeds); err != nil {
			return false, fmt.Errorf("解析专辑笔记列表失败: %w", err)
		}
		for _, id := range feeds.Notes {
			if id == noteID {
				return true, nil
			}
		}
		if !feeds.HasMore {
			return false, nil
		}

		// 还有更多，滚动加载（go-rod 原生滚动，不注入 JS）
		if err := page.Mouse.Scroll(0, 900, 3); err != nil {
			return false, fmt.Errorf("滚动专辑页失败: %w", err)
		}
		time.Sleep(700 * time.Millisecond)
	}
	return false, nil
}

// ===== 写：必须带签名，只能借页面自己的 API 函数 =====

// 专辑的增删改接口需要 x-s / x-t / x-s-common 签名，签名由小红书自己的 axios
// 拦截器生成，Go 侧没法复现（手写请求会被拒："当前账号存在异常"）。
//
// 这里不去操作页面元素，只从 webpack 模块表里按导出函数名取出站点自己的 API
// 函数来调用，签名自动带上。按函数名查找，不写死 module id（发版会变）。
const boardWriteAPIJS = `
function () {
  if (window.__xhsBoardApi) return true;
  var key = Object.keys(window).filter(function (k) { return /^webpackChunk/.test(k); })[0];
  if (!key || !Array.isArray(window[key])) return false;

  var req = null;
  try { window[key].push([[Symbol('xhs-mcp')], {}, function (r) { req = r; }]); } catch (e) { return false; }
  if (!req || !req.c) return false;

  for (var id in req.c) {
    var exp;
    try { exp = req.c[id] && req.c[id].exports; } catch (e) { continue; }
    if (!exp || typeof exp !== 'object') continue;

    var names = {};
    try {
      Object.keys(exp).forEach(function (k) {
        var v = exp[k];
        if (typeof v === 'function' && v.name) names[v.name] = k;
      });
    } catch (e) { continue; }

    if (names.postApiSnsWebV1NoteMove && names.postApiSnsWebV1Board) {
      window.__xhsBoardApi = function (fn, arg) { return exp[names[fn]](arg); };
      return true;
    }
  }
  return false;
}`

// jsEnvelope 写接口脚本统一返回的信封，避免接口报错时 rod 直接 panic
type jsEnvelope struct {
	OK   bool            `json:"ok"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// 页面刚加载时风控 SDK 还没就绪，签名头不完整，接口会短暂报错，退避重试即可。
// "薯队长遇到了点小麻烦"是小红书的通用兜底错误，也归到这类。
var warmingUpErrs = []string{"没有权限访问", "账号存在异常", "薯队长"}

func isWarmingUpErr(msg string) bool {
	for _, s := range warmingUpErrs {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// ensureWriteAPI 确保页面在小红书域名下且已解析出可用的 API 函数
func ensureWriteAPI(page *rod.Page) error {
	info, err := page.Info()
	if err != nil {
		return fmt.Errorf("读取页面信息失败: %w", err)
	}
	if !strings.Contains(info.URL, "xiaohongshu.com") {
		if err := gotoPage(page, "https://www.xiaohongshu.com/explore"); err != nil {
			return err
		}
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		obj, err := page.Eval(boardWriteAPIJS)
		if err == nil && obj.Value.Bool() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("页面未加载小红书 API 模块，请确认页面已加载完成")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// callWriteAPI 调用页面自己的接口函数，返回 data 部分
func callWriteAPI(page *rod.Page, js string) (json.RawMessage, error) {
	var lastMsg string

	for attempt := 0; attempt < 5; attempt++ {
		obj, err := page.Eval(js)
		if err != nil {
			return nil, fmt.Errorf("执行页面脚本失败: %w", err)
		}

		var env jsEnvelope
		if err := json.Unmarshal([]byte(obj.Value.Str()), &env); err != nil {
			return nil, fmt.Errorf("解析页面返回失败: %w", err)
		}
		if env.OK {
			return env.Data, nil
		}

		lastMsg = env.Msg
		if !isWarmingUpErr(env.Msg) {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}

	return nil, fmt.Errorf("小红书接口报错: %s", lastMsg)
}

// callNoteMove 调 /api/sns/web/v1/note/move。
// sourceBoardID 为笔记当前所在专辑，留空表示笔记还没归过类；
// targetBoardID 留空表示把笔记移出专辑（仍保留收藏）。两个方向都不影响收藏时间。
func callNoteMove(page *rod.Page, noteID, targetBoardID, sourceBoardID string) error {
	js := fmt.Sprintf(`async () => {
		try {
			const body = { targetBoardId: %s, notesId: %s };
			const src = %s;
			if (src) body.sourceBoardId = src;
			const r = await window.__xhsBoardApi('postApiSnsWebV1NoteMove', body);
			return JSON.stringify({ ok: true, data: r || {} });
		} catch (e) {
			return JSON.stringify({ ok: false, msg: (e && (e.msg || e.message)) || String(e) });
		}
	}`, strconv.Quote(targetBoardID), strconv.Quote(noteID), strconv.Quote(sourceBoardID))

	_, err := callWriteAPI(page, js)
	return err
}

// findNoteBoard 找出笔记当前在哪个专辑里，找不到返回空串。
// 要逐个专辑翻页，比较慢；调用方知道原专辑时应直接传 sourceBoardID 避免走这里。
func (a *BoardAction) findNoteBoard(ctx context.Context, page *rod.Page, noteID string) (string, error) {
	boards, err := a.ListBoards(ctx, "")
	if err != nil {
		return "", err
	}
	for _, b := range boards {
		if b.NoteCount == 0 {
			continue
		}
		in, err := noteInBoard(page, b.ID, noteID)
		if err != nil {
			return "", err
		}
		if in {
			return b.ID, nil
		}
	}
	return "", nil
}

// MoveNoteToBoard 把已收藏的笔记归入专辑，收藏时间不变。
//
// sourceBoardID 是笔记当前所在专辑，不知道就留空：
// note/move 不带 source_board_id 时只对"还没归过专辑"的笔记生效，对已归类的
// 笔记服务端照样返回 success=true 但笔记不会动，所以这里会回读校验，没生效再
// 自动找出原专辑重试（较慢）。调用方知道原专辑时直接传进来可以省掉查找。
func (a *BoardAction) MoveNoteToBoard(ctx context.Context, noteID, targetBoardID, sourceBoardID string) error {
	if noteID == "" || targetBoardID == "" {
		return fmt.Errorf("note_id 和 target_board_id 都不能为空")
	}

	page := a.page.Context(ctx)
	if err := ensureWriteAPI(page); err != nil {
		return err
	}

	if err := callNoteMove(page, noteID, targetBoardID, sourceBoardID); err != nil {
		return err
	}

	in, err := noteInBoard(page, targetBoardID, noteID)
	if err != nil {
		return err
	}
	if in {
		logrus.Infof("成功将笔记 %s 移动到专辑 %s（收藏时间未变）", noteID, targetBoardID)
		return nil
	}
	if sourceBoardID != "" {
		return fmt.Errorf("移动笔记 %s 到专辑 %s 未生效", noteID, targetBoardID)
	}

	// 没生效说明笔记已经在别的专辑里，找出原专辑再移一次
	found, err := a.findNoteBoard(ctx, page, noteID)
	if err != nil {
		return err
	}
	if found == "" {
		return fmt.Errorf("移动笔记 %s 失败，且未找到它当前所在的专辑（可能未收藏）", noteID)
	}

	if err := ensureWriteAPI(page); err != nil {
		return err
	}
	if err := callNoteMove(page, noteID, targetBoardID, found); err != nil {
		return err
	}

	in, err = noteInBoard(page, targetBoardID, noteID)
	if err != nil {
		return err
	}
	if !in {
		return fmt.Errorf("移动笔记 %s 到专辑 %s 未生效", noteID, targetBoardID)
	}

	logrus.Infof("成功将笔记 %s 从专辑 %s 移动到 %s（收藏时间未变）", noteID, found, targetBoardID)
	return nil
}

// RemoveNoteFromBoard 把笔记移出专辑但保留收藏，收藏时间不变。
// sourceBoardID 留空时自动查找笔记当前所在的专辑（较慢）。
func (a *BoardAction) RemoveNoteFromBoard(ctx context.Context, noteID, sourceBoardID string) error {
	if noteID == "" {
		return fmt.Errorf("note_id 不能为空")
	}

	page := a.page.Context(ctx)

	if sourceBoardID == "" {
		found, err := a.findNoteBoard(ctx, page, noteID)
		if err != nil {
			return err
		}
		if found == "" {
			return fmt.Errorf("笔记 %s 不在任何专辑里", noteID)
		}
		sourceBoardID = found
	}

	if err := ensureWriteAPI(page); err != nil {
		return err
	}
	// target 留空 = 移出专辑
	if err := callNoteMove(page, noteID, "", sourceBoardID); err != nil {
		return err
	}

	in, err := noteInBoard(page, sourceBoardID, noteID)
	if err != nil {
		return err
	}
	if in {
		return fmt.Errorf("笔记 %s 仍在专辑 %s 中，移出未生效", noteID, sourceBoardID)
	}

	logrus.Infof("成功将笔记 %s 移出专辑 %s（仍保留收藏，收藏时间未变）", noteID, sourceBoardID)
	return nil
}

// CreateBoard 新建一个空专辑。private 对应网页端"公开专辑"开关关闭。
// 网页端限制专辑名不超过 12 个字符。
func (a *BoardAction) CreateBoard(ctx context.Context, name string, private bool) (*Board, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("专辑名不能为空")
	}
	if len([]rune(name)) > 12 {
		return nil, fmt.Errorf("专辑名不能超过 12 个字符")
	}

	page := a.page.Context(ctx)
	if err := ensureWriteAPI(page); err != nil {
		return nil, err
	}

	privacy := 0
	if private {
		privacy = 1
	}

	js := fmt.Sprintf(`async () => {
		try {
			const r = await window.__xhsBoardApi('postApiSnsWebV1Board', { name: %s, privacy: %d });
			return JSON.stringify({ ok: true, data: r || {} });
		} catch (e) {
			return JSON.stringify({ ok: false, msg: (e && (e.msg || e.message)) || String(e) });
		}
	}`, strconv.Quote(name), privacy)

	data, err := callWriteAPI(page, js)
	if err != nil {
		return nil, err
	}

	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return nil, fmt.Errorf("解析创建结果失败: %w", err)
	}
	if created.ID == "" {
		return nil, fmt.Errorf("创建专辑失败：未返回专辑 id")
	}

	logrus.Infof("创建专辑成功：%s (%s)", created.Name, created.ID)
	return &Board{ID: created.ID, Name: created.Name}, nil
}

// DeleteBoard 删除专辑。专辑里的笔记不会被取消收藏，只是回到"未归类"状态，
// 收藏时间也不变。
func (a *BoardAction) DeleteBoard(ctx context.Context, boardID string) error {
	if boardID == "" {
		return fmt.Errorf("board_id 不能为空")
	}

	page := a.page.Context(ctx)
	if err := ensureWriteAPI(page); err != nil {
		return err
	}

	js := fmt.Sprintf(`async () => {
		try {
			const r = await window.__xhsBoardApi('deleteApiSnsWebV1Board', { params: { board_id: %s } });
			return JSON.stringify({ ok: true, data: r || {} });
		} catch (e) {
			return JSON.stringify({ ok: false, msg: (e && (e.msg || e.message)) || String(e) });
		}
	}`, strconv.Quote(boardID))

	if _, err := callWriteAPI(page, js); err != nil {
		return err
	}

	logrus.Infof("删除专辑成功：%s", boardID)
	return nil
}
