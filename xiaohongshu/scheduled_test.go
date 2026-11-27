package xiaohongshu

import (
	"strings"
	"testing"
)

func TestScheduledScanPagination(t *testing.T) {
	s := newScheduledScan()
	first := `{"success":true,"code":0,"data":{"notes":[{"id":"published","tab_status":1,"schedule_post_time":0},{"id":"image","display_title":"图文预约","type":"normal","tab_status":4,"post_timing":true,"schedule_post_time":1793754000000}],"page":1,"tags":[{"notes_count":3}]}}`
	last := `{"success":true,"code":0,"data":{"notes":[{"id":"video","display_title":"视频预约","type":"video","tab_status":4,"post_timing":true,"schedule_post_time":1793793600000}],"page":-1,"tags":[{"notes_count":3}]}}`
	if err := s.add(0, []byte(first)); err != nil {
		t.Fatal(err)
	}
	if s.done || s.nextPage != 1 {
		t.Fatal("first page must not imply completeness")
	}
	if err := s.add(1, []byte(last)); err != nil {
		t.Fatal(err)
	}
	if !s.done || s.pages != 2 || len(s.seen) != 3 || len(s.items) != 2 {
		t.Fatalf("unexpected scan: %+v", s)
	}
	if s.items[0].Title != "图文预约" || s.items[0].ScheduleAt != "2026-11-04T09:00:00+08:00" {
		t.Fatalf("unexpected appointment: %+v", s.items[0])
	}
}

func TestScheduledScanEmptyAndFailures(t *testing.T) {
	empty := `{"success":true,"code":0,"data":{"notes":[],"page":-1,"tags":[{"notes_count":0}]}}`
	s := newScheduledScan()
	if err := s.add(0, []byte(empty)); err != nil || !s.done || s.items == nil {
		t.Fatalf("valid empty list: %v", err)
	}
	for _, body := range []string{
		`{"success":false,"code":-100}`,
		`{"success":true,"code":0,"data":{}}`,
		strings.Replace(empty, `"notes_count":0`, `"notes_count":1`, 1),
		strings.Replace(empty, `"page":-1`, `"page":0`, 1),
		`{"success":true,"code":0,"data":{"notes":[{"id":"x","tab_status":99,"schedule_post_time":0}],"page":-1,"tags":[{"notes_count":1}]}}`,
		`{"success":true,"code":0,"data":{"notes":[{"id":"x","tab_status":4,"post_timing":true,"schedule_post_time":0}],"page":-1,"tags":[{"notes_count":1}]}}`,
		`{"success":true,"code":0,"data":{"notes":[{"id":"x","tab_status":4,"post_timing":true,"schedule_post_time":1793754000}],"page":-1,"tags":[{"notes_count":1}]}}`,
	} {
		if err := newScheduledScan().add(0, []byte(body)); err == nil {
			t.Fatalf("must reject incomplete/invalid response: %s", body)
		}
	}
}

func TestScheduledScanRejectsChangedOrRepeatedPages(t *testing.T) {
	first := `{"success":true,"code":0,"data":{"notes":[{"id":"first","tab_status":1,"schedule_post_time":0}],"page":1,"tags":[{"notes_count":2}]}}`
	last := `{"success":true,"code":0,"data":{"notes":[{"id":"second","tab_status":1,"schedule_post_time":0}],"page":-1,"tags":[{"notes_count":2}]}}`
	for _, tc := range []struct {
		name string
		page int
		body string
	}{
		{"重复游标", 0, last},
		{"跳过请求页", 2, last},
		{"总数变化", 1, strings.Replace(last, `"notes_count":2`, `"notes_count":3`, 1)},
		{"重复笔记", 1, strings.Replace(last, `"second"`, `"first"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newScheduledScan()
			if err := s.add(0, []byte(first)); err != nil {
				t.Fatal(err)
			}
			if err := s.add(tc.page, []byte(tc.body)); err == nil {
				t.Fatal("不完整分页不能返回成功")
			}
		})
	}
}
