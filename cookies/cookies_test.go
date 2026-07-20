package cookies

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetCookiesFilePath 校验路径优先级：旧路径(/tmp/cookies.json) > COOKIES_PATH > 当前目录。
// 用 TMPDIR 重定向 os.TempDir() 到测试目录，做到 hermetic、不碰真实 /tmp。
func TestGetCookiesFilePath(t *testing.T) {
	t.Run("旧路径存在时优先返回旧路径", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TMPDIR", dir)
		t.Setenv("COOKIES_PATH", "/should/not/be/used.json")

		oldPath := filepath.Join(dir, "cookies.json")
		assert.NoError(t, os.WriteFile(oldPath, []byte("[]"), 0644))

		assert.Equal(t, oldPath, GetCookiesFilePath())
	})

	t.Run("旧路径不存在时用COOKIES_PATH", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TMPDIR", dir)
		t.Setenv("COOKIES_PATH", "/custom/cookies.json")

		assert.Equal(t, "/custom/cookies.json", GetCookiesFilePath())
	})

	t.Run("都不存在时回退当前目录", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TMPDIR", dir)
		t.Setenv("COOKIES_PATH", "")

		assert.Equal(t, "cookies.json", GetCookiesFilePath())
	})
}

// TestLoadSaveDeleteCookies 校验 cookie 文件存取往返与删除的幂等。
func TestLoadSaveDeleteCookies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	c := NewLoadCookie(path)

	// 未写入时读取应报错
	_, err := c.LoadCookies()
	assert.Error(t, err)

	// 写入后能原样读回
	want := []byte(`[{"name":"web_session","value":"x"}]`)
	assert.NoError(t, c.SaveCookies(want))
	got, err := c.LoadCookies()
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	// 删除后文件消失，且再次删除幂等（不报错）
	assert.NoError(t, c.DeleteCookies())
	assert.NoFileExists(t, path)
	assert.NoError(t, c.DeleteCookies())
}
