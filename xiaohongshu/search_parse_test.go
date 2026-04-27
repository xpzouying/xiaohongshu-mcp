package xiaohongshu

import (
	"encoding/json"
	"testing"
)

// #569 搜索结果解析测试

// TestParseSearchFeedsWithNoteCard 验证标准搜索结果解析
func TestParseSearchFeedsWithNoteCard(t *testing.T) {
	// 模拟搜索结果 JSON（含 noteCard 嵌套）
	rawJSON := `[
		{
			"xsecToken": "ABCtoken123",
			"id": "feed001",
			"modelType": "note",
			"noteCard": {
				"type": "normal",
				"displayTitle": "露营攻略分享",
				"user": {
					"userId": "user001",
					"nickname": "露营达人",
					"avatar": "https://example.com/avatar.jpg"
				},
				"interactInfo": {
					"liked": false,
					"likedCount": "1234"
				}
			}
		}
	]`

	var feeds []Feed
	err := json.Unmarshal([]byte(rawJSON), &feeds)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(feeds) != 1 {
		t.Fatalf("应解析出 1 条 Feed，实际 %d 条", len(feeds))
	}

	feed := feeds[0]
	if feed.NoteCard.DisplayTitle != "露营攻略分享" {
		t.Errorf("displayTitle 应为「露营攻略分享」，实际为「%s」", feed.NoteCard.DisplayTitle)
	}
	if feed.NoteCard.User.Nickname != "露营达人" {
		t.Errorf("nickname 应为「露营达人」，实际为「%s」", feed.NoteCard.User.Nickname)
	}
	if feed.NoteCard.User.UserID != "user001" {
		t.Errorf("userId 应为「user001」，实际为「%s」", feed.NoteCard.User.UserID)
	}
}

// TestParseSearchFeedsWithFlatFields 验证搜索结果字段在顶层（非 noteCard 嵌套）时也能解析
func TestParseSearchFeedsWithFlatFields(t *testing.T) {
	// 小红书搜索结果可能把 displayTitle 放在顶层
	rawJSON := `[
		{
			"xsecToken": "ABCtoken456",
			"id": "feed002",
			"modelType": "note",
			"displayTitle": "平铺的标题",
			"noteCard": {
				"type": "normal",
				"displayTitle": "",
				"user": {}
			}
		}
	]`

	var feeds []Feed
	err := json.Unmarshal([]byte(rawJSON), &feeds)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	feed := feeds[0]
	title := feed.GetDisplayTitle()
	if title != "平铺的标题" {
		t.Errorf("GetDisplayTitle 应返回「平铺的标题」，实际为「%s」", title)
	}
}

// TestParseSearchFeedsWithNickName 验证 nickName（大写 N）也能解析
func TestParseSearchFeedsWithNickName(t *testing.T) {
	rawJSON := `[
		{
			"xsecToken": "token789",
			"id": "feed003",
			"noteCard": {
				"displayTitle": "测试",
				"user": {
					"userId": "user003",
					"nickName": "大写N昵称"
				}
			}
		}
	]`

	var feeds []Feed
	err := json.Unmarshal([]byte(rawJSON), &feeds)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	feed := feeds[0]
	nickname := feed.NoteCard.User.GetNickname()
	if nickname != "大写N昵称" {
		t.Errorf("GetNickname 应返回「大写N昵称」，实际为「%s」", nickname)
	}
}
