package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeShortLinkURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "完整 https 链接",
			input: "https://xhslink.com/o/1kZVDhZks0n",
			want:  "https://xhslink.com/o/1kZVDhZks0n",
		},
		{
			name:  "http 链接强制转为 https",
			input: "http://xhslink.com/o/1kZVDhZks0n",
			want:  "https://xhslink.com/o/1kZVDhZks0n",
		},
		{
			name:  "无协议前缀自动补全",
			input: "xhslink.com/a/abc123",
			want:  "https://xhslink.com/a/abc123",
		},
		{
			name:  "首尾空白被清理",
			input: "  https://xhslink.com/o/abc  ",
			want:  "https://xhslink.com/o/abc",
		},
		{
			name:  "子域名同样支持",
			input: "https://www.xhslink.com/o/abc",
			want:  "https://www.xhslink.com/o/abc",
		},
		{
			name:    "非白名单域名",
			input:   "https://example.com/o/abc",
			wantErr: true,
		},
		{
			name:    "无协议且非白名单域名",
			input:   "example.com/o/abc",
			wantErr: true,
		},
		{
			name:    "空字符串",
			input:   "",
			wantErr: true,
		},
		{
			name:    "非法 URL",
			input:   "https://xhslink.com/%zz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeShortLinkURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractFeedIDFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "discovery/item 路径",
			path: "/discovery/item/69ba1706000000001a023daf",
			want: "69ba1706000000001a023daf",
		},
		{
			name: "explore 路径",
			path: "/explore/69ba1706000000001a023daf",
			want: "69ba1706000000001a023daf",
		},
		{
			name: "note 路径",
			path: "/note/69ba1706000000001a023daf",
			want: "69ba1706000000001a023daf",
		},
		{
			name: "空路径",
			path: "",
			want: "",
		},
		{
			name: "不匹配的路径",
			path: "/user/profile/123456",
			want: "",
		},
		{
			name: "路径缺少 ID",
			path: "/explore/",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, extractFeedIDFromPath(tt.path))
		})
	}
}

func TestParseRedirectURL(t *testing.T) {
	const originalURL = "https://xhslink.com/o/1kZVDhZks0n"

	t.Run("完整参数解析成功", func(t *testing.T) {
		redirect := "https://www.xiaohongshu.com/discovery/item/69ba1706000000001a023daf" +
			"?app_platform=ios&share_id=f5d4aabc8e014cce9d331b372eb94a22" +
			"&xsec_source=app_share&xsec_token=CBc35g8tRNZgaxC5icq1yawooXNOzVHw8ke6PGldcYa8U%3D"

		result, err := parseRedirectURL(redirect, originalURL)
		require.NoError(t, err)
		require.Equal(t, "69ba1706000000001a023daf", result.FeedID)
		require.Equal(t, "CBc35g8tRNZgaxC5icq1yawooXNOzVHw8ke6PGldcYa8U=", result.XsecToken)
		require.Equal(t, originalURL, result.OriginalURL)
		require.Equal(t, redirect, result.RedirectURL)
		require.Equal(t, "f5d4aabc8e014cce9d331b372eb94a22", result.ShareID)
		require.Equal(t, "ios", result.AppPlatform)
		require.Equal(t, "app_share", result.XsecSource)
	})

	t.Run("可选参数缺失时为空", func(t *testing.T) {
		redirect := "https://www.xiaohongshu.com/explore/69ba1706000000001a023daf?xsec_token=abc"

		result, err := parseRedirectURL(redirect, originalURL)
		require.NoError(t, err)
		require.Equal(t, "69ba1706000000001a023daf", result.FeedID)
		require.Equal(t, "abc", result.XsecToken)
		require.Empty(t, result.ShareID)
		require.Empty(t, result.AppPlatform)
		require.Empty(t, result.XsecSource)
	})

	t.Run("缺少 feed_id", func(t *testing.T) {
		_, err := parseRedirectURL("https://www.xiaohongshu.com/user/profile/123?xsec_token=abc", originalURL)
		require.Error(t, err)
	})

	t.Run("缺少 xsec_token", func(t *testing.T) {
		_, err := parseRedirectURL("https://www.xiaohongshu.com/explore/69ba1706000000001a023daf", originalURL)
		require.Error(t, err)
	})

	t.Run("非法重定向 URL", func(t *testing.T) {
		_, err := parseRedirectURL("https://xiaohongshu.com/explore/%zz", originalURL)
		require.Error(t, err)
	})
}
