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
		stdio    bool
	)
	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.StringVar(&port, "port", ":18060", "端口")
	flag.BoolVar(&stdio, "stdio", false, "使用 stdio 模式运行 MCP（不启动 HTTP 服务器）")
	flag.Parse()

	if len(binPath) == 0 {
		binPath = os.Getenv("ROD_BROWSER_BIN")
	}

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建应用服务器
	appServer := NewAppServer(xiaohongshuService)

	if stdio {
		if port != ":18060" {
			logrus.Warn("stdio 模式下 --port 参数将被忽略")
		}
		// 使用 stdio 模式运行 MCP
		if err := appServer.RunStdio(); err != nil {
			logrus.Fatalf("failed to run stdio server: %v", err)
		}
	} else {
		// 启动 HTTP 服务器
		if err := appServer.Start(port); err != nil {
			logrus.Fatalf("failed to run server: %v", err)
		}
	}
}
