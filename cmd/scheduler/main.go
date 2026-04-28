package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/internal/pipeline"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})

	var keyword string
	var minLikes int
	var binPath string
	flag.StringVar(&keyword, "keyword", os.Getenv("SCHEDULER_KEYWORD"), "搜索关键词（优先级：命令行 > 环境变量 > keywords.txt）")
	flag.IntVar(&minLikes, "min-likes", 500, "最低点赞数")
	flag.StringVar(&binPath, "bin", os.Getenv("BROWSER_BIN_PATH"), "浏览器路径")
	flag.Parse()

	if keyword == "" {
		keyword = readKeywordFromFile()
	}
	if keyword == "" {
		logrus.Fatal("请通过 -keyword 或 SCHEDULER_KEYWORD 指定搜索关键词")
	}

	cfg := pipeline.Config{
		DMXAPIKey:      os.Getenv("DMXAPI_KEY"),
		FeishuWebhook:  os.Getenv("FEISHU_WEBHOOK_URL"),
		MinLikes:       minLikes,
		BrowserBinPath: binPath,
	}
	if cfg.DMXAPIKey == "" {
		logrus.Fatal("请设置环境变量 DMXAPI_KEY")
	}
	if cfg.FeishuWebhook == "" {
		logrus.Fatal("请设置环境变量 FEISHU_WEBHOOK_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	if err := pipeline.Run(ctx, cfg, keyword); err != nil {
		logrus.Fatalf("流水线执行失败: %v", err)
	}
	logrus.Info("完成！")
}

// readKeywordFromFile 从 keywords.txt 读取第一个非空行
func readKeywordFromFile() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "keywords.txt"))
	if err != nil {
		data, err = os.ReadFile("cmd/scheduler/keywords.txt")
		if err != nil {
			return ""
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
