package configs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// TestResolveFingerprintSeed 校验 seed 取值优先级：环境变量 > 会话文件 > 新生成并写回。
func TestResolveFingerprintSeed(t *testing.T) {
	newStore := func(t *testing.T) cookies.Cookier {
		t.Helper()
		return cookies.NewLoadCookie(filepath.Join(t.TempDir(), "cookies.json"))
	}

	t.Run("环境变量优先于文件", func(t *testing.T) {
		t.Setenv("XHS_FP_SEED", "11111")
		store := newStore(t)
		assert.NoError(t, store.SaveSeed(22222))

		assert.Equal(t, 11111, ResolveFingerprintSeed(store))
	})

	t.Run("没有环境变量时用文件里的", func(t *testing.T) {
		t.Setenv("XHS_FP_SEED", "")
		store := newStore(t)
		assert.NoError(t, store.SaveSeed(22222))

		assert.Equal(t, 22222, ResolveFingerprintSeed(store))
	})

	t.Run("都没有时生成一个并写回，下次读到同一个", func(t *testing.T) {
		t.Setenv("XHS_FP_SEED", "")
		store := newStore(t)

		got := ResolveFingerprintSeed(store)
		assert.Positive(t, got)
		// 关键：必须落盘，否则下次启动又换一套
		assert.Equal(t, got, store.LoadSeed())
		assert.Equal(t, got, ResolveFingerprintSeed(store))
	})
}
