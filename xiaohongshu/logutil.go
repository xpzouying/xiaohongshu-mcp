package xiaohongshu

import (
	"net/url"
	"regexp"
	"strings"
)

var xsecTokenQueryRe = regexp.MustCompile(`(?i)([?&]xsec_token=)[^&]*`)

// RedactURL 去掉日志里的 xsec_token 等敏感 query，保留可排障的路径。
func RedactURL(raw string) string {
	if raw == "" {
		return raw
	}
	if u, err := url.Parse(raw); err == nil && u.RawQuery != "" {
		q := u.Query()
		for _, k := range []string{"xsec_token", "xsecToken", "token"} {
			if q.Has(k) {
				q.Set(k, "***")
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	}
	return xsecTokenQueryRe.ReplaceAllString(raw, `${1}***`)
}

// RedactToken 日志里只留前后缀提示长度。
func RedactToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return ""
	}
	if len(tok) <= 8 {
		return "***"
	}
	return tok[:4] + "***" + tok[len(tok)-2:]
}
