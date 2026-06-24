package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveVideoURLFromDetail(t *testing.T) {
	tests := []struct {
		name     string
		note     FeedDetail
		expected string
	}{
		{
			name: "优先 h264 masterUrl",
			note: FeedDetail{
				Video: &DetailVideo{
					Media: DetailVideoMedia{
						Stream: DetailVideoStream{
							H264: []DetailVideoStreamItem{
								{MasterURL: "https://example.com/h264.m3u8"},
							},
						},
						OriginVideoKey: "https://example.com/origin.mp4",
					},
				},
			},
			expected: "https://example.com/h264.m3u8",
		},
		{
			name: "h264 缺失时回退 h265 masterUrl",
			note: FeedDetail{
				Video: &DetailVideo{
					Media: DetailVideoMedia{
						Stream: DetailVideoStream{
							H265: []DetailVideoStreamItem{
								{MasterURL: "https://example.com/h265.m3u8"},
							},
						},
					},
				},
			},
			expected: "https://example.com/h265.m3u8",
		},
		{
			name: "stream 缺失时回退 originVideoKey",
			note: FeedDetail{
				Video: &DetailVideo{
					Media: DetailVideoMedia{
						OriginVideoKey: "https://example.com/origin.mp4",
					},
				},
			},
			expected: "https://example.com/origin.mp4",
		},
		{
			name:     "非视频帖子返回空字符串",
			note:     FeedDetail{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ResolveVideoURLFromDetail(tt.note)
			require.Equal(t, tt.expected, actual)
		})
	}
}
