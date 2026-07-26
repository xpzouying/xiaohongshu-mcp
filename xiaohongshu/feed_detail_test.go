package xiaohongshu

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// shouldSkipButton 决定「展开 N 条回复」按钮点不点。
// 这里把边界语义固定下来：threshold 是"允许展开的上限"，
// 回复数正好等于阈值仍会展开，只有严格超过才跳过。
func TestShouldSkipButton(t *testing.T) {
	regex := regexp.MustCompile(`展开\s*(\d+)\s*条回复`)

	tests := []struct {
		name      string
		text      string
		threshold int
		want      bool
	}{
		{name: "回复数超过阈值则跳过", text: "展开 20 条回复", threshold: 10, want: true},
		{name: "回复数等于阈值仍展开", text: "展开 10 条回复", threshold: 10, want: false},
		{name: "回复数小于阈值则展开", text: "展开 3 条回复", threshold: 10, want: false},

		{name: "阈值为 0 表示不限制", text: "展开 999 条回复", threshold: 0, want: false},
		{name: "阈值为负数同样不限制", text: "展开 999 条回复", threshold: -1, want: false},

		{name: "文案不含数字不跳过", text: "展开更多回复", threshold: 10, want: false},
		{name: "文案完全不匹配不跳过", text: "收起", threshold: 10, want: false},
		{name: "空文案不跳过", text: "", threshold: 10, want: false},

		{name: "数字与文字之间无空格", text: "展开20条回复", threshold: 10, want: true},
		{name: "数字与文字之间多个空格", text: "展开   20   条回复", threshold: 10, want: true},

		{name: "阈值为 1 时 2 条回复跳过", text: "展开 2 条回复", threshold: 1, want: true},
		{name: "阈值为 1 时 1 条回复展开", text: "展开 1 条回复", threshold: 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldSkipButton(tt.text, tt.threshold, regex))
		})
	}
}
