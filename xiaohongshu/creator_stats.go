package xiaohongshu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// 创作者中心「数据看板」各页面，以及页面加载时自己会请求的接口。
// 只监听这些接口的响应，不构造请求，签名由页面自己处理。
const (
	urlCreatorAccount  = "https://creator.xiaohongshu.com/statistics/account"
	urlCreatorAnalysis = "https://creator.xiaohongshu.com/statistics/data-analysis"
	urlCreatorFans     = "https://creator.xiaohongshu.com/statistics/fans-data"
	urlCreatorNote     = "https://creator.xiaohongshu.com/statistics/note-detail?noteId="

	apiAccountBase   = "/api/galaxy/v2/creator/datacenter/account/base"
	apiAccountSource = "/api/galaxy/v2/creator/datacenter/audience/source/account"
	apiNoteList      = "/api/galaxy/creator/datacenter/note/analyze/list"
	apiFansOverall   = "/api/galaxy/creator/data/fans/overall_new"
	apiActiveFans    = "/api/galaxy/creator/data/active_fans_new"
	apiNoteBase      = "/api/galaxy/creator/datacenter/note/base"
	apiNoteSource    = "/api/galaxy/creator/datacenter/note/audience/source"
	apiNoteAudience  = "/api/galaxy/creator/datacenter/note/audience/source/detail"

	creatorPageTimeout = 60 * time.Second
)

// 统计时间统一按北京时间展示，与创作者中心一致。
var cst = time.FixedZone("CST", 8*3600)

// CreatorStats 创作者中心数据看板：账号概览、观众来源、近期笔记、粉丝。
// 指标名沿用创作者中心页面上的中文叫法。
type CreatorStats struct {
	Period     string                     `json:"period"`
	Overview   map[string]json.RawMessage `json:"overview"`
	Change     map[string]json.RawMessage `json:"change_percent"` // 环比，页面上显示「-」的不返回
	Daily      []map[string]any           `json:"daily"`
	Sources    map[string]float64         `json:"sources_percent"`
	NotesTotal int                        `json:"notes_total"`
	Notes      []CreatorNoteRow           `json:"notes"`
	Fans       map[string]json.RawMessage `json:"fans"`
	FansDaily  []map[string]any           `json:"fans_daily"`
	ActiveFans []ActiveFan                `json:"active_fans"`
}

// CreatorNoteRow 「内容分析」列表里的一篇笔记。
type CreatorNoteRow struct {
	NoteID         string  `json:"note_id"`
	Title          string  `json:"title"`
	PostTime       string  `json:"post_time"`
	Impressions    int     `json:"曝光"`
	Views          int     `json:"观看"`
	CoverClickRate float64 `json:"封面点击率(%)"`
	Likes          int     `json:"点赞"`
	Comments       int     `json:"评论"`
	Collects       int     `json:"收藏"`
	NewFans        int     `json:"涨粉"`
	Shares         int     `json:"分享"`
	ViewTimeAvg    float64 `json:"人均观看时长(秒)"`
	Danmaku        int     `json:"弹幕"`
}

// ActiveFan 「我的活跃粉丝」里的一位。
type ActiveFan struct {
	UserID       string `json:"user_id"`
	Nickname     string `json:"nickname"`
	Interactions int    `json:"互动次数"`
}

// NoteStats 单篇笔记的「笔记数据详情」。
type NoteStats struct {
	NoteID    string                     `json:"note_id"`
	Title     string                     `json:"title"`
	Type      string                     `json:"type"`
	PostTime  string                     `json:"post_time"`
	Topics    []string                   `json:"topics"`
	UpdatedAt string                     `json:"data_updated_at"`
	Core      map[string]json.RawMessage `json:"core"`
	FansShare map[string]json.RawMessage `json:"fans_share_percent"` // 各项里粉丝贡献的占比
	FansOnly  map[string]json.RawMessage `json:"fans_only"`          // 只算粉丝时的指标
	Compare   []NoteCompare              `json:"compare_with_similar"`
	Daily     []map[string]any           `json:"daily"`
	Hourly    []map[string]any           `json:"hourly"`
	Sources   noteAudience               `json:"sources"`
	Audience  noteAudience               `json:"audience"`
}

// NoteCompare 「笔记诊断」里与同类笔记的对比，观看数不足 100 时为空。
type NoteCompare struct {
	Metric  string  `json:"metric"`
	Value   float64 `json:"value"`
	Median  float64 `json:"median"`
	Summary string  `json:"summary"`
	Level   string  `json:"level"`
}

