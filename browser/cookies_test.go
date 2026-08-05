package browser

import (
	"encoding/json"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 历史 cookies.json 是 CDP（go-rod/proto.NetworkCookie）线格式。
// cookies.json 线上是 v2 信封格式 {version,seed,cookies:[...]}，必须能解出内层数组。
// 真实 CDP 导出里 partitionKey 是对象、sourcePort 形态不一，
// 这些字段必须原样透传而不让整份数组反序列化失败（线上 cookies.json 即如此）。
func TestCookiesToOptional_PartitionKeyObject(t *testing.T) {
	raw := `[
		{"name":"abRequestId","value":"v","domain":".xiaohongshu.com","path":"/","expires":1814856565.328461,"secure":false,"sourcePort":80,"sourceScheme":"NonSecure"},
		{"name":"web_session","value":"abc","domain":".xiaohongshu.com","path":"/","expires":1815245084.024262,"secure":true,"httpOnly":true,"sameSite":"None","partitionKey":{"topLevelSite":"https://xiaohongshu.com"},"sourcePort":443}
	]`
	ocs := cookiesToOptional([]byte(raw))
	require.Len(t, ocs, 2)
	assert.Equal(t, "abRequestId", ocs[0].Name)
	assert.Equal(t, "web_session", ocs[1].Name)
	assert.Equal(t, playwright.SameSiteAttributeNone, ocs[1].SameSite)
}

func TestCookiesToOptional_V2Envelope(t *testing.T) {
	v2 := `{
		"version": 2,
		"seed": 20260805,
		"saved_at": "2026-08-05T00:00:00Z",
		"cookies": [
			{"name":"web_session","value":"abc","domain":".xiaohongshu.com","path":"/","expires":1815245084.024262,"httpOnly":true,"secure":true,"sameSite":"None"},
			{"name":"a1","value":"x","domain":".xiaohongshu.com","path":"/","expires":1814856566,"secure":false}
		]
	}`
	ocs := cookiesToOptional([]byte(v2))
	require.Len(t, ocs, 2)
	assert.Equal(t, "web_session", ocs[0].Name)
	require.NotNil(t, ocs[0].SameSite)
	assert.Equal(t, playwright.SameSiteAttributeNone, ocs[0].SameSite)
	// secure 一律强制 true（见 cookies.go 注释）
	require.NotNil(t, ocs[1].Secure)
	assert.True(t, *ocs[1].Secure)
}

func TestCookiesToOptional_CDPLegacy(t *testing.T) {
	cdp := `[
		{
			"name": "web_session",
			"value": "abc123",
			"domain": ".xiaohongshu.com",
			"path": "/",
			"expires": 1893456000,
			"size": 22,
			"httpOnly": true,
			"secure": true,
			"session": false,
			"sameSite": "Lax",
			"priority": "Medium",
			"sameParty": false,
			"sourceScheme": "Secure",
			"sourcePort": 443
		}
	]`
	ocs := cookiesToOptional([]byte(cdp))
	require.Len(t, ocs, 1)
	c := ocs[0]
	assert.Equal(t, "web_session", c.Name)
	assert.Equal(t, "abc123", c.Value)
	require.NotNil(t, c.Domain)
	assert.Equal(t, ".xiaohongshu.com", *c.Domain)
	require.NotNil(t, c.Path)
	assert.Equal(t, "/", *c.Path)
	require.NotNil(t, c.Expires)
	assert.Equal(t, 1893456000.0, *c.Expires)
	require.NotNil(t, c.HttpOnly)
	assert.True(t, *c.HttpOnly)
	require.NotNil(t, c.SameSite)
	assert.Equal(t, playwright.SameSiteAttributeLax, c.SameSite)
}

func TestCookiesToOptional_SameSiteVariants(t *testing.T) {
	cases := map[string]*playwright.SameSiteAttribute{
		"Strict":  playwright.SameSiteAttributeStrict,
		"strict":  playwright.SameSiteAttributeStrict,
		"None":    playwright.SameSiteAttributeNone,
		"Lax":     playwright.SameSiteAttributeLax,
		"":        playwright.SameSiteAttributeLax, // 缺省归一化为 Lax
		"unknown": playwright.SameSiteAttributeLax, // 未识别值兜底 Lax
	}
	for in, want := range cases {
		got := normalizeSameSite(in)
		require.NotNil(t, got, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}
}

func TestCookiesToOptional_SkipsNamelessAndGarbage(t *testing.T) {
	assert.Nil(t, cookiesToOptional([]byte("not json")))
	assert.Empty(t, cookiesToOptional([]byte(`[{"value":"x"}]`))) // 无 name 跳过
}

// 保存走 CDP 线格式，保证与历史 cookies.json 对外稳定、可被重新读取。
func TestCookiesRoundTrip(t *testing.T) {
	src := []playwright.Cookie{
		{
			Name:     "a1",
			Value:    "v1",
			Domain:   ".xiaohongshu.com",
			Path:     "/",
			Expires:  1893456000,
			HttpOnly: true,
			Secure:   true,
			SameSite: playwright.SameSiteAttributeNone,
		},
	}
	data := cookiesFromPlaywright(src)

	// 序列化结果应能再次被 cookiesToOptional 读取
	ocs := cookiesToOptional(data)
	require.Len(t, ocs, 1)
	assert.Equal(t, "a1", ocs[0].Name)
	assert.Equal(t, "v1", ocs[0].Value)
	require.NotNil(t, ocs[0].SameSite)
	assert.Equal(t, playwright.SameSiteAttributeNone, ocs[0].SameSite)

	// 线格式仍含 CDP 风格字段名
	var raw []map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, raw, 1)
	assert.Contains(t, raw[0], "sameSite")
	assert.Contains(t, raw[0], "httpOnly")
}
