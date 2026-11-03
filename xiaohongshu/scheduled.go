package xiaohongshu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const creatorNotesPath = "/api/galaxy/v2/creator/note/user/posted"
const creatorUserPath = "/api/galaxy/user/info"
const creatorNoteManagerURL = "https://creator.xiaohongshu.com/new/note-manager"

// ScheduledNote 是仍有效的预约，也包含在后台手动建立的预约。
type ScheduledNote struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	ScheduleAt string `json:"schedule_at"`
}

type ScheduledNotesResponse struct {
	AccountID   string          `json:"account_id"`
	AccountName string          `json:"account_name"`
	Source      string          `json:"source"`
	Timezone    string          `json:"timezone"`
	CheckedAt   string          `json:"checked_at"`
	Complete    bool            `json:"complete"`
	Pages       int             `json:"pages"`
	Scanned     int             `json:"scanned_notes"`
	Count       int             `json:"count"`
	Items       []ScheduledNote `json:"items"`
}

type creatorNote struct {
	ID           string `json:"id"`
	Title        string `json:"display_title"`
	Type         string `json:"type"`
	Status       *int   `json:"tab_status"`
	PostTiming   bool   `json:"post_timing"`
	ScheduleTime *int64 `json:"schedule_post_time"`
}

type creatorNotesEnvelope struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    struct {
		Notes *[]creatorNote `json:"notes"`
		Page  *int           `json:"page"`
		Tags  []struct {
			Count *int `json:"notes_count"`
		} `json:"tags"`
	} `json:"data"`
}

type scheduledScan struct {
	nextPage int
	total    int
	pages    int
	done     bool
	seen     map[string]bool
	items    []ScheduledNote
}

func newScheduledScan() *scheduledScan {
	return &scheduledScan{total: -1, seen: map[string]bool{}, items: []ScheduledNote{}}
}

// add 仅在后台返回 -1 游标且总数一致时确认完整；缺字段或分页异常都报错。
func (s *scheduledScan) add(requestPage int, body []byte) error {
	var response creatorNotesEnvelope
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("解析后台笔记列表失败: %w", err)
	}
	d := response.Data
	if !response.Success || response.Code != 0 || d.Notes == nil || d.Page == nil || len(d.Tags) == 0 || d.Tags[0].Count == nil {
		return fmt.Errorf("后台列表读取失败或缺少完整性字段（code=%d），请检查登录或页面结构", response.Code)
	}
	if s.done || requestPage != s.nextPage || *d.Page < -1 || (*d.Page != -1 && *d.Page <= requestPage) {
		return fmt.Errorf("后台分页重复、跳页或游标无效，无法确认完整预约列表")
	}
	if *d.Tags[0].Count < 0 || (s.total >= 0 && s.total != *d.Tags[0].Count) {
		return fmt.Errorf("查询期间后台笔记总数变化，请重新查询")
	}
	s.total = *d.Tags[0].Count
	for _, note := range *d.Notes {
		if note.ID == "" || s.seen[note.ID] || note.Status == nil || note.ScheduleTime == nil || *note.Status < 1 || *note.Status > 4 {
			return fmt.Errorf("笔记标识重复或状态/预约字段缺失，无法确认完整预约列表")
		}
		s.seen[note.ID] = true
		stamp := *note.ScheduleTime
		if *note.Status == 4 {
			if stamp <= 0 || !note.PostTiming {
				return fmt.Errorf("预约笔记 %s 的预约时间或状态不完整", note.ID)
			}
			// 创作者中心使用毫秒时间戳，界面时区为 GMT+8。
			when := time.UnixMilli(stamp).In(time.FixedZone("Asia/Shanghai", 8*60*60))
			if when.Year() < 2020 || when.Year() > 2100 {
				return fmt.Errorf("预约笔记 %s 的时间单位或数值异常", note.ID)
			}
			s.items = append(s.items, ScheduledNote{ID: note.ID, Title: note.Title, Type: note.Type, Status: "scheduled", ScheduleAt: when.Format(time.RFC3339)})
		} else if stamp != 0 || note.PostTiming {
			return fmt.Errorf("笔记 %s 的预约标记与后台状态不一致，请重新查询", note.ID)
		}
	}
	s.pages++
	s.nextPage = *d.Page
	if len(s.seen) > s.total || (s.nextPage == -1 && len(s.seen) != s.total) {
		return fmt.Errorf("后台分页条数与总数不一致，无法确认完整预约列表")
	}
	s.done = s.nextPage == -1
	return nil
}

type creatorResponse struct {
	path string
	page int
	body []byte
	err  error
}

