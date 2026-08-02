package xiaohongshu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNotificationTab(t *testing.T) {
	cases := []struct {
		in      string
		want    NotificationTab
		wantErr bool
	}{
		{"", TabMentions, false},
		{"mentions", TabMentions, false},
		{"LIKES", TabLikes, false},
		{"  connections  ", TabConnections, false},
		{"评论和@", "", true},
		{"unknown", "", true},
	}

	for _, c := range cases {
		got, err := ParseNotificationTab(c.in)
		if c.wantErr {
			assert.Error(t, err, "input=%q", c.in)
			continue
		}
		require.NoError(t, err, "input=%q", c.in)
		assert.Equal(t, c.want, got, "input=%q", c.in)
	}
}

// comment 构造一条带评论的通知。
func comment(id, nick, text, status string) rawNotification {
	return rawNotification{
		ID:       "n-" + id,
		Title:    "评论了你的笔记",
		UserInfo: rawUser{UserID: "u-" + id, Nickname: nick, XsecToken: "ut-" + id},
		Comment: rawComment{
			ID:      id,
			Content: text,
			Illegal: rawIllegal{Status: status},
		},
		Item: rawItem{
			ID:        "f-" + id,
			Type:      itemTypeNote,
			Content:   "笔记标题",
			XsecToken: "ft-" + id,
			Illegal:   rawIllegal{Status: statusNormal},
		},
	}
}

// 状态非正常的条目要被过滤掉，且其正文不能出现在结果里。
func TestConvertNotifications_FiltersInvisible(t *testing.T) {
	raw := []rawNotification{
		comment("c1", "甲", "正常评论", statusNormal),
		comment("c2", "乙", "不可见条目的正文", "NOT_NORMAL"),
		comment("c3", "丙", "另一条正常", statusNormal),
	}

	items, filtered := convertNotifications(raw, 20)

	require.Len(t, items, 2)
	assert.Equal(t, 1, filtered)
	assert.Equal(t, "c1", items[0].CommentID)
	assert.Equal(t, "c3", items[1].CommentID)

	for _, it := range items {
		assert.NotEqual(t, "不可见条目的正文", it.CommentText)
	}
}

// 白名单：未知状态一律按不可见处理，而不是放行。
func TestConvertNotifications_UnknownStatusIsFiltered(t *testing.T) {
	a := comment("c1", "甲", "x", "UNSEEN_A")
	b := comment("c2", "乙", "y", statusNormal)
	b.Item.Illegal.Status = "UNSEEN_B"

	items, filtered := convertNotifications([]rawNotification{a, b}, 20)

	assert.Empty(t, items)
	assert.Equal(t, 2, filtered)
}

// 状态字段为空时（例如「新增关注」没有评论/笔记）不应被误判为不可见。
func TestConvertNotifications_KeepsItemsWithoutStatus(t *testing.T) {
	raw := []rawNotification{
		{ID: "1", Title: "关注了你", User: rawUser{UserID: "u1", Nickname: "甲"}},
		{ID: "2", Title: "赞了你的笔记", Item: rawItem{ID: "f1", Type: itemTypeNote, Illegal: rawIllegal{Status: statusNormal}}},
	}

	items, filtered := convertNotifications(raw, 20)

	assert.Len(t, items, 2)
	assert.Zero(t, filtered)
}

// limit 只截正常条目，被过滤的不占名额。
func TestConvertNotifications_LimitCountsVisibleOnly(t *testing.T) {
	raw := []rawNotification{
		comment("c1", "甲", "x", "NOT_NORMAL"),
		comment("c2", "乙", "x", statusNormal),
		comment("c3", "丙", "x", statusNormal),
		comment("c4", "丁", "x", statusNormal),
	}

	items, filtered := convertNotifications(raw, 2)

	require.Len(t, items, 2)
	assert.Equal(t, "c2", items[0].CommentID)
	assert.Equal(t, "c3", items[1].CommentID)
	assert.Equal(t, 1, filtered)
}

// 非笔记类型的关联对象也带 id 和 token，但与笔记 id 不通用，不能给出去。
func TestConvertNotifications_OmitsFeedIDForNonNote(t *testing.T) {
	r := comment("c1", "甲", "x", statusNormal)
	r.Title = "收藏了你的笔记"
	r.Item.Type = "board_info"
	r.Item.Content = "某专辑"

	items, _ := convertNotifications([]rawNotification{r}, 20)

	require.Len(t, items, 1)
	assert.Empty(t, items[0].FeedID)
	assert.Empty(t, items[0].FeedXsecToken)
	// 标题仍要保留，调用方还得知道发生了什么
	assert.Equal(t, "某专辑", items[0].FeedTitle)
	assert.Equal(t, "收藏了你的笔记", items[0].Title)
}

// TestNotificationPayload_Unmarshal 固定页面状态到结构体的字段映射。
//
// 字段名一旦对不上，解析出来的是零值而不是报错——列表会静默变成一堆空条目，
// 因此用一份同形报文钉住。
func TestNotificationPayload_Unmarshal(t *testing.T) {
	const fixture = `{
		"cursor": "1",
		"hasMore": true,
		"messageList": [{
			"id": "7000000000000000001",
			"type": "comment/item",
			"title": "评论了你的笔记",
			"time": 1700000000,
			"userInfo": {"userid": "u1", "nickname": "甲", "xsecToken": "ut1"},
			"commentInfo": {
				"id": "c1", "content": "评论正文", "liked": true,
				"illegalInfo": {"illegalStatus": "NORMAL"}
			},
			"itemInfo": {
				"id": "f1", "type": "note_info", "content": "笔记标题", "xsecToken": "ft1",
				"illegalInfo": {"illegalStatus": "NORMAL"}
			}
		}]
	}`

	var payload notificationPayload
	require.NoError(t, json.Unmarshal([]byte(fixture), &payload))

	assert.True(t, payload.HasMore)
	require.Len(t, payload.MessageList, 1)

	r := payload.MessageList[0]
	assert.Equal(t, "评论了你的笔记", r.Title)
	assert.Equal(t, int64(1700000000), r.Time)
	assert.Equal(t, "甲", r.from().Nickname)
	assert.Equal(t, "ut1", r.from().XsecToken)
	assert.Equal(t, "c1", r.Comment.ID)
	assert.True(t, r.Comment.Liked)
	assert.Equal(t, statusNormal, r.Comment.Illegal.Status)
	assert.Equal(t, itemTypeNote, r.Item.Type)
	assert.Equal(t, "ft1", r.Item.XsecToken)
	assert.True(t, r.visible())

	items, filtered := convertNotifications(payload.MessageList, 20)
	require.Len(t, items, 1)
	assert.Zero(t, filtered)
	assert.Equal(t, "f1", items[0].FeedID)
	assert.Equal(t, "ft1", items[0].FeedXsecToken)
}

// 「新增关注」分区的用户字段名与其他分区不同，两者都要能取到。
func TestRawNotification_From(t *testing.T) {
	const fixture = `{"messageList": [
		{"id": "1", "title": "关注了你", "user": {"userid": "u9", "nickname": "丙", "xsecToken": "ut9"}},
		{"id": "2", "title": "评论了你的笔记", "userInfo": {"userid": "u8", "nickname": "丁"}}
	]}`

	var payload notificationPayload
	require.NoError(t, json.Unmarshal([]byte(fixture), &payload))
	require.Len(t, payload.MessageList, 2)

	assert.Equal(t, "丙", payload.MessageList[0].from().Nickname)
	assert.Equal(t, "u9", payload.MessageList[0].from().UserID)
	assert.Equal(t, "丁", payload.MessageList[1].from().Nickname)
}
