package xiaohongshu

import "testing"

func TestDetectPublishState(t *testing.T) {
	tests := []struct {
		name     string
		pageText string
		want     publishState
	}{
		{
			name:     "发布成功",
			pageText: "你的笔记已提交，发布成功，稍后可在主页查看",
			want:     publishStateSuccess,
		},
		{
			name:     "图片仍在上传",
			pageText: "封面处理中，图片还未上传成功，请稍后再试",
			want:     publishStateImageUploading,
		},
		{
			name:     "图片仍在上传-备用文案",
			pageText: "当前图片还未上传完成，请稍后重新发布",
			want:     publishStateImageUploading,
		},
		{
			name:     "未知状态",
			pageText: "正在保存草稿",
			want:     publishStateUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectPublishState(tt.pageText); got != tt.want {
				t.Fatalf("detectPublishState() = %v, want %v", got, tt.want)
			}
		})
	}
}
