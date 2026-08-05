package browser

import (
	"encoding/json"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// cookieWire 同时兼容两种 cookie 线格式：
//   - CDP（go-rod/proto.NetworkCookie）：sameParty/sourcePort/failed 等字段
//   - Playwright 原生（Cookie/OptionalCookie）
//
// cookies.json 历史上由 CDP GetCookies 落盘，迁入 Playwright 后须无感读取；
// 保存时再统一写回 CDP 形状，保持文件格式对外稳定。
type cookieWire struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	Size     int     `json:"size,omitempty"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	Session  bool    `json:"session,omitempty"`
	SameSite string  `json:"sameSite,omitempty"`

	// CDP 专有，透传保留、不参与注入。partitionKey 在部分响应里是对象，
	// sourcePort/sourceScheme 也可能缺失或类型不一，统一用 json.RawMessage 透传，
	// 避免单条字段形态不符导致整份数组反序列化失败。
	Priority     string          `json:"priority,omitempty"`
	SameParty    bool            `json:"sameParty,omitempty"`
	SourceScheme string          `json:"sourceScheme,omitempty"`
	SourcePort   json.RawMessage `json:"sourcePort,omitempty"`
	PartitionKey json.RawMessage `json:"partitionKey,omitempty"`
	Failed       bool            `json:"failed,omitempty"`

	// Playwright 注入时可用 URL 代替 domain/path
	URL string `json:"url,omitempty"`
}

// cookiesToOptional 把 cookie 文件（CDP 或 Playwright 线格式）转为注入上下文
// 所需的 []playwright.OptionalCookie。无法解析的条目跳过，不阻断启动。
//
// 同时兼容 cookies.json 的两种外层形态：
//   - v1：裸 cookie 数组
//   - v2：{version, seed, cookies:[...]} 信封（见 cookies/cookies.go）
func cookiesToOptional(data []byte) []playwright.OptionalCookie {
	var wires []cookieWire
	if err := json.Unmarshal(data, &wires); err != nil {
		// 裸数组解析失败：按 v2 信封再试一次
		var env struct {
			Cookies json.RawMessage `json:"cookies"`
		}
		if err2 := json.Unmarshal(data, &env); err2 != nil || len(env.Cookies) == 0 {
			return nil
		}
		if err3 := json.Unmarshal(env.Cookies, &wires); err3 != nil {
			return nil
		}
	}
	out := make([]playwright.OptionalCookie, 0, len(wires))
	for _, w := range wires {
		if w.Name == "" {
			continue
		}
		oc := playwright.OptionalCookie{
			Name:  w.Name,
			Value: w.Value,
		}
		if w.Domain != "" {
			oc.Domain = playwright.String(w.Domain)
		}
		if w.Path != "" {
			oc.Path = playwright.String(w.Path)
		} else {
			oc.Path = playwright.String("/")
		}
		if w.URL != "" {
			oc.URL = playwright.String(w.URL)
		}
		if w.Expires != 0 {
			oc.Expires = playwright.Float(w.Expires)
		}
		oc.HttpOnly = playwright.Bool(w.HTTPOnly)
		// 强制 Secure：小红书全站 HTTPS。历史 CDP 文件里部分 cookie
		// sourceScheme=NonSecure（secure=false），Chromium 对此宽容、仍会在
		// HTTPS 请求中带上；而 Juggler/Firefox 会拒绝把 insecure cookie 注入
		// 到 https 域，导致整个 AddCookies 批量失败、登录态丢失。
		oc.Secure = playwright.Bool(true)
		if ss := normalizeSameSite(w.SameSite); ss != nil {
			oc.SameSite = ss
		}
		out = append(out, oc)
	}
	return out
}

// cookiesFromPlaywright 把上下文导出的 Cookie 转回 CDP 线格式落盘，
// 与历史 cookies.json 保持一致。
func cookiesFromPlaywright(cks []playwright.Cookie) []byte {
	wires := make([]cookieWire, 0, len(cks))
	for _, c := range cks {
		w := cookieWire{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
			Session:  c.Expires == 0 || c.Expires == -1,
		}
		if c.SameSite != nil {
			w.SameSite = canonicalSameSite(string(*c.SameSite))
		}
		wires = append(wires, w)
	}
	data, err := json.Marshal(wires)
	if err != nil {
		return []byte("[]")
	}
	return data
}

// canonicalSameSite 归一化 SameSite 文本，保证落盘格式稳定（首字母大写）。
func canonicalSameSite(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict":
		return "Strict"
	case "none":
		return "None"
	default:
		return "Lax"
	}
}

func normalizeSameSite(s string) *playwright.SameSiteAttribute {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict":
		return playwright.SameSiteAttributeStrict
	case "none":
		return playwright.SameSiteAttributeNone
	case "lax", "":
		return playwright.SameSiteAttributeLax
	default:
		return playwright.SameSiteAttributeLax
	}
}
