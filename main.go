package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func main() {
	var (
		headless bool
		binPath  string // 浏览器二进制文件路径
		port     string
	)
	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.StringVar(&port, "port", ":18060", "端口")
	flag.Parse()

	if len(binPath) == 0 {
		binPath = os.Getenv("ROD_BROWSER_BIN")
	}
	// 未显式指定浏览器：自动准备内置浏览器，失败即退出。
	if binPath == "" {
		bin, err := browser.EnsureBrowser()
		if err != nil {
			logrus.Fatalf("%v", err)
		}
		binPath = bin
	}
	logrus.Infof("using browser binary: %s", binPath)

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)
	// 入口层读 env、解析成固定指纹 seed 和代理，经 configs 透传给浏览器工厂。
	configs.SetFingerprintSeed(configs.FingerprintSeedFromEnv())
	configs.SetProxy(configs.ProxyFromEnv())

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService)
	if err := appServer.Start(port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
