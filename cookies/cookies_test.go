package cookies

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetCookiesFilePath 校验路径优先级：COOKIES_PATH > 当前目录 > /tmp（旧路径兜底）。
// 用 TMPDIR 重定向 os.TempDir()、t.Chdir 重定向当前目录，做到 hermetic、不碰真实 /tmp。
func TestGetCookiesFilePath(t *testing.T) {
	t.Run("显式指定的COOKIES_PATH永远最优先", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TMPDIR", dir)
		t.Setenv("COOKIES_PATH", "/custom/cookies.json")

		// 即使 /tmp 下躺着旧文件，也不能盖掉显式配置
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "cookies.json"), []byte("[]"), 0644))

		assert.Equal(t, "/custom/cookies.json", GetCookiesFilePath())
	})

	t.Run("未设COOKIES_PATH时本地目录优先于tmp", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		t.Setenv("COOKIES_PATH", "")

		// /tmp 下有旧文件
		assert.NoError(t, os.WriteFile(filepath.Join(tmp, "cookies.json"), []byte("[]"), 0644))
		// 本地目录也有
		cwd := t.TempDir()
		t.Chdir(cwd)
		assert.NoError(t, os.WriteFile(filepath.Join(cwd, "cookies.json"), []byte("[]"), 0644))

		assert.Equal(t, "cookies.json", GetCookiesFilePath())
	})

	t.Run("本地没有时兜底到tmp旧路径", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		t.Setenv("COOKIES_PATH", "")
		t.Chdir(t.TempDir()) // 本地目录是空的

		oldPath := filepath.Join(tmp, "cookies.json")
		assert.NoError(t, os.WriteFile(oldPath, []byte("[]"), 0644))

		assert.Equal(t, oldPath, GetCookiesFilePath())
	})

	t.Run("都不存在时回退当前目录", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TMPDIR", dir)
		t.Setenv("COOKIES_PATH", "")
		t.Chdir(t.TempDir())

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
