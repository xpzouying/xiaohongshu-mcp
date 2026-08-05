package main

import (
	"context"
	"flag"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// 扫码登录（有头）：登录成功后把 cookie 落盘到 cookies.json，供常驻服务复用。
func main() {
	flag.Parse()

	store := cookies.NewLoadCookie(cookies.GetCookiesFilePath())

	b, err := browser.NewBrowser(false,
		browser.WithFingerprintSeed(configs.ResolveFingerprintSeed(store)),
		browser.WithProxy(configs.ProxyFromEnv()),
	)
	if err != nil {
		logrus.Fatalf("start camoufox failed: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		logrus.Fatalf("new page failed: %v", err)
	}
	defer page.Close()

	action := xiaohongshu.NewLogin(page)

	status, err := action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status: %v", err)
	}
	logrus.Infof("当前登录状态: %v", status)
	if status {
		return
	}

	logrus.Info("开始登录流程...")
	if err = action.Login(context.Background()); err != nil {
		logrus.Fatalf("登录失败: %v", err)
	}

	// 登录成功，导出 cookie 落盘
	data, err := b.Cookies()
	if err != nil {
		logrus.Fatalf("read cookies failed: %v", err)
	}
	if err := store.SaveCookies(data); err != nil {
		logrus.Fatalf("failed to save cookies: %v", err)
	}

	// 再次确认登录状态
	status, err = action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status after login: %v", err)
	}
	if status {
		logrus.Info("登录成功！")
	} else {
		logrus.Error("登录流程完成但仍未登录")
	}
}