// ListScheduledNotes 读取创作者页面自身的请求响应，只导航和滚动“全部”页。
func ListScheduledNotes(ctx context.Context, browserPage *rod.Page) (*ScheduledNotesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	page := browserPage.Context(ctx)
	responses := make(chan creatorResponse, 64)
	pending := map[proto.NetworkRequestID]creatorResponse{}
	send := func(r creatorResponse) {
		select {
		case responses <- r:
		case <-ctx.Done():
		}
	}
	wait := page.EachEvent(func(e *proto.NetworkResponseReceived) {
		u, err := url.Parse(e.Response.URL)
		if err != nil || u.Hostname() != "creator.xiaohongshu.com" || (u.Path != creatorNotesPath && u.Path != creatorUserPath) {
			return
		}
		r := creatorResponse{path: u.Path}
		if u.Path == creatorNotesPath {
			if u.Query().Get("tab") != "0" {
				return
			}
			r.page, err = strconv.Atoi(u.Query().Get("page"))
			if err != nil {
				r.err = fmt.Errorf("后台请求缺少分页参数")
			}
		}
		if e.Response.Status != 200 {
			r.err = fmt.Errorf("创作者后台返回 HTTP %d，请检查登录状态", e.Response.Status)
		}
		pending[e.RequestID] = r
	}, func(e *proto.NetworkLoadingFinished) {
		r, ok := pending[e.RequestID]
		if !ok {
			return
		}
		delete(pending, e.RequestID)
		if r.err == nil {
			body, err := (proto.NetworkGetResponseBody{RequestID: e.RequestID}).Call(page)
			if err != nil {
				r.err = fmt.Errorf("读取后台响应失败: %w", err)
			} else if body.Base64Encoded {
				r.body, r.err = base64.StdEncoding.DecodeString(body.Body)
			} else {
				r.body = []byte(body.Body)
			}
		}
		send(r)
	}, func(e *proto.NetworkLoadingFailed) {
		if r, ok := pending[e.RequestID]; ok {
			delete(pending, e.RequestID)
			r.err = fmt.Errorf("后台列表请求失败: %s", e.ErrorText)
			send(r)
		}
	})
	listenerDone := make(chan struct{})
	go func() { defer close(listenerDone); wait() }()
	defer func() { cancel(); <-listenerDone }()
	if err := page.Navigate(creatorNoteManagerURL); err != nil {
		return nil, fmt.Errorf("打开笔记管理失败: %w", err)
	}

	result := &ScheduledNotesResponse{Source: "creator_backend", Timezone: "Asia/Shanghai"}
	scan := newScheduledScan()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	idle := time.NewTimer(35 * time.Second)
	defer idle.Stop()
	scrollToTop := false
	for !scan.done || result.AccountID == "" {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("后台预约查询未完成: %w", ctx.Err())
		case <-idle.C:
			return nil, fmt.Errorf("未取得完整后台预约列表或账号，可能登录失效、网络异常或页面结构变化")
		case r := <-responses:
			if r.err != nil {
				return nil, r.err
			}
			if r.path == creatorUserPath {
				var user struct {
					Success bool `json:"success"`
					Data    struct {
						ID   string `json:"userId"`
						Name string `json:"userName"`
					} `json:"data"`
				}
				if err := json.Unmarshal(r.body, &user); err != nil || !user.Success || user.Data.ID == "" {
					return nil, fmt.Errorf("创作者中心登录失效或无法确认当前账号")
				}
				if result.AccountID != "" && result.AccountID != user.Data.ID {
					return nil, fmt.Errorf("查询期间当前账号变化，请重新查询")
				}
				result.AccountID, result.AccountName = user.Data.ID, user.Data.Name
			} else {
				if err := scan.add(r.page, r.body); err != nil {
					return nil, err
				}
			}
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(35 * time.Second)
		case <-tick.C:
			if !scan.done && scan.pages > 0 {
				// 交替滚动到顶部标签和底部哨兵，触发页面原有分页加载。
				selector := "#notes-request"
				if scrollToTop {
					selector = ".tab-bar"
				}
				exists, element, err := page.Has(selector)
				if err != nil {
					return nil, fmt.Errorf("加载下一页预约失败: %w", err)
				}
				if exists {
					if err := element.ScrollIntoView(); err != nil {
						return nil, fmt.Errorf("加载下一页预约失败: %w", err)
					}
					scrollToTop = !scrollToTop
				}
			}
		}
	}
	sort.Slice(scan.items, func(i, j int) bool { return scan.items[i].ScheduleAt < scan.items[j].ScheduleAt })
	result.Items, result.Count, result.Pages, result.Scanned = scan.items, len(scan.items), scan.pages, len(scan.seen)
	result.Complete = true
	result.CheckedAt = time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format(time.RFC3339)
	return result, nil
}
