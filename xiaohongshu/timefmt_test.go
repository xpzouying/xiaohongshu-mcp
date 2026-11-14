package xiaohongshu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampText(t *testing.T) {
	cases := []struct {
		name string
		ts   int64
		want string
	}{
		// 笔记与评论给的是 13 位毫秒
		{"毫秒", 1702195200000, "2023-12-10 16:00"},
		// 单位判断错会把这个值读成 1970-01-20
		{"秒", 1702195200, "2023-12-10 16:00"},
		{"分界值按毫秒读", msTimestampMin, "2001-09-09 09:46"},
		{"分界值下一位按秒读", msTimestampMin - 1, "33658-09-27 09:46"},
		// 站点没给该字段时的零值，不能渲染成 1970
		{"零值", 0, ""},
		{"负数", -1, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, timestampText(c.ts))
		})
	}
}

// TestTimestampText_IgnoresLocalZone 钉住时区：输出跟着东八区走，
// 不跟运行机器的 TZ 走，否则同一条笔记在不同部署环境里日期会不一样。
func TestTimestampText_IgnoresLocalZone(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	assert.Equal(t, "2023-12-10 16:00", timestampText(1702195200000))
}

// TestFillCommentTimeText_Nested 子评论同样要回填。
// 这里也钉住「按下标改写」——若改回 for range 副本，断言会全部落空。
func TestFillCommentTimeText_Nested(t *testing.T) {
	comments := []Comment{
		{
			ID:         "c1",
			CreateTime: 1702195200000,
			SubComments: []Comment{
				{ID: "c1-1", CreateTime: 1702195300000},
				{ID: "c1-2", CreateTime: 0}, // 缺失时间戳，保持空串
			},
		},
		{ID: "c2", CreateTime: 1702281600000},
	}

	fillCommentTimeText(comments)

	assert.Equal(t, "2023-12-10 16:00", comments[0].CreateTimeText)
	assert.Equal(t, "2023-12-10 16:01", comments[0].SubComments[0].CreateTimeText)
	assert.Empty(t, comments[0].SubComments[1].CreateTimeText)
	assert.Equal(t, "2023-12-11 16:00", comments[1].CreateTimeText)
}

// TestTimeText_OmitEmpty 没有时间戳时字段不应出现在返回体里，
// 免得调用方看到 "timeText": "" 以为站点真给了个空日期。
func TestTimeText_OmitEmpty(t *testing.T) {
	empty, err := json.Marshal(Comment{ID: "c1"})
	require.NoError(t, err)
	assert.NotContains(t, string(empty), "createTimeText")

	filled, err := json.Marshal(Comment{ID: "c1", CreateTimeText: "2023-12-10 16:00"})
	require.NoError(t, err)
	assert.Contains(t, string(filled), `"createTimeText":"2023-12-10 16:00"`)

	note, err := json.Marshal(FeedDetail{NoteID: "n1"})
	require.NoError(t, err)
	assert.NotContains(t, string(note), "timeText")

	item, err := json.Marshal(NotificationItem{ID: "i1"})
	require.NoError(t, err)
	assert.NotContains(t, string(item), "time_text")
}
