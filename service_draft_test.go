package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

// 参数不合法时必须在下载图片、打开浏览器之前返回。
func TestSaveDraftRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		req  PublishRequest
		want string
	}{
		{PublishRequest{Title: " ", Content: "正文", Images: []string{"missing.png"}}, "标题和正文不能为空"},
		{PublishRequest{Title: "标题", Content: "\n", Images: []string{"missing.png"}}, "标题和正文不能为空"},
		{PublishRequest{Title: "标题", Content: "正文"}, "至少需要一张图片"},
		{PublishRequest{Title: "这是一个肯定超过二十个中文字上限的超长草稿标题", Content: "正文", Images: []string{"missing.png"}}, "标题长度超过限制"},
	} {
		_, err := NewXiaohongshuService().SaveDraft(context.Background(), &tt.req)
		require.ErrorContains(t, err, tt.want)
	}
}
