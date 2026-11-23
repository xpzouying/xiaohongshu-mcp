package configs

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// 主站有两个域名：国内 www.xiaohongshu.com；海外 IP 登录后会话落在 www.rednote.com。
// 会话在哪个域名，就得访问哪个域名，否则页面是未登录状态（例如搜索页只显示登录框）。
const (
	hostCN   = "www.xiaohongshu.com"
	hostIntl = "www.rednote.com"

	creatorHostCN   = "creator.xiaohongshu.com"
	creatorHostIntl = "creator.rednote.com"
)

// WebURL 返回主站上 path 对应的完整地址，path 以 / 开头。
func WebURL(path string) string {
	return "https://" + WebHost() + path
}

// IsIntl 当前是否走海外站 rednote.com。
func IsIntl() bool {
	return WebHost() == hostIntl
}

// CreatorURL 返回创作者中心上 path 对应的完整地址，域名跟随主站。
func CreatorURL(path string) string {
	if IsIntl() {
		return "https://" + creatorHostIntl + path
	}
	return "https://" + creatorHostCN + path
}

// WebHost 返回当前应访问的主站域名。
// XHS_SITE=rednote|xiaohongshu 可强制指定；未设时按 cookies 里 web_session 的域名推断。
// 每次调用都重新读取，进程内扫码登录后能立刻切换。
func WebHost() string {
	switch strings.ToLower(os.Getenv("XHS_SITE")) {
	case "rednote":
		return hostIntl
	case "xiaohongshu":
		return hostCN
	}
	data, err := cookies.NewLoadCookie(cookies.GetCookiesFilePath()).LoadCookies()
	if err == nil && sessionOnRednote(data) {
		return hostIntl
	}
	return hostCN
}

// sessionOnRednote 判断登录会话 cookie 是否在 rednote.com 域名下。
func sessionOnRednote(data []byte) bool {
	var cks []struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
	}
	if json.Unmarshal(data, &cks) != nil {
		return false
	}
	for _, c := range cks {
		if c.Name == "web_session" && strings.HasSuffix(c.Domain, "rednote.com") {
			return true
		}
	}
	return false
}
