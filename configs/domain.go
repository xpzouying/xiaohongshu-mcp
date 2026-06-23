package configs

import "os"

const (
	// 默认域名
	defaultDomain = "www.xiaohongshu.com"

	// 创作者平台域名
	defaultCreatorDomain = "creator.xiaohongshu.com"
)

var siteDomain = ""

// InitDomain 初始化站点域名。
// 支持通过环境变量 XHS_DOMAIN 或启动参数设置。
// 例如: "www.rednote.com" (海外用户)
func InitDomain(domain string) {
	siteDomain = domain
}

// GetSiteDomain 获取站点域名。
// 优先级: 启动参数 > 环境变量 > 默认值
func GetSiteDomain() string {
	if siteDomain != "" {
		return siteDomain
	}
	if d := os.Getenv("XHS_DOMAIN"); d != "" {
		return d
	}
	return defaultDomain
}

// GetCreatorDomain 获取创作者平台域名。
func GetCreatorDomain() string {
	domain := GetSiteDomain()
	if domain == "www.rednote.com" {
		return "creator.rednote.com"
	}
	return defaultCreatorDomain
}

// GetBaseURL 获取站点基础URL。
func GetBaseURL() string {
	return "https://" + GetSiteDomain()
}
