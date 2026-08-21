package xiaohongshu

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// feedReadyTimeout 等待详情页出结果的上限。
//
// 定这么短是因为等待从 load 事件之后才开始，只覆盖 SPA 的水合延迟：实测正常笔记
// （含视频帖）在 load 那一刻容器就已存在，等待接近零成本。而笔记不存在时页面既不
// 渲染下面任何一个容器、又会在几秒后跳回首页信息流（首页容器正常笔记页上也有，
// 没法拿来区分），只能靠超时兜底——这个上限直接决定失效笔记要白等多久，
// 放大到十几秒就会比改动前更慢。超时不算失败，交给后续选择器自己轮询。
const feedReadyTimeout = 8 * time.Second

// feedReadySelectors 详情页导航完成的判据。
// 前两个是正常笔记的内容容器（点赞收藏栏 / 笔记滚动容器）；后四个是
// checkPageAccessible 认的错误容器，笔记私密或违规时出现，一并等上是为了这类
// 情况不用等满 feedReadyTimeout。具体是哪种由随后的 checkPageAccessible 判定，
// 这里只负责「页面有结果了」。
const feedReadySelectors = ".interact-container, .note-scroller, " +
	".access-wrapper, .error-wrapper, .not-found-wrapper, .blocked-wrapper"

// waitFeedPageReady 等待 feed 详情页可用。
//
// 背景：视频帖的播放器（进度条等 UI）会持续改动 DOM，导致 rod 的
// MustWaitDOMStable 在超时内永远等不到「DOM 稳定」，点赞/评论/详情全部卡死
// （2026-08-12 实测：图文帖正常，所有视频帖 MustWaitDOMStable 60s 超时）。
// 这里改为等 load 事件 + 轮询关键容器，不再要求整页 DOM 静止。
func waitFeedPageReady(page *rod.Page) {
	page.MustWaitLoad()

	if _, err := page.Timeout(feedReadyTimeout).Element(feedReadySelectors); err != nil {
		// 没等到不代表页面不可用，后续 Element 各自带轮询，继续往下走。
		// 用 Warn 不用 Debug：本项目没配日志级别，Debug 打不出来，
		// 而这是排查「页面结构变了」的第一手线索，丢了就只剩后面一串莫名的超时。
		logrus.Warnf("详情页关键容器未在 %s 内出现，继续尝试: %v", feedReadyTimeout, err)
	}

	pauseVideos(page)
}

// pauseVideos 暂停页面内所有视频并关掉自动播放，减少播放器造成的持续 DOM 变动。
// go-rod 没有对应原语，只能走 Eval；图文帖没有 video，等同空操作。
func pauseVideos(page *rod.Page) {
	_, _ = page.Eval(`() => {
		document.querySelectorAll('video').forEach(v => {
			try { v.pause(); } catch (e) {}
			v.autoplay = false;
			v.muted = true;
		});
	}`)
}
