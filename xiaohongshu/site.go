package xiaohongshu

import (
	"fmt"
	"os"
	"strings"
)

// webURL 返回当前站点的网页地址，fork 默认使用 RedNote。
func webURL(path string) string {
	if isRedNote() {
		return "https://www.rednote.com" + path
	}
	return "https://www.xiaohongshu.com" + path
}

// creatorURL 返回当前站点的创作者中心地址。
func creatorURL(path string) string {
	if isRedNote() {
		return "https://creator.rednote.com" + path
	}
	return "https://creator.xiaohongshu.com" + path
}

func isRedNote() bool {
	site := strings.TrimSpace(os.Getenv("XHS_SITE"))
	return site == "" || strings.EqualFold(site, "rednote")
}

// ValidateSite 防止拼错配置后静默切换账号站点。
func ValidateSite() error {
	site := strings.ToLower(strings.TrimSpace(os.Getenv("XHS_SITE")))
	switch site {
	case "", "rednote", "xiaohongshu":
		return nil
	default:
		return fmt.Errorf("不支持的 XHS_SITE %q；请使用 rednote 或 xiaohongshu", site)
	}
}
