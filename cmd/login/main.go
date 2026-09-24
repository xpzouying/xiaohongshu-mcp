package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func main() {
	flag.Parse()

	// 登录的时候，需要界面，所以不能无头模式。
	// 登录与后续运行共用同一个 seed：首次登录生成并写入会话文件，之后一直复用。
	store := cookies.NewLoadCookie(cookies.GetCookiesFilePath())

	b := browser.NewBrowser(false,
		browser.WithFingerprintSeed(configs.ResolveFingerprintSeed(store)),
		browser.WithProxy(configs.ProxyFromEnv()),
	)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewLogin(page)

	status, err := action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status: %v", err)
	}

	logrus.Infof("当前登录状态: %v", status)

	if !status {
		// 开始登录流程
		logrus.Info("开始登录流程...")
		if err = action.Login(context.Background()); err != nil {
			logrus.Fatalf("登录失败: %v", err)
		}
	}

	// 登录只走了主站，创作平台是另一套 cookie。不先访问一次让它签发，
	// 发布时上传图片会被踢回创作平台首页。已登录时也要补，所以不能提前 return。
	if err := warmupCreator(page); err != nil {
		logrus.Warnf("预热创作平台失败: %v，仍然保存当前 cookies", err)
	}

	if err := saveCookies(page); err != nil {
		logrus.Fatalf("failed to save cookies: %v", err)
	}

	// 再次检查登录状态确认成功
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

const (
	// 直接打登录页。发布页不需要登录态也能加载，导航过去不会触发任何认证。
	creatorLoginURL = "https://creator.xiaohongshu.com/login?source=official"
	// 创作平台要单独扫码，留够人操作的时间
	creatorLoginWait = 3 * time.Minute
)

// warmupCreator 走一遍创作平台的登录，拿到 creator 域下的登录凭据。
func warmupCreator(page *rod.Page) error {
	pp := page.Timeout(creatorLoginWait + 30*time.Second)

	if err := pp.Navigate(creatorLoginURL); err != nil {
		return err
	}
	if err := pp.WaitLoad(); err != nil {
		return err
	}

	logrus.Warnf("如果窗口里出现二维码，请扫码登录创作平台（最多等 %v）", creatorLoginWait)

	deadline := time.Now().Add(creatorLoginWait)
	lastURL := ""

	for {
		info, err := pp.Info()
		if err != nil {
			return err
		}

		if info.URL != lastURL {
			logrus.Infof("创作平台当前页面: %s", info.URL)
			lastURL = info.URL
		}

		// 离开登录页即认为认证完成
		if !strings.Contains(info.URL, "/login") {
			time.Sleep(3 * time.Second) // 等 cookie 写完
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("等待创作平台登录超时，仍停在 %s", info.URL)
		}

		time.Sleep(2 * time.Second)
	}
}

// hasCreatorAuth 判断有没有拿到创作平台的登录凭据。
// 注意凭据是发在父域 .xiaohongshu.com 上的，creator 字样出现在 name 里而不是 domain，
// 按 domain 找只会捞到 acw_tc 这个 WAF 探针。
func hasCreatorAuth(cks []*proto.NetworkCookie) bool {
	for _, ck := range cks {
		if strings.HasPrefix(ck.Name, "access-token-creator.") ||
			ck.Name == "galaxy_creator_session_id" {
			return true
		}
	}
	return false
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	if hasCreatorAuth(cks) {
		logrus.Infof("保存 cookies: 共 %d 个，创作平台登录态已拿到", len(cks))
	} else {
		logrus.Warnf("保存 cookies: 共 %d 个，但没拿到创作平台登录态，发布图文会失败", len(cks))
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}
