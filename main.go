package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/account"
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
	if binPath != "" {
		logrus.Infof("using browser binary: %s", binPath)
	} else {
		logrus.Infof("browser binary is not configured; rod will auto-detect or download Chromium")
	}

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()
	dataDir := os.Getenv("XHS_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(".", "data")
	}
	accountRegistry, err := account.NewFileRegistry(dataDir)
	if err != nil {
		logrus.Fatalf("failed to initialize account registry: %v", err)
	}
	cookieStore, err := account.NewFileCookieStore(dataDir)
	if err != nil {
		logrus.Fatalf("failed to initialize account cookie store: %v", err)
	}
	maxConcurrency := 2
	if value := os.Getenv("XHS_MAX_ACCOUNT_CONCURRENCY"); value != "" {
		maxConcurrency, err = strconv.Atoi(value)
		if err != nil {
			logrus.Fatalf("invalid XHS_MAX_ACCOUNT_CONCURRENCY: %v", err)
		}
	}
	locks, err := account.NewLockManager(maxConcurrency)
	if err != nil {
		logrus.Fatalf("failed to initialize account locks: %v", err)
	}
	accountManager := account.NewAccountManager(accountRegistry, locks, newAccountBrowserFactory(cookieStore, newBrowserWithAccountCookie))

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService, accountRegistry, accountManager)
	if err := appServer.Start(port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
