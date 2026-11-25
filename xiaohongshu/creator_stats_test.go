package xiaohongshu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 响应结构按 2026-11 创作者中心实测接口裁剪，数值为虚构。
var creatorBodies = map[string][]byte{
	apiAccountBase: []byte(`{"success":true,"msg":"成功","data":{"analyse_infos":[],
		"seven":{"begin_time":1793808000000,"end_time":1794326400000,
			"impl_count":2000,"impl_count_rate":600,"impl_count_rate_display":true,
			"view_count":200,"view_count_rate":0,"view_count_rate_display":false,
			"cover_click_rate":6.4,"cover_click_cycle_rate":-43,"cover_click_cycle_rate_display":true,
			"view_list":[{"date":1793894400000,"count":3,"count_with_double":0},{"date":1793808000000,"count":5,"count_with_double":0}],
			"cover_click_rate_list":[{"date":1793808000000,"count":0,"count_with_double":7.5}]},
		"thirty":{"begin_time":1791820800000,"end_time":1794326400000,"impl_count":9000}}}`),
	apiAccountSource: []byte(`{"success":true,"data":{"seven":[{"title":"搜索","value":6,"color":"#000","source_type":2}],"thirty":[]}}`),
	apiNoteList: []byte(`{"success":true,"data":{"total":12,"note_infos":[
		{"id":"n1","title":"标题","post_time":1794396596000,"imp_count":216,"read_count":132,"coverClickRate":0.061,"like_count":5,"view_time_avg":5}]}}`),
	apiFansOverall: []byte(`{"success":true,"data":{"seven":{"fans_count":6,"rise_fans_count":1,"leave_fans_count":0,
		"fans_list":[{"date":1793808000000,"count":5,"set_date":true}],"rise_fans_list":[{"date":1793808000000,"count":1}]}}}`),
	apiActiveFans: []byte(`{"success":true,"data":{"seven":[{"user_id":"u1","name":"粉丝甲","count":10,"set_count":true}]}}`),
}

func TestBuildCreatorStats(t *testing.T) {
	stats, err := buildCreatorStats("seven", creatorBodies)
	require.NoError(t, err)

	assert.Equal(t, "2026-11-05 至 2026-11-11", stats.Period)
	assert.JSONEq(t, `2000`, string(stats.Overview["曝光"]))
	assert.JSONEq(t, `6.4`, string(stats.Overview["封面点击率(%)"]))
	// 页面显示「-」（display=false）的环比不返回。
	assert.Contains(t, stats.Change, "曝光")
	assert.Contains(t, stats.Change, "封面点击率(%)")
	assert.NotContains(t, stats.Change, "观看")

	// 按日期排序合并；比率取 count_with_double，计数取 count。
	require.Len(t, stats.Daily, 2)
	assert.Equal(t, "2026-11-05", stats.Daily[0]["time"])
	assert.Equal(t, 5.0, stats.Daily[0]["观看"])
	assert.Equal(t, 7.5, stats.Daily[0]["封面点击率(%)"])
	assert.Equal(t, 3.0, stats.Daily[1]["观看"])

	assert.Equal(t, map[string]float64{"搜索": 6}, stats.Sources)
	assert.Equal(t, 12, stats.NotesTotal)
	require.Len(t, stats.Notes, 1)
	assert.Equal(t, 6.1, stats.Notes[0].CoverClickRate) // 小数转百分比
	assert.Equal(t, "2026-11-11 19:29", stats.Notes[0].PostTime)

	assert.JSONEq(t, `6`, string(stats.Fans["总粉丝数"]))
	assert.Equal(t, 1.0, stats.FansDaily[0]["新增粉丝数"])
	assert.Equal(t, []ActiveFan{{UserID: "u1", Nickname: "粉丝甲", Interactions: 10}}, stats.ActiveFans)

	thirty, err := buildCreatorStats("thirty", creatorBodies)
	require.NoError(t, err)
	assert.JSONEq(t, `9000`, string(thirty.Overview["曝光"]))
}

func TestBuildCreatorStatsAPIFailure(t *testing.T) {
	bodies := map[string][]byte{}
	for k, v := range creatorBodies {
		bodies[k] = v
	}
	bodies[apiNoteList] = []byte(`{"success":false,"msg":"无权限"}`)
	_, err := buildCreatorStats("seven", bodies)
	require.ErrorContains(t, err, "无权限")
}

func TestBuildNoteStats(t *testing.T) {
	bodies := map[string][]byte{
		apiNoteBase: []byte(`{"success":true,"data":{
			"note_info":{"id":"n1","desc":"标题","type":"VIDEO","post_time":1794396596000,"tags":[{"type":"topic","name":"考研英语"}]},
			"data_last_update_time":1794412800000,
			"impl_count":120,"view_count":15,"exit_view2s_rate":78.3,
			"impl_count_rate_with_fans":14.2,"cover_click_rate_with_fans":64.7,
			"analyse_infos":[{"quota":"clickRate","count":0.06,"median":0.048,"summary":"超过61%同类","special_display_v2":"有待提升"},{"quota":"vqa","count":4.3}],
			"day":{"view_list":[{"date":1794326400000,"count":131,"count_with_double":131}]},
			"hour":{"view_list":[{"date":1794398400000,"count":25,"count_with_double":25}],"imp_list":[]}}}`),
		apiNoteSource:   []byte(`{"success":true,"data":{"no_data":true,"no_data_tip_msg":"观看数不足100，暂时无法分析","source":[]}}`),
		apiNoteAudience: []byte(`{"success":true,"data":{"no_data":false,"gender":[{"title":"女","value":60}]}}`),
	}
	stats, err := buildNoteStats(bodies)
	require.NoError(t, err)

	assert.Equal(t, "标题", stats.Title)
	assert.Equal(t, []string{"考研英语"}, stats.Topics)
	assert.Equal(t, "2026-11-12 00:00", stats.UpdatedAt)
	assert.JSONEq(t, `78.3`, string(stats.Core["2秒退出率(%)"]))
	assert.JSONEq(t, `14.2`, string(stats.FansShare["曝光"]))
	assert.JSONEq(t, `64.7`, string(stats.FansOnly["封面点击率(%)"]))
	require.Len(t, stats.Compare, 2)
	assert.Equal(t, "封面点击率", stats.Compare[0].Metric)
	assert.Equal(t, "vqa", stats.Compare[1].Metric) // 未知指标保留原名
	assert.Equal(t, "2026-11-11 20:00", stats.Hourly[0]["time"])
	assert.True(t, stats.Sources.NoData)
	assert.Equal(t, "观看数不足100，暂时无法分析", stats.Sources.Tip)
	out, err := json.Marshal(stats.Audience)
	require.NoError(t, err)
	assert.JSONEq(t, `{"no_data":false,"gender":[{"title":"女","value":60}]}`, string(out))
}

func TestPeriodKeyAndMatch(t *testing.T) {
	for days, want := range map[int]string{0: "seven", 7: "seven", 30: "thirty"} {
		got, err := periodKey(days)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	_, err := periodKey(14)
	assert.Error(t, err)

	paths := []string{apiNoteSource, apiNoteAudience}
	assert.Equal(t, apiNoteSource, matchCreatorAPI("https://creator.xiaohongshu.com"+apiNoteSource+"?note_id=1", paths))
	assert.Equal(t, apiNoteAudience, matchCreatorAPI("https://creator.xiaohongshu.com"+apiNoteAudience+"?note_id=1", paths))
	assert.Empty(t, matchCreatorAPI("https://edith.xiaohongshu.com"+apiNoteSource, paths))
}
