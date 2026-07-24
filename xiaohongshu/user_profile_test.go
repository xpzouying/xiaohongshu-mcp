package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserIDFromProfileURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		want    string
		wantErr bool
	}{
		{
			name:   "profile URL",
			rawURL: "https://www.xiaohongshu.com/user/profile/abc123?xsec_token=token",
			want:   "abc123",
		},
		{
			name:   "relative profile URL",
			rawURL: "/user/profile/abc123",
			want:   "abc123",
		},
		{
			name:    "missing user ID",
			rawURL:  "https://www.xiaohongshu.com/explore",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			rawURL:  "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := userIDFromProfileURL(tt.rawURL)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
