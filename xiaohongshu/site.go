package xiaohongshu

import "fmt"

// 站点标识。
const (
	SiteXiaohongshu = "xiaohongshu" // 国内站 xiaohongshu.com
	SiteRednote     = "rednote"     // 海外站 rednote.com(与国内站同一套 xhs-pc-web 前端,账号体系独立)
)

// SiteConfig 站点配置:同一套自动化流程支持国内站与海外站。
type SiteConfig struct {
	Name        string // 站点标识
	Base        string // 主站根,用于拼接笔记/用户/搜索链接
	Home        string // 首页(explore),登录与导航入口
	PublishURL  string // 创作平台图文/视频发布页
	LoggedInSel string // 已登录判定:侧栏用户入口(class 选择器,语言无关)
	ForceZhCN   bool   // 页面强制中文 locale(海外站默认英文,中文化后文本选择器可复用)
}

var sites = map[string]SiteConfig{
	SiteXiaohongshu: {
		Name:        SiteXiaohongshu,
		Base:        "https://www.xiaohongshu.com",
		Home:        "https://www.xiaohongshu.com/explore",
		PublishURL:  "https://creator.xiaohongshu.com/publish/publish?source=official",
		LoggedInSel: `.main-container .user .link-wrapper .channel`,
		ForceZhCN:   false,
	},
	SiteRednote: {
		Name:        SiteRednote,
		Base:        "https://www.rednote.com",
		Home:        "https://www.rednote.com/explore",
		PublishURL:  "https://creator.rednote.com/publish/publish?source=official",
		LoggedInSel: `.main-container .user .link-wrapper .channel`,
		ForceZhCN:   true,
	},
}

// 当前站点,默认国内站保持向后兼容;启动时通过 SetSite 切换。
var currentSite = sites[SiteXiaohongshu]

// SetSite 设置当前站点,进程启动时调用一次。
func SetSite(name string) error {
	s, ok := sites[name]
	if !ok {
		return fmt.Errorf("未知站点: %q(支持 %s / %s)", name, SiteXiaohongshu, SiteRednote)
	}
	currentSite = s
	return nil
}

// Site 返回当前站点配置。
func Site() SiteConfig { return currentSite }
