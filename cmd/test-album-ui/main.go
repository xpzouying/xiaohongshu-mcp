package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func init() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	fmt.Println("=== 专辑创建 UI 自动化测试 ===")
	fmt.Println()

	// 1. 启动浏览器（headless 模式，因为服务器没有 X display）
	fmt.Println("🚀 启动浏览器 (headless)...")
	b := browser.NewBrowser(true, browser.WithBinPath(configs.GetBinPath()))
	defer b.Close()

	// 2. 创建页面
	fmt.Println("📄 创建页面...")
	page := b.NewPage()
	defer page.Close()

	// 3. 创建 UI 服务（headless 模式）
	uiService := xiaohongshu.NewAlbumUIService(page, true)

	// 4. 测试创建专辑
	testAlbumName := fmt.Sprintf("测试专辑_%s", time.Now().Format("150405"))
	fmt.Printf("\n📝 创建专辑: %s\n", testAlbumName)
	fmt.Println("  （浏览器窗口会打开，请勿操作鼠标键盘）")

	if err := uiService.CreateAlbumViaUI(testAlbumName); err != nil {
		fmt.Printf("\n❌ 创建专辑失败: %v\n", err)
		fmt.Println("\n💡 可能原因:")
		fmt.Println("   1. cookies 已过期，需要重新登录")
		fmt.Println("   2. 小红书页面结构变化，需要调整选择器")
		fmt.Println("   3. 被安全验证拦截")
		os.Exit(1)
	}
	fmt.Println("\n✅ 专辑创建成功！")

	fmt.Println("\n🎉 测试完成！")
}