// noteAudience 观看来源 / 观众画像，结构原样透传；观看数不足时只有提示。
type noteAudience struct {
	NoData   bool            `json:"no_data"`
	Tip      string          `json:"no_data_tip_msg,omitempty"`
	Source   json.RawMessage `json:"source,omitempty"`
	Gender   json.RawMessage `json:"gender,omitempty"`
	Age      json.RawMessage `json:"age,omitempty"`
	City     json.RawMessage `json:"city,omitempty"`
	Interest json.RawMessage `json:"interest,omitempty"`
}

// metric 接口字段到页面叫法的映射；change 是对应环比字段，可空。
type metric struct {
	key, label, change string
}

var accountMetrics = []metric{
	{"impl_count", "曝光", "impl_count_rate"},
	{"view_count", "观看", "view_count_rate"},
	{"cover_click_rate", "封面点击率(%)", "cover_click_cycle_rate"},
	{"avg_view_time", "平均观看时长(秒)", "avg_view_time_rate"},
	{"video_full_view_rate", "视频完播率(%)", "video_full_view_cycle_rate"},
	{"like_count", "点赞", "like_count_rate"},
	{"collect_count", "收藏", "collect_count_rate"},
	{"comment_count", "评论", "comment_count_rate"},
	{"share_count", "分享", "share_count_rate"},
	{"danmaku_count", "弹幕", "danmaku_count_rate"},
	{"home_view_count", "主页访客", "home_view_count_rate"},
	{"net_rise_fans_count", "净涨粉", "net_rise_fans_count_rate"},
	{"rise_fans_count", "新增关注", "rise_fans_count_rate"},
	{"loss_fans_count", "取消关注", "loss_fans_count_rate"},
	{"publish_note_num", "发布笔记", "publish_note_num_rate"},
}

var accountDaily = []metric{
	{key: "impl_count_list", label: "曝光"},
	{key: "view_list", label: "观看"},
	{key: "cover_click_rate_list", label: "封面点击率(%)"},
	{key: "like_list", label: "点赞"},
	{key: "collect_list", label: "收藏"},
	{key: "comment_list", label: "评论"},
	{key: "share_list", label: "分享"},
	{key: "home_view_list", label: "主页访客"},
	{key: "net_rise_fans_count_list", label: "净涨粉"},
}

var fansMetrics = []metric{
	{key: "fans_count", label: "总粉丝数"},
	{key: "rise_fans_count", label: "新增粉丝数"},
	{key: "leave_fans_count", label: "流失粉丝数"},
}

var fansDaily = []metric{
	{key: "fans_list", label: "总粉丝数"},
	{key: "rise_fans_list", label: "新增粉丝数"},
	{key: "leave_fans_list", label: "流失粉丝数"},
}

var noteCore = []metric{
	{key: "impl_count", label: "曝光"},
	{key: "view_count", label: "观看"},
	{key: "cover_click_rate", label: "封面点击率(%)"},
	{key: "view_time_avg", label: "平均观看时长(秒)"},
	{key: "full_view_rate", label: "完播率(%)"},
	{key: "finish5s_rate", label: "5秒完播率(%)"},
	{key: "exit_view2s_rate", label: "2秒退出率(%)"},
	{key: "interaction_rate", label: "互动率(%)"},
	{key: "rise_fans_count", label: "涨粉"},
	{key: "like_count", label: "点赞"},
	{key: "comment_count", label: "评论"},
	{key: "collect_count", label: "收藏"},
	{key: "share_count", label: "分享"},
	{key: "danmaku_count", label: "弹幕"},
}

// 详情页「粉丝占比 xx%」。
var noteFansShare = []metric{
	{key: "impl_count_rate_with_fans", label: "曝光"},
	{key: "view_rate_with_fans", label: "观看"},
	{key: "like_rate_with_fans", label: "点赞"},
	{key: "comment_rate_with_fans", label: "评论"},
	{key: "collect_rate_with_fans", label: "收藏"},
	{key: "share_rate_with_fans", label: "分享"},
}

// 详情页「粉丝 xx」：只算粉丝时的指标值。
var noteFansOnly = []metric{
	{key: "cover_click_rate_with_fans", label: "封面点击率(%)"},
	{key: "view_time_avg_with_fans", label: "平均观看时长(秒)"},
}

var noteSeries = []metric{
	{key: "imp_list", label: "曝光"},
	{key: "view_list", label: "观看"},
	{key: "like_list", label: "点赞"},
	{key: "comment_list", label: "评论"},
	{key: "collect_list", label: "收藏"},
	{key: "share_list", label: "分享"},
	{key: "rise_fans_list", label: "涨粉"},
}

