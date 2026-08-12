package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

// version 构建版本号，发布时通过 -ldflags "-X main.version=vX.Y.Z" 注入。
var version = "dev"

type startupConfig struct {
	headless bool
	port     string
	token    string
}

func parseStartupConfig(flagSet *flag.FlagSet, args []string, getenv func(string) string) (startupConfig, error) {
	var config startupConfig
	flagSet.BoolVar(&config.headless, "headless", true, "是否无头模式")
	flagSet.StringVar(&config.port, "port", ":18060", "端口")
	flagSet.StringVar(&config.token, "token", "", "鉴权 Token，留空则关闭鉴权")

	if err := flagSet.Parse(args); err != nil {
		return startupConfig{}, err
	}

	tokenSet := false
	flagSet.Visit(func(parsedFlag *flag.Flag) {
		if parsedFlag.Name == "token" {
			tokenSet = true
		}
	})
	if !tokenSet {
		config.token = getenv("AUTH_TOKEN")
	}

	return config, nil
}

func main() {
	config, err := parseStartupConfig(flag.CommandLine, os.Args[1:], os.Getenv)
	if err != nil {
		return
	}

	logrus.Infof("xiaohongshu-mcp version: %s", version)

	// 只用内置浏览器。启动时就备好，缺它直接退出，不拖到第一个请求才失败。
	binPath, err := browser.EnsureBrowser()
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	logrus.Infof("using browser binary: %s", binPath)

	configs.InitHeadless(config.headless)
	// 入口层解析出 seed 和代理，经 configs 透传给浏览器工厂。
	// seed 取值：环境变量 > 会话文件 > 新生成并写回，保证同一账号每次启动一致。
	configs.SetFingerprintSeed(configs.ResolveFingerprintSeed(
		cookies.NewLoadCookie(cookies.GetCookiesFilePath())))
	configs.SetProxy(configs.ProxyFromEnv())

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService, config.token)
	if err := appServer.Start(config.port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
