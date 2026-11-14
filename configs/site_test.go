package configs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWebHost 校验主站域名：XHS_SITE 优先，否则按 web_session 所在域名推断。
func TestWebHost(t *testing.T) {
	writeCookies := func(t *testing.T, body string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "cookies.json")
		assert.NoError(t, os.WriteFile(path, []byte(body), 0644))
		t.Setenv("COOKIES_PATH", path)
	}
	intl := `{"version":2,"cookies":[{"name":"a1","domain":".rednote.com"},{"name":"web_session","domain":".rednote.com"}]}`
	cn := `[{"name":"web_session","domain":".xiaohongshu.com"}]`

	t.Run("会话在 rednote.com 时走海外站", func(t *testing.T) {
		t.Setenv("XHS_SITE", "")
		writeCookies(t, intl)
		assert.Equal(t, "https://www.rednote.com/explore", WebURL("/explore"))
		assert.Equal(t, "https://creator.rednote.com/publish", CreatorURL("/publish"))
		assert.True(t, IsIntl())
	})

	t.Run("会话在 xiaohongshu.com 时走国内站", func(t *testing.T) {
		t.Setenv("XHS_SITE", "")
		writeCookies(t, cn)
		assert.Equal(t, "https://www.xiaohongshu.com/explore", WebURL("/explore"))
		assert.Equal(t, "https://creator.xiaohongshu.com/publish", CreatorURL("/publish"))
	})

	t.Run("没有 cookies 时默认国内站", func(t *testing.T) {
		t.Setenv("XHS_SITE", "")
		t.Setenv("COOKIES_PATH", filepath.Join(t.TempDir(), "missing.json"))
		assert.Equal(t, "www.xiaohongshu.com", WebHost())
	})

	t.Run("XHS_SITE 覆盖推断", func(t *testing.T) {
		writeCookies(t, intl)
		t.Setenv("XHS_SITE", "xiaohongshu")
		assert.Equal(t, "www.xiaohongshu.com", WebHost())
		writeCookies(t, cn)
		t.Setenv("XHS_SITE", "REDNOTE")
		assert.Equal(t, "www.rednote.com", WebHost())
	})
}