var compareLabels = map[string]string{
	"clickRate":      "封面点击率",
	"fullViewRate":   "完播率",
	"fullView5sRate": "5秒完播率",
	"viewTimeAvg":    "平均观看时长(秒)",
	"interactRate":   "互动率",
	"noteEngage":     "互动数",
}

// CreatorStatsAction 读取创作者中心数据看板。
type CreatorStatsAction struct {
	page *rod.Page
}

func NewCreatorStatsAction(page *rod.Page) *CreatorStatsAction {
	return &CreatorStatsAction{page: page}
}

// Overview 依次打开账号概览、内容分析、粉丝数据三页，汇总近 days 天（7 或 30）的数据。
func (a *CreatorStatsAction) Overview(ctx context.Context, days int) (*CreatorStats, error) {
	period, err := periodKey(days)
	if err != nil {
		return nil, err
	}

	bodies := map[string][]byte{}
	for _, step := range []struct {
		url   string
		paths []string
	}{
		{urlCreatorAccount, []string{apiAccountBase, apiAccountSource}},
		// shortcut: 只取内容分析页默认的第一页（近 30 天发布、最多 10 篇），总数见 notes_total；要更多再做翻页。
		{urlCreatorAnalysis, []string{apiNoteList}},
		{urlCreatorFans, []string{apiFansOverall, apiActiveFans}},
	} {
		got, err := captureAPIs(ctx, a.page, step.url, step.paths...)
		if err != nil {
			return nil, err
		}
		maps.Copy(bodies, got)
	}
	return buildCreatorStats(period, bodies)
}

// Note 打开单篇笔记的数据详情页。
func (a *CreatorStatsAction) Note(ctx context.Context, noteID string) (*NoteStats, error) {
	noteID = strings.TrimSpace(noteID)
	if noteID == "" {
		return nil, fmt.Errorf("note_id 不能为空")
	}
	bodies, err := captureAPIs(ctx, a.page, urlCreatorNote+url.QueryEscape(noteID),
		apiNoteBase, apiNoteSource, apiNoteAudience)
	if err != nil {
		return nil, err
	}
	return buildNoteStats(bodies)
}

func periodKey(days int) (string, error) {
	switch days {
	case 0, 7:
		return "seven", nil
	case 30:
		return "thirty", nil
	}
	return "", fmt.Errorf("days 只支持 7 或 30，收到 %d", days)
}

// captureAPIs 打开 pageURL，收集页面自己请求的指定接口响应（按 path 精确匹配），
// 全部到齐后返回 path → 响应体。
func captureAPIs(ctx context.Context, page *rod.Page, pageURL string, paths ...string) (map[string][]byte, error) {
	// 先用未取消的 page 开启 Network：否则 EachEvent 结束时会用已取消的 ctx 关掉 Network
	// 却记不下状态，同一页面下一次 EachEvent 以为已开启而不再开，收不到任何事件。
	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		return nil, fmt.Errorf("开启网络监听失败: %w", err)
	}
	pg := page.Timeout(creatorPageTimeout)
	p, cancel := pg.WithCancel()

	type response struct {
		path string
		body []byte
	}
	got := make(chan response, 32)
	pending := map[proto.NetworkRequestID]string{}
	wait := p.EachEvent(func(e *proto.NetworkResponseReceived) {
		if path := matchCreatorAPI(e.Response.URL, paths); path != "" && e.Response.Status == 200 {
			pending[e.RequestID] = path
		}
	}, func(e *proto.NetworkLoadingFinished) {
		path, ok := pending[e.RequestID]
		if !ok {
			return
		}
		delete(pending, e.RequestID)
		res, err := (proto.NetworkGetResponseBody{RequestID: e.RequestID}).Call(p)
		if err != nil {
			return
		}
		body := []byte(res.Body)
		if res.Base64Encoded {
			if body, err = base64.StdEncoding.DecodeString(res.Body); err != nil {
				return
			}
		}
		// 不阻塞事件循环：同一接口重复请求时多余的响应直接丢弃。
		select {
		case got <- response{path, body}:
		default:
		}
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		wait()
	}()
	defer func() {
		cancel()
		<-done
	}()

	if err := pg.Navigate(pageURL); err != nil {
		return nil, fmt.Errorf("打开 %s 失败: %w", pageURL, err)
	}

	bodies := make(map[string][]byte, len(paths))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for len(bodies) < len(paths) {
		select {
		case r := <-got:
			bodies[r.path] = r.body
		case <-ticker.C:
			if blocked := CheckPageBlocked(page); blocked != nil {
				return nil, blocked
			}
		case <-pg.GetContext().Done():
			// 创作者中心未登录时停在自己的 /login，而不是主站的 /website-login。
			if onCreatorLogin(page) {
				return nil, errors.ErrLoginRequired
			}
			var missing []string
			for _, path := range paths {
				if _, ok := bodies[path]; !ok {
					missing = append(missing, path)
				}
			}
			return nil, fmt.Errorf("等待创作者中心数据超时（%s），未拿到: %v", pageURL, missing)
		}
	}
	humanize.Delay(ctx, humanize.Reading)
	return bodies, nil
}

