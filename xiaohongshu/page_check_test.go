package xiaohongshu

import (
	"testing"

	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// TestClassifyBlockedURL 用的是实采地址：前两条来自真实的"安全验证"重定向，
// 其余是正常页面，用来确认不会误判。
func TestClassifyBlockedURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want error
	}{
		{
			name: "安全验证页（实采，verifyType=124）",
			url: "https://www.xiaohongshu.com/website-login/captcha" +
				"?redirectPath=https%3A%2F%2Fwww.xiaohongshu.com%2Fsearch_result%2F%3Fkeyword%3D%E6%BB%91%E9%9B%AA" +
				"%26source%3Dweb_explore_feed%26exSource%3D" +
				"&verifyUuid=886fa543-636e-4994-8b49-bb527ab432e7&verifyType=124&verifyBiz=461&verifyMsg=null",
			want: errors.ErrVerificationRequired,
		},
		{
			name: "安全验证页（无 query）",
			url:  "https://www.xiaohongshu.com/website-login/captcha",
			want: errors.ErrVerificationRequired,
		},
		{
			name: "登录页",
			url:  "https://www.xiaohongshu.com/website-login/",
			want: errors.ErrLoginRequired,
		},
		{
			name: "搜索结果页（正常）",
			url:  "https://www.xiaohongshu.com/search_result?keyword=%E6%BB%91%E9%9B%AA&source=web_explore_feed",
			want: nil,
		},
		{
			name: "首页（正常）",
			url:  "https://www.xiaohongshu.com/explore",
			want: nil,
		},
		{
			name: "笔记详情页（正常）",
			url:  "https://www.xiaohongshu.com/explore/69bdf6df000000002102f2fb?xsec_token=abc&xsec_source=pc_feed",
			want: nil,
		},
		{
			name: "空地址",
			url:  "",
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyBlockedURL(c.url)
			if got != c.want {
				t.Fatalf("classifyBlockedURL(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

// TestClassifyBlockedURL_VerificationBeatsLogin 确认 captcha 与 login 前缀重叠时，
// 优先判定为"需要安全验证"而不是更笼统的"需要登录"——两者的处置方式不同。
func TestClassifyBlockedURL_VerificationBeatsLogin(t *testing.T) {
	url := "https://www.xiaohongshu.com/website-login/captcha?verifyType=124"
	if got := classifyBlockedURL(url); got != errors.ErrVerificationRequired {
		t.Fatalf("captcha 地址被判定为 %v，期望 ErrVerificationRequired", got)
	}
}
