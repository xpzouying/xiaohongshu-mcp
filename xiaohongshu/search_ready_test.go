package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSearchResponse(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		empty   bool
		wantErr string
	}{
		{"有结果", `{"code":0,"success":true,"data":{"items":[{"id":"note"}],"has_more":true}}`, false, ""},
		{"空数组", `{"code":0,"success":true,"data":{"items":[],"has_more":false}}`, true, ""},
		{"空结果省略items", `{"code":0,"success":true,"data":{"has_more":false}}`, true, ""},
		{"空结果items为null", `{"code":0,"success":true,"data":{"items":null,"has_more":false}}`, true, ""},
		{"接口失败", `{"code":-100,"success":false,"msg":"fixture"}`, false, "code=-100"},
		{"成功字段与code冲突", `{"code":-100,"success":true,"data":{"has_more":false}}`, false, "code=-100"},
		{"缺少成功标记", `{"data":{"items":[],"has_more":false}}`, false, "成功标记"},
		{"缺少success", `{"code":0,"data":{"has_more":false}}`, false, "搜索接口"},
		{"缺少data", `{"code":0,"success":true}`, false, "缺少 data"},
		{"空对象不是空结果", `{"code":0,"success":true,"data":{}}`, false, "未确认结果为空"},
		{"尚有后续结果", `{"code":0,"success":true,"data":{"items":[],"has_more":true}}`, false, "未确认结果为空"},
		{"items类型错误", `{"code":0,"success":true,"data":{"items":{},"has_more":false}}`, false, "解析搜索响应"},
		{"无效JSON", `<html>fixture</html>`, false, "解析搜索响应"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseSearchResponse([]byte(tc.body))
			if tc.wantErr != "" {
				require.ErrorContains(t, result.err, tc.wantErr)
				require.False(t, result.empty)
				return
			}
			require.NoError(t, result.err)
			require.Equal(t, tc.empty, result.empty)
		})
	}
}

func TestIsSearchNotesURL(t *testing.T) {
	for _, raw := range []string{
		"https://so.xiaohongshu.com/api/sns/web/v2/search/notes",
		"https://edith.xiaohongshu.com/api/sns/web/v1/search/notes?fixture=1",
	} {
		require.True(t, isSearchNotesURL(raw), raw)
	}
	for _, raw := range []string{
		"https://so.xiaohongshu.com/api/sns/web/v1/search/recommend",
		"https://so.xiaohongshu.com/api/sns/web/v1/homefeed",
		"https://xiaohongshu.com.example.com/api/sns/web/v2/search/notes",
		"https://notxiaohongshu.com/api/sns/web/v2/search/notes",
		"://invalid",
	} {
		require.False(t, isSearchNotesURL(raw), raw)
	}
}
