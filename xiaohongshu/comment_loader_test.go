package xiaohongshu

import (
	"testing"
)

// #578 评论加载超时相关测试

// TestCalculateMaxAttempts 验证最大尝试次数计算
func TestCalculateMaxAttempts(t *testing.T) {
	tests := []struct {
		name            string
		maxCommentItems int
		wantMin         int
		wantMax         int
	}{
		{
			name:            "不限制评论数时使用默认值",
			maxCommentItems: 0,
			wantMin:         defaultMaxAttempts,
			wantMax:         defaultMaxAttempts,
		},
		{
			name:            "20条评论（保底不低于默认值）",
			maxCommentItems: 20,
			wantMin:         defaultMaxAttempts, // 20*3=60 < 500，保底为 500
			wantMax:         defaultMaxAttempts,
		},
		{
			name:            "800条评论应有足够尝试次数",
			maxCommentItems: 800,
			wantMin:         500, // 至少不低于默认值
			wantMax:         2400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CommentLoadConfig{MaxCommentItems: tt.maxCommentItems}
			cl := &commentLoader{config: config}
			got := cl.calculateMaxAttempts()

			if got < tt.wantMin {
				t.Errorf("maxAttempts=%d 应 >= %d", got, tt.wantMin)
			}
			if got > tt.wantMax {
				t.Errorf("maxAttempts=%d 应 <= %d", got, tt.wantMax)
			}
		})
	}
}

// TestGetScrollInterval 验证滚动间隔配置
func TestGetScrollInterval(t *testing.T) {
	tests := []struct {
		speed   string
		wantMin int // 毫秒
		wantMax int
	}{
		{"slow", 1200, 1500},
		{"normal", 600, 800},
		{"fast", 300, 400},
		{"unknown", 600, 800}, // 默认 normal
	}

	for _, tt := range tests {
		t.Run(tt.speed, func(t *testing.T) {
			// 多次测试覆盖随机范围
			for i := 0; i < 20; i++ {
				d := getScrollInterval(tt.speed)
				ms := int(d.Milliseconds())
				if ms < tt.wantMin || ms > tt.wantMax {
					t.Errorf("speed=%s: 间隔 %dms 不在 [%d, %d] 范围内", tt.speed, ms, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

// TestDefaultCommentLoadConfig 验证默认配置合理性
func TestDefaultCommentLoadConfig(t *testing.T) {
	config := DefaultCommentLoadConfig()

	if config.ScrollSpeed != "normal" {
		t.Errorf("默认 ScrollSpeed 应为 normal，实际 %s", config.ScrollSpeed)
	}
	if config.MaxCommentItems != 0 {
		t.Errorf("默认 MaxCommentItems 应为 0（不限制），实际 %d", config.MaxCommentItems)
	}
	if config.MaxRepliesThreshold != 10 {
		t.Errorf("默认 MaxRepliesThreshold 应为 10，实际 %d", config.MaxRepliesThreshold)
	}
}