func matchCreatorAPI(raw string, paths []string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() != "creator.xiaohongshu.com" || !slices.Contains(paths, u.Path) {
		return ""
	}
	return u.Path
}

func onCreatorLogin(page *rod.Page) bool {
	tree, err := (proto.PageGetFrameTree{}).Call(page.Timeout(3 * time.Second))
	if err != nil || tree.FrameTree == nil || tree.FrameTree.Frame == nil {
		return false
	}
	u, err := url.Parse(tree.FrameTree.Frame.URL)
	return err == nil && u.Hostname() == "creator.xiaohongshu.com" && strings.HasPrefix(u.Path, "/login")
}

// apiData 解开 {success, msg, data} 外壳。
func apiData(bodies map[string][]byte, path string, v any) error {
	var env struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(bodies[path], &env); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	if !env.Success {
		return fmt.Errorf("%s 返回失败: %s", path, env.Msg)
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		return fmt.Errorf("解析 %s 的 data 失败: %w", path, err)
	}
	return nil
}

func buildCreatorStats(period string, bodies map[string][]byte) (*CreatorStats, error) {
	var account, acc map[string]json.RawMessage
	if err := apiData(bodies, apiAccountBase, &account); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(account[period], &acc)

	var sources map[string]json.RawMessage
	if err := apiData(bodies, apiAccountSource, &sources); err != nil {
		return nil, err
	}
	var sourceItems []struct {
		Title string  `json:"title"`
		Value float64 `json:"value"`
	}
	_ = json.Unmarshal(sources[period], &sourceItems)

	var noteList struct {
		Total int `json:"total"`
		Notes []struct {
			ID             string  `json:"id"`
			Title          string  `json:"title"`
			PostTime       int64   `json:"post_time"`
			Impressions    int     `json:"imp_count"`
			Views          int     `json:"read_count"`
			CoverClickRate float64 `json:"coverClickRate"`
			Likes          int     `json:"like_count"`
			Comments       int     `json:"comment_count"`
			Collects       int     `json:"fav_count"`
			NewFans        int     `json:"increase_fans_count"`
			Shares         int     `json:"share_count"`
			ViewTimeAvg    float64 `json:"view_time_avg"`
			Danmaku        int     `json:"danmaku_count"`
		} `json:"note_infos"`
	}
	if err := apiData(bodies, apiNoteList, &noteList); err != nil {
		return nil, err
	}

	var fans map[string]json.RawMessage
	if err := apiData(bodies, apiFansOverall, &fans); err != nil {
		return nil, err
	}
	var fansPeriod map[string]json.RawMessage
	_ = json.Unmarshal(fans[period], &fansPeriod)

	var active map[string]json.RawMessage
	if err := apiData(bodies, apiActiveFans, &active); err != nil {
		return nil, err
	}
	var activeItems []struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
		Count  int    `json:"count"`
	}
	_ = json.Unmarshal(active[period], &activeItems)

	var begin, end int64
	_ = json.Unmarshal(acc["begin_time"], &begin)
	_ = json.Unmarshal(acc["end_time"], &end)

	stats := &CreatorStats{
		Period:     formatMs(begin, time.DateOnly) + " 至 " + formatMs(end, time.DateOnly),
		Overview:   pick(acc, accountMetrics),
		Change:     map[string]json.RawMessage{},
		Daily:      series(acc, accountDaily, time.DateOnly),
		Sources:    map[string]float64{},
		NotesTotal: noteList.Total,
		Notes:      []CreatorNoteRow{},
		Fans:       pick(fansPeriod, fansMetrics),
		FansDaily:  series(fansPeriod, fansDaily, time.DateOnly),
		ActiveFans: []ActiveFan{},
	}
	for _, m := range accountMetrics {
		if string(acc[m.change+"_display"]) == "true" {
			stats.Change[m.label] = acc[m.change]
		}
	}
	for _, s := range sourceItems {
		stats.Sources[s.Title] = s.Value
	}
	for _, n := range noteList.Notes {
		stats.Notes = append(stats.Notes, CreatorNoteRow{
			NoteID:         n.ID,
			Title:          n.Title,
			PostTime:       formatMs(n.PostTime, "2006-01-02 15:04"),
			Impressions:    n.Impressions,
			Views:          n.Views,
			CoverClickRate: math.Round(n.CoverClickRate*1000) / 10, // 接口给的是小数
			Likes:          n.Likes,
			Comments:       n.Comments,
			Collects:       n.Collects,
			NewFans:        n.NewFans,
			Shares:         n.Shares,
			ViewTimeAvg:    n.ViewTimeAvg,
			Danmaku:        n.Danmaku,
		})
	}
	for _, f := range activeItems {
		stats.ActiveFans = append(stats.ActiveFans, ActiveFan{UserID: f.UserID, Nickname: f.Name, Interactions: f.Count})
	}
	return stats, nil
}

