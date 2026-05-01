package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func main() {
	var (
		headless bool
		binPath  string // 浏览器二进制文件路径
		port     string
		domain   string // 站点域名
	)
	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.StringVar(&port, "port", ":18060", "端口")
	flag.StringVar(&domain, "domain", "", "站点域名，默认 www.xiaohongshu.com，国际版用 www.rednote.com")
	flag.Parse()

	if len(binPath) == 0 {
		binPath = os.Getenv("ROD_BROWSER_BIN")
	}

	// 支持环境变量设置域名
	if len(domain) == 0 {
		domain = os.Getenv("XHS_DOMAIN")
	}

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)
	if len(domain) > 0 {
		configs.SetSiteDomain(domain)
	}

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService)
	if err := appServer.Start(port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
