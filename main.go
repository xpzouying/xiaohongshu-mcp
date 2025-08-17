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

	if err := startServer(); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
