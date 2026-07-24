package xiaohongshu

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQrcodePageConfig(t *testing.T) {
	tests := []struct {
		name         string
		pageURL      string
		wantSelector string
		wantTimeout  time.Duration
	}{
		{
			name:         "普通登录页",
			pageURL:      "https://www.xiaohongshu.com/explore",
			wantSelector: loginQrcodeSelector,
			wantTimeout:  loginQrcodeTimeout,
		},
		{
			name:         "安全验证页",
			pageURL:      "https://www.xiaohongshu.com/website-login/captcha?verifyType=124",
			wantSelector: verificationQrcodeSelector,
			wantTimeout:  verificationQrcodeTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, timeout := qrcodePageConfig(tt.pageURL)
			require.Equal(t, tt.wantSelector, selector)
			require.Equal(t, tt.wantTimeout, timeout)
		})
	}
}