func buildNoteStats(bodies map[string][]byte) (*NoteStats, error) {
	var base map[string]json.RawMessage
	if err := apiData(bodies, apiNoteBase, &base); err != nil {
		return nil, err
	}
	var info struct {
		ID       string `json:"id"`
		Desc     string `json:"desc"`
		Type     string `json:"type"`
		PostTime int64  `json:"post_time"`
		Tags     []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}
	_ = json.Unmarshal(base["note_info"], &info)
	var compare []struct {
		Quota   string  `json:"quota"`
		Count   float64 `json:"count"`
		Median  float64 `json:"median"`
		Summary string  `json:"summary"`
		Level   string  `json:"special_display_v2"`
	}
	_ = json.Unmarshal(base["analyse_infos"], &compare)
	var day, hour map[string]json.RawMessage
	_ = json.Unmarshal(base["day"], &day)
	_ = json.Unmarshal(base["hour"], &hour)
	var updated int64
	_ = json.Unmarshal(base["data_last_update_time"], &updated)

	stats := &NoteStats{
		NoteID:    info.ID,
		Title:     info.Desc,
		Type:      info.Type,
		PostTime:  formatMs(info.PostTime, "2006-01-02 15:04"),
		Topics:    []string{},
		UpdatedAt: formatMs(updated, "2006-01-02 15:04"),
		Core:      pick(base, noteCore),
		FansShare: pick(base, noteFansShare),
		FansOnly:  pick(base, noteFansOnly),
		Compare:   []NoteCompare{},
		Daily:     series(day, noteSeries, time.DateOnly),
		Hourly:    series(hour, noteSeries, "2006-01-02 15:04"),
	}
	for _, t := range info.Tags {
		stats.Topics = append(stats.Topics, t.Name)
	}
	for _, c := range compare {
		label := compareLabels[c.Quota]
		if label == "" {
			label = c.Quota
		}
		stats.Compare = append(stats.Compare, NoteCompare{Metric: label, Value: c.Count, Median: c.Median, Summary: c.Summary, Level: c.Level})
	}
	if err := apiData(bodies, apiNoteSource, &stats.Sources); err != nil {
		return nil, err
	}
	if err := apiData(bodies, apiNoteAudience, &stats.Audience); err != nil {
		return nil, err
	}
	return stats, nil
}

func pick(src map[string]json.RawMessage, ms []metric) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(ms))
	for _, m := range ms {
		if v, ok := src[m.key]; ok {
			out[m.label] = v
		}
	}
	return out
}

// series 把按指标分开的 [{date,count}] 列表合成按时间排序的行。
func series(src map[string]json.RawMessage, ms []metric, layout string) []map[string]any {
	rows := map[int64]map[string]any{}
	for _, m := range ms {
		var points []struct {
			Date       int64   `json:"date"`
			Count      float64 `json:"count"`
			CountFloat float64 `json:"count_with_double"`
		}
		if json.Unmarshal(src[m.key], &points) != nil {
			continue
		}
		for _, pt := range points {
			row := rows[pt.Date]
			if row == nil {
				row = map[string]any{"time": formatMs(pt.Date, layout)}
				rows[pt.Date] = row
			}
			// 比率类指标的值只在 count_with_double 里，计数类则只在 count 里。
			row[m.label] = pt.Count
			if pt.CountFloat != 0 {
				row[m.label] = pt.CountFloat
			}
		}
	}
	out := []map[string]any{}
	for _, d := range slices.Sorted(maps.Keys(rows)) {
		out = append(out, rows[d])
	}
	return out
}

func formatMs(ms int64, layout string) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).In(cst).Format(layout)
}
