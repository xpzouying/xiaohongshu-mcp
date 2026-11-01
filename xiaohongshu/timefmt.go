package xiaohongshu

import "time"

// cstZone 小红书是国内站点，时间戳一律按东八区渲染。
//
// 用 FixedZone 而不是 LoadLocation("Asia/Shanghai")：后者依赖系统 tzdata，
// 精简镜像（scratch、未装 tzdata 的 alpine）里会返回错误，稍不留神就静默退化成
// UTC，整体差 8 小时——而这种偏差在返回体里看不出异常，只会让调用方算错日期。
var cstZone = time.FixedZone("CST", 8*60*60)

// msTimestampMin 秒与毫秒时间戳的分界。
//
// 1e12 当秒是公元 33658 年，当毫秒是 2001-09-09，现代时间戳落在哪一侧都没有歧义。
// 之所以自适应而不是写死单位：笔记与评论的时间戳是 13 位毫秒，
// 而通知列表的 time 单位站点未作保证，写死一种会在另一处错三个数量级。
const msTimestampMin int64 = 1e12

// timestampText 把站点给的 Unix 时间戳渲染成东八区的 "2006-01-02 15:04"。
//
// 非正数（字段缺失时的零值）返回空串，配合 tag 上的 omitempty 直接不落进返回体。
// 否则调用方会读到 "1970-01-01 08:00" 并当成真实发布日期。
func timestampText(ts int64) string {
	if ts <= 0 {
		return ""
	}

	t := time.Unix(ts, 0)
	if ts >= msTimestampMin {
		t = time.UnixMilli(ts)
	}
	return t.In(cstZone).Format("2006-01-02 15:04")
}

// fillCommentTimeText 递归回填评论及其子评论的可读时间。
//
// 必须按下标改写：for range 拿到的是 Comment 副本，写进副本等于什么都没做。
func fillCommentTimeText(comments []Comment) {
	for i := range comments {
		comments[i].CreateTimeText = timestampText(comments[i].CreateTime)
		fillCommentTimeText(comments[i].SubComments)
	}
}
