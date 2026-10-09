package xiaohongshu

import (
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// waitLoadTolerant 等待页面 load 事件，超时或失败只告警、不中断流程。
//
// 小红书页面常挂有长连接或迟迟不完成的资源请求，load 事件在部分网络环境下
// 不会在合理时间内触发，但此时 DOM 早已可用；后续的元素查找本身自带轮询，
// 因此等不到 load 不应让整个工具调用超时失败。用法与 publish.go 中
// WaitLoad 出错仅告警的处理方式一致。
func waitLoadTolerant(page *rod.Page, d time.Duration) {
	if err := page.Timeout(d).WaitLoad(); err != nil {
		logrus.Warnf("等待 load 事件未完成（%s），继续执行: %v", d, err)
	}
}
