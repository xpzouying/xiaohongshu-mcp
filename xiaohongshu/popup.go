package xiaohongshu

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// dismissCookieConsent 处理海外用户访问时出现的 cookie 同意弹窗
// 非阻塞：如果没有弹窗则立即返回
func dismissCookieConsent(page *rod.Page) {
	// 检测 cookie 同意弹窗是否存在
	hasBanner, _, _ := page.Has("div.cookie-banner")
	if !hasBanner {
		return
	}

	has, btn, err := page.Has(".cookie-banner__btn--primary")
	if err != nil || !has {
		logrus.Debug("检测到 cookie 弹窗容器，但未找到接受按钮（selector 可能已变更）")
		return
	}

	logrus.Info("检测到 cookie 同意弹窗，自动点击接受")

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		logrus.Warnf("点击 cookie 同意按钮失败: %v，尝试移除弹窗", err)
		// 兜底：直接移除弹窗容器
		if hasBanner, banner, _ := page.Has("div.cookie-banner"); hasBanner {
			if err := banner.Remove(); err != nil {
				logrus.Warnf("移除 cookie 弹窗容器失败: %v", err)
			}
		}
	}
}
