package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func init() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
}

func main() {
	fmt.Println("=== 小红书页面诊断 ===")
	fmt.Println()

	b := browser.NewBrowser(true, browser.WithBinPath(configs.GetBinPath()))
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	// 导航到个人主页
	fmt.Println("🌐 导航到个人主页...")
	if err := page.Timeout(30 * time.Second).Navigate("https://www.xiaohongshu.com/user/profile/me"); err != nil {
		fmt.Printf("❌ 导航失败: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(5 * time.Second)

	// 检查标题
	title := page.MustInfo().Title
	fmt.Printf("📄 页面标题: %s\n", title)

	if strings.Contains(title, "Security") || strings.Contains(title, "验证") {
		fmt.Println("\n❌ 被安全验证拦截！cookies 无效或已过期")
		fmt.Println("💡 请确保 cookies 包含 httpOnly 的 web_session 和 id_token")
		os.Exit(1)
	}

	// 截图
	page.MustScreenshot("/tmp/diagnose_profile.png")
	fmt.Println("📸 截图已保存到 /tmp/diagnose_profile.png")

	// 获取页面所有文本
	body, err := page.Element("body")
	if err == nil {
		text, _ := body.Text()
		// 截取前 2000 字符
		if len(text) > 2000 {
			text = text[:2000] + "..."
		}
		fmt.Printf("\n📝 页面文本内容（前2000字符）:\n%s\n", text)
	}

	// 查找所有按钮和 tab
	fmt.Println("\n🔍 页面上的按钮/tab 元素:")
	script := `
		(function() {
			var result = [];
			var selectors = ['button', 'a', '[role="tab"]', '[role="button"]', '.tab', '.tab-item', '.nav-item', '.user-tab'];
			selectors.forEach(function(sel) {
				document.querySelectorAll(sel).forEach(function(el) {
					if (el.offsetParent !== null) {
						result.push({
							tag: el.tagName,
							text: (el.innerText || '').substring(0, 30),
							class: (el.className || '').substring(0, 60),
							href: el.href || ''
						});
					}
				});
			});
			return JSON.stringify(result);
		})()
	`
	result, err := page.Eval(script)
	if err == nil && result != nil {
		fmt.Println(result.Value.String())
	}

	// 检查 cookies 中的登录状态
	fmt.Println("\n🍪 当前 cookies 检查:")
	cookies, _ := page.Cookies([]string{"web_session", "a1", "id_token", "websectiga"})
	for _, c := range cookies {
		fmt.Printf("  %s = %s... (domain=%s, httpOnly=%v)\n",
			c.Name,
			func() string {
				v := c.Value
				if len(v) > 30 {
					return v[:30] + "..."
				}
				return v
			}(),
			c.Domain, c.HTTPOnly)
	}

	fmt.Println("\n✅ 诊断完成")
}
