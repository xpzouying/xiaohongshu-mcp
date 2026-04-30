package main

import (
	"context"
	"flag"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/go-rod/stealth"
)

func main() {
	headless := flag.Bool("headless", true, "无头模式")
	categoriesFile := flag.String("file", "收藏分类结果.json", "分类结果文件")
	flag.Parse()

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🚀 小红书收藏专辑自动同步 (UI 自动化版)")
	fmt.Println(strings.Repeat("=", 60))

	// 检查分类文件
	if _, err := os.Stat(*categoriesFile); os.IsNotExist(err) {
		fmt.Printf("❌ 分类文件不存在: %s\n", *categoriesFile)
		os.Exit(1)
	}

	data, _ := os.ReadFile(*categoriesFile)
	var catResult struct {
		Total      int                    `json:"total"`
		Categories map[string]interface{} `json:"categories"`
	}
	json.Unmarshal(data, &catResult)
	fmt.Printf("📊 分类文件: %d 条笔记\n", catResult.Total)
	for cat, d := range catResult.Categories {
		if cat == "其他" {
			continue
		}
		if m, ok := d.(map[string]interface{}); ok {
			if count, ok := m["count"].(float64); ok && count > 0 {
				fmt.Printf("   📁 %-15s %.0f 条\n", cat+":", count)
			}
		}
	}
	fmt.Println()

	// 创建浏览器
	fmt.Println("🌐 初始化浏览器...")
	b := browser.NewBrowser(*headless)
	defer b.Close()

	// 使用 stealth 模式创建页面
	browserInstance := b.(*BrowserWrapper) // 注意：这里需要根据实际的 browser 包调整

	// 简化方案：直接用 rod 创建页面
	fmt.Println("✅ 浏览器就绪")

	// 导航到收藏页面
	fmt.Println("\n📂 导航到收藏页面...")
	page := b.NewPage()
	defer page.Close()

	// 使用 stealth
	page, _ = stealth.Page(page.Browser())

	err := page.Timeout(30 * time.Second).Navigate("https://www.xiaohongshu.com/user/profile/me?tab=fav&subTab=album")
	if err != nil {
		fmt.Printf("❌ 导航失败: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(5 * time.Second)

	title := page.MustInfo().Title
	fmt.Printf("页面标题: %s\n", title)

	if strings.Contains(title, "Security") {
		fmt.Println("❌ 页面显示安全验证，cookies 可能过期")
		os.Exit(1)
	}

	// 尝试通过 UI 操作创建专辑
	fmt.Println("\n📝 尝试创建专辑...")

	// 查找"创建专辑"按钮
	btns, err := page.Elements("button")
	if err != nil {
		fmt.Printf("查找按钮失败: %v\n", err)
	}

	found := false
	for _, btn := range btns {
		text, _ := btn.Text()
		fmt.Printf("按钮: %q\n", text)
		if strings.Contains(text, "创建") && strings.Contains(text, "专辑") {
			fmt.Println("✅ 找到「创建专辑」按钮")
			btn.MustClick(proto.InputMouseButtonLeft, 1)
			time.Sleep(2 * time.Second)
			found = true
			break
		}
	}

	if !found {
		fmt.Println("⚠️ 未找到创建专辑按钮，尝试其他方法...")
		// 尝试导航到创建专辑页面
		page.Timeout(15 * time.Second).Navigate("https://www.xiaohongshu.com/album/create")
		time.Sleep(3 * time.Second)
		fmt.Printf("新标题: %s\n", page.MustInfo().Title)
	}

	// 获取页面内容用于调试
	body, _ := page.Element("body")
	if body != nil {
		text, _ := body.Text()
		if len(text) > 500 {
			text = text[:500]
		}
		fmt.Printf("页面内容预览: %s\n", text)
	}
}
