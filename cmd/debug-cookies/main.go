package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
}

func main() {
	// Read cookies file
	data, err := os.ReadFile("cookies.json")
	if err != nil {
		fmt.Printf("❌ 读取 cookies 文件失败: %v\n", err)
		return
	}

	// 先测试是否能正确解析为 proto.NetworkCookie
	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		fmt.Printf("❌ 解析 cookies JSON 失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 成功解析 %d 个 cookies\n", len(cookies))

	for _, c := range cookies {
		fmt.Printf("  %s (domain=%s, httpOnly=%v)\n", c.Name, c.Domain, c.HTTPOnly)
	}

	// 启动浏览器
	fmt.Println("\n🚀 启动浏览器...")
	b := browser.NewBrowser(true, browser.WithBinPath(configs.GetBinPath()))
	defer b.Close()

	// 创建页面
	page := b.NewPage()
	defer page.Close()

	// 导航到个人主页
	fmt.Println("\n🌐 导航到个人主页...")
	if err := page.Timeout(30 * time.Second).Navigate("https://www.xiaohongshu.com/user/profile/me"); err != nil {
		fmt.Printf("❌ 导航失败: %v\n", err)
		return
	}
	time.Sleep(5 * time.Second)

	// 检查标题
	title := page.MustInfo().Title
	fmt.Printf("📄 页面标题: %s\n", title)

	// 检查页面内容
	body, err := page.Element("body")
	if err == nil {
		text, _ := body.Text()
		if len(text) > 500 {
			text = text[:500] + "..."
		}
		fmt.Printf("📝 页面文本: %s\n", text)
	}

	// 检查当前页面 cookies
	fmt.Println("\n🍪 当前页面 cookies:")
	allCookies, _ := page.Cookies([]string{})
	fmt.Printf("  当前有 %d 个 cookies\n", len(allCookies))
	for _, c := range allCookies {
		if c.Name == "web_session" || c.Name == "a1" || c.Name == "id_token" {
			fmt.Printf("  %s = %s... (domain=%s, httpOnly=%v)\n",
				c.Name, c.Value[:min(30, len(c.Value))], c.Domain, c.HTTPOnly)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
