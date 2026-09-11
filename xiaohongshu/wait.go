package xiaohongshu

// 页面等待的统一策略。
//
// 小红书的页面几乎从不"安静"：懒加载图片、无限滚动占位、自动播放的视频，都会让 load
// 事件迟迟不触发、让 DOM 永远无法零变化。而 rod 的 MustWaitLoad / MustWaitStable /
// MustWaitDOMStable 自身不带上限（官方注释明确写着 "d is not the max wait timeout"），
// 唯一的退出路径是外层 page deadline——于是它们只能一路吃满那个 deadline 再 panic。
// 线上 3 天 59 次 context deadline exceeded 就是这么来的；deadline 长的地方
// （详情页 10 分钟、评论回复 5 分钟）表现为客户端干等几分钟的"卡死"。
//
// 收口成两条规矩：
//
//  1. "等页面安静"这类等待一律视为**尽力而为的缓冲**——给短上限，超时只告警不失败。
//     真正的就绪条件永远是它后面那句具体的元素查找或数据判断，页面静没静不决定结果。
//  2. 需要精确等待时，用 page.Wait(rod.Eval(...)) 直接等业务条件本身。
//     feed_detail.go 就是这么做的：等 __INITIAL_STATE__.note.noteDetailMap 落地，
//     而不是等 DOM 静止——同一条视频笔记详情因此从 21 秒降到 10 秒。
//
// 一句话：不要再裸用 Must*Wait* 系列。

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// softWaitBudget 缓冲等待的默认上限。
// 取 8 秒：实测图文详情页约 5 秒即可静止，够用；视频笔记永远静不下来，早点放行反而更好。
const softWaitBudget = 8 * time.Second

// softWaitLoad 等 load 事件，有上限、不 panic。what 用于日志定位是哪条链路。
func softWaitLoad(page *rod.Page, what string) {
	if err := page.Timeout(softWaitBudget).WaitLoad(); err != nil {
		logrus.Warnf("%s：未在 %s 内 load 完成（资源多时常见），继续", what, softWaitBudget)
	}
}

// softWaitStable 等页面稳定（load + 网络空闲 + DOM 零变化三者同时成立），有上限、不 panic。
// 这是最苛刻的一档，小红书页面基本达不成，所以更要有上限。
func softWaitStable(page *rod.Page, what string) {
	if err := page.Timeout(softWaitBudget).WaitStable(300 * time.Millisecond); err != nil {
		logrus.Warnf("%s：未在 %s 内稳定（懒加载页面常见），继续", what, softWaitBudget)
	}
}

// softWaitData 等某段业务数据就绪（通常是 __INITIAL_STATE__ 的某个字段注水完成），
// 有上限、不 panic。
//
// 这是三档里**最该优先用**的一档：等的是结果本身，而不是页面的外观状态。
//
// 尤其注意别只等 __INITIAL_STATE__ 这个壳——壳在页面初始化时就存在，具体数据要等接口
// 回来才填。"软超时 + 只检查壳"会在慢网络下提前放行，让后面一次性的提取拿到空值，
// 于是报成"没有结果"或"可能未登录"，比 panic 更难排查。传入的 js 必须判到真实的值。
func softWaitData(page *rod.Page, js string, budget time.Duration, what string) {
	if err := page.Timeout(budget).Wait(rod.Eval(js)); err != nil {
		logrus.Warnf("%s：数据未在 %s 内就绪，仍尝试解析", what, budget)
	}
}

// softWaitDOMStable 等 DOM 静止，有上限、不 panic。
func softWaitDOMStable(page *rod.Page, what string) {
	if err := page.Timeout(softWaitBudget).WaitDOMStable(time.Second, 0); err != nil {
		logrus.Warnf("%s：DOM 未在 %s 内静止（视频笔记常见），继续", what, softWaitBudget)
	}
}
