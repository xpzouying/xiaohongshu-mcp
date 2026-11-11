package xiaohongshu

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestHasCreatorSessionCookie(t *testing.T) {
	cases := []struct {
		name    string
		cookies []*proto.NetworkCookie
		want    bool
	}{
		{
			name:    "galaxy_creator_session_id 存在",
			cookies: []*proto.NetworkCookie{{Name: creatorSessionCookie, Domain: "creator.xiaohongshu.com"}},
			want:    true,
		},
		{
			name:    "无 galaxy 时回退 web_session",
			cookies: []*proto.NetworkCookie{{Name: creatorSessionWebSessionCookie, Domain: ".xiaohongshu.com"}},
			want:    true,
		},
		{
			name:    "其他域名的 web_session 不算",
			cookies: []*proto.NetworkCookie{{Name: creatorSessionWebSessionCookie, Domain: ".example.com"}},
			want:    false,
		},
		{
			name:    "没有任何登录态 cookie",
			cookies: []*proto.NetworkCookie{{Name: "a1", Domain: ".xiaohongshu.com"}},
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasCreatorSessionCookie(tc.cookies); got != tc.want {
				t.Fatalf("hasCreatorSessionCookie() = %v, want %v", got, tc.want)
			}
		})
	}
}
