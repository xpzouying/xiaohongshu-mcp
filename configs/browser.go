package configs

import "strings"

var (
	useHeadless = true

	binPath = ""

	// 站点域名，默认为 xiaohongshu.com，可切换为 rednote.com（国际版）
	siteDomain    = "www.xiaohongshu.com"
	creatorDomain = "creator.xiaohongshu.com"
)

func InitHeadless(h bool) {
	useHeadless = h
}

// IsHeadless 是否无头模式。
func IsHeadless() bool {
	return useHeadless
}

func SetBinPath(b string) {
	binPath = b
}

func GetBinPath() string {
	return binPath
}

// SetSiteDomain 设置站点域名（如 www.rednote.com）
func SetSiteDomain(d string) {
	siteDomain = d
	// 自动推断 creator 域名
	if strings.Contains(d, "rednote.com") {
		creatorDomain = "creator.rednote.com"
	} else {
		creatorDomain = "creator.xiaohongshu.com"
	}
}

// GetSiteDomain 获取站点域名
func GetSiteDomain() string {
	return siteDomain
}

// GetCreatorDomain 获取创作者平台域名
func GetCreatorDomain() string {
	return creatorDomain
}

// ExploreURL 获取首页 explore 地址
func ExploreURL() string {
	return "https://" + siteDomain + "/explore"
}

// HomeURL 获取首页地址
func HomeURL() string {
	return "https://" + siteDomain
}

// SearchURL 获取搜索结果地址前缀
func SearchURL() string {
	return "https://" + siteDomain + "/search_result"
}

// FeedDetailURL 获取帖子详情地址
func FeedDetailURL(feedID, xsecToken string) string {
	return "https://" + siteDomain + "/explore/" + feedID + "?xsec_token=" + xsecToken + "&xsec_source=pc_feed"
}

// UserProfileURL 获取用户主页地址
func UserProfileURL(userID, xsecToken string) string {
	return "https://" + siteDomain + "/user/profile/" + userID + "?xsec_token=" + xsecToken + "&xsec_source=pc_note"
}

// PublishURL 获取发布页地址
func PublishURL() string {
	return "https://" + creatorDomain + "/publish/publish?source=official"
}
