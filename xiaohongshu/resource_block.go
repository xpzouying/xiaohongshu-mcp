package xiaohongshu

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// blockHeavyResources 拦掉图片、音视频、字体请求，返回停止函数。
//
// 读取类页面的数据全部取自 window.__INITIAL_STATE__，图片和视频的 URL 就在那段 JSON 里，
// 浏览器根本不需要真的把它们下载下来、解码、渲染。拦掉的收益是双份的：
//
//   - 省内存：实测单次 search_feeds 峰值 11 个 chrome 进程 / 1078MB，而容器上限只有
//     1200MB——一次调用就吃掉九成，并发上限因此是 1。图片解码是大头，压下来才谈得上并发。
//   - 省时间：视频不加载就不会播放，而视频播放正是详情页 DOM 持续抖动、等不到静止的
//     唯一原因（见 001 补丁），拦掉后不必再干等那 15 秒软超时。
//
// 只用于读取类流程（搜索/列表/详情）。发布类流程要真实上传和预览，不要用。
func blockHeavyResources(page *rod.Page) func() {
	router := page.HijackRequests()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		switch ctx.Request.Type() {
		case proto.NetworkResourceTypeImage,
			proto.NetworkResourceTypeMedia,
			proto.NetworkResourceTypeFont:
			// BlockedByClient 是浏览器插件拦广告时的常规做法，站点普遍容忍。
			ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
		default:
			ctx.ContinueRequest(&proto.FetchContinueRequest{})
		}
	})

	go router.Run()

	return func() {
		if err := router.Stop(); err != nil {
			logrus.Debugf("停止资源拦截器: %v", err)
		}
	}
}
