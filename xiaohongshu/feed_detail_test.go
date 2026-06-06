package xiaohongshu

import (
	"fmt"
	"testing"
)

func TestIsPermanentAccessError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error 不是永久错误",
			err:  nil,
			want: false,
		},
		{
			name: "笔记已删除是永久错误",
			err:  fmt.Errorf("笔记不可访问: 该笔记已被删除"),
			want: true,
		},
		{
			name: "私密笔记是永久错误",
			err:  fmt.Errorf("笔记不可访问: 私密笔记"),
			want: true,
		},
		{
			name: "因用户设置不可见是永久错误",
			err:  fmt.Errorf("笔记不可访问: 因用户设置，你无法查看"),
			want: true,
		},
		{
			name: "风控拦截页是临时错误，应当重试",
			err:  fmt.Errorf("笔记不可访问: Sorry, This Page Isn't Available Right Now.\n请打开小红书App扫码查看"),
			want: false,
		},
		{
			name: "其他未知错误视为临时错误",
			err:  fmt.Errorf("笔记不可访问: some unknown error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermanentAccessError(tt.err); got != tt.want {
				t.Errorf("isPermanentAccessError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
