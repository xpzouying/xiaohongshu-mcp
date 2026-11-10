package xiaohongshu

import "testing"

func TestMentionTextMatches(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"完全一致", "小明", true},
		{"带 @ 前缀的候选", "@小明", true},
		{"拼音不是昵称", "XiaoMing", false},
		{"带粉丝数后缀（空格分隔）", "小明 12万粉丝", true},
		{"带分隔符后缀", "小明 · 关注", true},
		{"前缀相同但连着别的字", "小明同学", false},
		{"空候选", "", false},
		{"候选不含目标", "旅行日记", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mentionTextMatches(tc.candidate, "小明"); got != tc.want {
				t.Fatalf("mentionTextMatches(%q, %q) = %v, want %v", tc.candidate, "小明", got, tc.want)
			}
		})
	}
}

func TestMentionTextMatchesCaseInsensitive(t *testing.T) {
	if !mentionTextMatches("xiaoming", "XiaoMing") {
		t.Fatal("英文昵称应大小写不敏感")
	}
}

func TestMentionTextMatchesStripsAt(t *testing.T) {
	if !mentionTextMatches("小明", "@小明") {
		t.Fatal("请求侧带 @ 也应能匹配")
	}
}

func TestMentionAttrMatches(t *testing.T) {
	// data-user 的真实结构：{"userId":"<24位id>","nickname":"<昵称>"}
	attr := `{"userId":"0123456789abcdef01234567","nickname":"张三"}`
	u, ok := parseMentionUser(attr)
	if !ok || u.UserID != "0123456789abcdef01234567" || u.Nickname != "张三" {
		t.Fatalf("解析 data-user 失败: %+v ok=%v", u, ok)
	}
	if _, ok := parseMentionUser(`not-json`); ok {
		t.Fatal("非法 JSON 不应解析成功")
	}
	if _, ok := parseMentionUser(`{}`); ok {
		t.Fatal("空对象不应解析成功")
	}
}

func TestParseAtTarget(t *testing.T) {
	tests := []struct {
		raw      string
		wantName string
		wantID   string
	}{
		{"张三", "张三", ""},
		{"@张三", "张三", ""},
		{"  张三  ", "张三", ""},
		{"0123456789abcdef01234567", "", "0123456789abcdef01234567"},
		{"张三|0123456789abcdef01234567", "张三", "0123456789abcdef01234567"},
		{"张三｜0123456789abcdef01234567", "张三", "0123456789abcdef01234567"},
		{"小明|不是id", "小明|不是id", ""},
		{"", "", ""},
	}
	for _, tc := range tests {
		got := parseAtTarget(tc.raw)
		if got.Name != tc.wantName || got.UserID != tc.wantID {
			t.Fatalf("parseAtTarget(%q) = %+v, want name=%q id=%q", tc.raw, got, tc.wantName, tc.wantID)
		}
	}
}

func TestMentionTextEquals(t *testing.T) {
	if !mentionTextEquals("@就是66", "就是66") {
		t.Fatal("节点文本 @昵称 应去掉 @ 后匹配")
	}
	if !mentionTextEquals(" xiaoming ", "XiaoMing") {
		t.Fatal("空白与大小写应归一化")
	}
	if mentionTextEquals("@就是66", "就是6") {
		t.Fatal("不同昵称不应匹配")
	}
}
