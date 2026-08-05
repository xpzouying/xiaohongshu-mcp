package main

import (
	"flag"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// version 构建版本号，发布时通过 -ldflags "-X main.version=vX.Y.Z" 注入。
var version = "dev"

func main() {
	var (
		headless bool
		port     string
	)
	flag.BoolVar(&headless, "headless", false, "是否无头模式（默认有头，降低自动化特征）")
	flag.StringVar(&port, "port", "127.0.0.1:18060", "监听地址，默认仅回环；全网卡用 --port :18060")
	flag.Parse()

	logrus.Infof("xiaohongshu-mcp version: %s", version)

	// 浏览器固定为 Camoufox：自构建、固定版本、可校验，运行时不下载。
	binPath, binSource, err := browser.ResolveCamoufoxBin()
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	configs.SetCamoufoxBin(binPath)
	logrus.Infof("using camoufox binary: %s (source=%s, version=%s)", binPath, binSource, browser.CamoufoxVersion)

	configs.InitHeadless(headless)
	// 入口层解析 seed 与代理，经 configs 透传给浏览器工厂。
	// seed 取值：环境变量 > 会话文件 > 新生成并写回，保证同一账号每次启动指纹一致。
	configs.SetFingerprintSeed(configs.ResolveFingerprintSeed(
		cookies.NewLoadCookie(cookies.GetCookiesFilePath())))
	configs.SetProxy(configs.ProxyFromEnv())

	logrus.Infof("browser session: camoufox shared+serial lease; risk_streak_limit=%d", riskStreakLimit())

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器（常驻浏览器在首次请求时懒创建，停服时 Close）
	appServer := NewAppServer(xiaohongshuService)
	if err := appServer.Start(port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
