package xiaohongshu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcceptsImage(t *testing.T) {
	cases := []struct {
		name   string
		accept string
		want   bool
	}{
		{"图片扩展名", ".png,.webp", true},
		{"大写扩展名", ".JPG", true},
		{"MIME 通配", "image/*", true},
		{"非图片扩展名", ".xyz,.abc", false},
		{"非图片 MIME", "video/mp4", false},
		{"空值", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, acceptsImage(c.accept))
		})
	}
}

func TestCheckImagePaths(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "ok.jpg")
	require.NoError(t, os.WriteFile(exists, []byte("x"), 0o644))
	missing := filepath.Join(dir, "missing.jpg")
	missing2 := filepath.Join(dir, "missing2.jpg")

	cases := []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{"都存在", []string{exists}, false},
		{"存在与缺失混合", []string{exists, missing}, true},
		{"全部缺失", []string{missing, missing2}, true},
		{"空列表", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkImagePaths(c.paths)
			if !c.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "图片文件不存在或不可读")
		})
	}

	// 缺失的文件要一次性全部列出，调用方才知道该补哪几个
	t.Run("缺失文件全部列出", func(t *testing.T) {
		err := checkImagePaths([]string{exists, missing, missing2})
		require.Error(t, err)
		assert.Contains(t, err.Error(), missing)
		assert.Contains(t, err.Error(), missing2)
		assert.NotContains(t, err.Error(), exists)
	})
}
