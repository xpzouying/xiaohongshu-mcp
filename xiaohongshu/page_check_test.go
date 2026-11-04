package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// TestClassifyBlockedPath 覆盖按路径判定的边界，重点是查询串不得参与判定。
func TestClassifyBlockedPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want error
	}{
		{"安全验证页", "/website-login/captcha", errors.ErrVerificationRequired},
		{"安全验证页带子路径", "/website-login/captcha/confirm", errors.ErrVerificationRequired},
		{"安全验证页尾斜杠", "/website-login/captcha/", errors.ErrVerificationRequired},
		{"登录页", "/website-login", errors.ErrLoginRequired},
		{"登录页尾斜杠", "/website-login/", errors.ErrLoginRequired},
		{"未知的登录子页归为登录", "/website-login/verify", errors.ErrLoginRequired},
		{"前缀相同但不是同一路径", "/website-loginX", nil},
		{"大小写敏感（站点固定小写）", "/WEBSITE-LOGIN/CAPTCHA", nil},
		{"搜索结果页", "/search_result", nil},
		{"首页", "/explore", nil},
		{"空路径", "", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, classifyBlockedPath(c.path))
		})
	}
}

// TestClassifyBlockedURL 确认判定用的是解析后的路径，而不是整条 URL 的子串。
func TestClassifyBlockedURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want error
	}{
		{
			name: "实采的安全验证地址（verifyType=124）",
			url: "https://www.xiaohongshu.com/website-login/captcha" +
				"?redirectPath=https%3A%2F%2Fwww.xiaohongshu.com%2Fsearch_result%2F%3Fkeyword%3D%E6%BB%91%E9%9B%AA" +
				"%26source%3Dweb_explore_feed%26exSource%3D" +
				"&verifyUuid=886fa543-636e-4994-8b49-bb527ab432e7&verifyType=124&verifyBiz=461&verifyMsg=null",
			want: errors.ErrVerificationRequired,
		},
		{
			// 关键回归：路径是登录页，只是查询串里带了验证页地址，不能判成验证页。
			name: "登录页 redirectPath 指向验证页",
			url:  "https://www.xiaohongshu.com/website-login/?redirectPath=/website-login/captcha",
			want: errors.ErrLoginRequired,
		},
		{
			name: "登录页 redirectPath 指向验证页（完整地址）",
			url:  "https://www.xiaohongshu.com/website-login/?redirectPath=https%3A%2F%2Fwww.xiaohongshu.com%2Fwebsite-login%2Fcaptcha",
			want: errors.ErrLoginRequired,
		},
		{
			name: "搜索页的查询串里带 captcha 也不误判",
			url:  "https://www.xiaohongshu.com/search_result?keyword=%E6%BB%91%E9%9B%AA&redirect=/website-login/captcha",
			want: nil,
		},
		{
			name: "正常搜索结果页",
			url:  "https://www.xiaohongshu.com/search_result?keyword=%E6%BB%91%E9%9B%AA&source=web_explore_feed",
			want: nil,
		},
		{
			// 编码斜杠保持编码：与 JS 侧 location.pathname 一致，两侧都判不出拦截页。
			name: "路径里的编码斜杠不参与匹配",
			url:  "https://www.xiaohongshu.com/website-login%2Fcaptcha",
			want: nil,
		},
		{
			name: "无法解析的地址按未拦截处理",
			url:  "https://www.xiaohongshu.com/%zz",
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, classifyBlockedURL(c.url))
		})
	}
}
