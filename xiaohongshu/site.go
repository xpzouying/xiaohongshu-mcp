package xiaohongshu

import (
	"os"
	"strings"
)

// webURL 返回当前站点的网页地址。默认保持兼容小红书国内站。
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

func creatorLoginURL() string {
	if isRedNote() {
		return creatorURL("/login?source=official")
	}
	return creatorURL("/?source=official")
}

func isRedNote() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("XHS_SITE")), "rednote")
}
