package xiaohongshu

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

// applySiteLocale 对海外站页面强制中文 locale。
// rednote.com 与国内站共用 xhs-pc-web 前端,切中文后既有的中文文本选择器
// (发布页 TAB、正文 placeholder 等)可直接复用。
func applySiteLocale(page *rod.Page) {
	if !Site().ForceZhCN {
		return
	}

	_ = proto.NetworkSetExtraHTTPHeaders{Headers: proto.NetworkHeaders{
		"Accept-Language": gson.New("zh-CN,zh;q=0.9,en;q=0.8"),
	}}.Call(page)

	_ = (&proto.EmulationSetLocaleOverride{Locale: "zh-CN"}).Call(page)
}
