package main

import (
	"flag"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func main() {
	var (
		headless bool
	)
	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.Parse()

	configs.InitHeadless(headless)

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService)
	if err := appServer.Start(":18060"); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
