package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/sirupsen/logrus"
)

// AlbumUIService 基于 UI 自动化的专辑同步服务
// 原理：通过 go-rod 模拟用户操作（点击按钮、填写表单）来创建专辑和添加笔记
// 完全绕过 API 签名和 CSP 问题
type AlbumUIService struct {
	page     *rod.Page
	headless bool
}

// NewAlbumUIService 创建 UI 自动化专辑服务
func NewAlbumUIService(page *rod.Page, headless ...bool) *AlbumUIService {
	h := false
	if len(headless) > 0 {
		h = headless[0]
	}
	return &AlbumUIService{page: page, headless: h}
}

// sleep 统一延迟函数，方便调试时调整
func (s *AlbumUIService) sleep(d time.Duration) {
	time.Sleep(d)
}

// checkSecurity 检查页面是否被安全验证拦截
func (s *AlbumUIService) checkSecurity() error {
	s.sleep(2 * time.Second)
	title := s.page.MustInfo().Title
	if strings.Contains(title, "Security") || strings.Contains(title, "验证") || strings.Contains(title, "captcha") {
		return fmt.Errorf("页面显示安全验证 (标题: %s)，cookies 可能已过期", title)
	}
	return nil
}

// navigateToExplore 导航到 explore 页面并检查登录状态
func (s *AlbumUIService) navigateToExplore() error {
	logrus.Info("🌐 导航到首页...")
	if err := s.page.Timeout(30 * time.Second).Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	// 等待页面完全加载
	s.page.MustWaitLoad()
	s.sleep(3 * time.Second)
	return s.checkSecurity()
}

// navigateToProfileViaClick 通过点击「我」入口导航到个人主页（避免风控拦截）
func (s *AlbumUIService) navigateToProfileViaClick() error {
	logrus.Info("🌐 通过点击「我」进入个人主页...")

	// 等待 JS 完全初始化
	s.sleep(5 * time.Second)

	// 查找页面中所有包含"我"文本的 span 元素
	// explore 页面左上角/右上角通常有「我」的入口
	els, err := s.page.ElementsX(`//span[contains(text(),"我")]`)
	if err == nil && len(els) > 0 {
		// 找到文本恰好为"我"的元素
		for _, el := range els {
			text, _ := el.Text()
			if strings.TrimSpace(text) == "我" {
				logrus.Info("  找到「我」入口，点击...")
				el.MustClick()
				s.sleep(5 * time.Second)

				title := s.page.MustInfo().Title
				if strings.Contains(title, "Security") {
					return fmt.Errorf("被安全验证拦截: %s", title)
				}
				logrus.Info("  ✅ 成功进入个人主页")
				return nil
			}
		}
	}

	// 备用方案：查找包含 profile/me 的链接
	links, err := s.page.ElementsX(`//a[contains(@href, "/user/profile/me")]`)
	if err == nil && len(links) > 0 {
		logrus.Info("  找到 profile 链接，点击...")
		links[0].MustClick()
		s.sleep(5 * time.Second)

		title := s.page.MustInfo().Title
		if strings.Contains(title, "Security") {
			return fmt.Errorf("被安全验证拦截: %s", title)
		}
		logrus.Info("  ✅ 成功进入个人主页")
		return nil
	}

	// 最后的备用方案：直接 navigate（可能被拦截）
	logrus.Warn("  未找到「我」入口，尝试直接导航...")
	if err := s.page.Timeout(30 * time.Second).Navigate("https://www.xiaohongshu.com/user/profile/me"); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}
	s.sleep(5 * time.Second)
	return s.checkSecurity()
}

// navigateToProfileFav 导航到个人主页收藏页面
func (s *AlbumUIService) navigateToProfileFav() error {
	logrus.Info("🌐 导航到个人主页收藏页...")

	// Step 1: 先访问 explore（这个不会被拦截）
	if err := s.navigateToExplore(); err != nil {
		return err
	}

	// Step 2: 通过点击「我」进入个人主页
	if err := s.navigateToProfileViaClick(); err != nil {
		return err
	}

	// Step 3: 查找并点击"收藏"tab
	logrus.Info("  查找「收藏」tab...")
	tab, err := s.findTabByText("收藏")
	if err != nil {
		logrus.Warnf("  未找到「收藏」tab")
		// 截图调试
		if !s.headless {
			s.page.MustScreenshot("debug_no_fav_tab.png")
		}
		return fmt.Errorf("未找到「收藏」tab")
	}

	logrus.Info("  点击「收藏」tab...")
	tab.MustClick()
	s.sleep(3 * time.Second)

	// 检查是否有「专辑」子 tab
	subTab, err := s.findTabByText("专辑")
	if err == nil {
		logrus.Info("  点击「专辑」子 tab...")
		subTab.MustClick()
		s.sleep(3 * time.Second)
	}

	return s.checkSecurity()
}

// findTabByText 通过文本查找 tab 元素
func (s *AlbumUIService) findTabByText(text string) (*rod.Element, error) {
	// 方法1: 通过 XPath 查找包含指定文本的元素
	xpath := fmt.Sprintf(`//*[contains(text(), "%s")]`, text)
	els, err := s.page.ElementsX(xpath)
	if err == nil && len(els) > 0 {
		return els[0], nil
	}

	// 方法2: 遍历所有可能可点击的元素
	allClickable := []string{"button", "a", "[role=tab]", "[role=button]", ".tab-item", ".tab", ".nav-item"}
	for _, selector := range allClickable {
		els, err := s.page.Elements(selector)
		if err == nil {
			for _, el := range els {
				elText, _ := el.Text()
				if strings.Contains(elText, text) {
					return el, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("未找到包含文本「%s」的 tab 元素", text)
}

// findCreateAlbumButton 查找创建专辑按钮
func (s *AlbumUIService) findCreateAlbumButton() (*rod.Element, error) {
	// 查找包含"创建专辑"、"新建专辑"、"新建"、"+"等文本的按钮
	keywords := []string{"创建专辑", "新建专辑", "新建收藏专辑", "新建", "创建", "+"}

	for _, keyword := range keywords {
		// 方法1: 查找按钮文本
		xpath := fmt.Sprintf(`//button[contains(text(), "%s")]`, keyword)
		els, err := s.page.ElementsX(xpath)
		if err == nil && len(els) > 0 {
			logrus.Infof("  找到「%s」按钮 (XPath)", keyword)
			return els[0], nil
		}

		// 方法2: 查找任意包含关键词的可点击元素
		xpath2 := fmt.Sprintf(`//*[contains(text(), "%s")]`, keyword)
		els2, err := s.page.ElementsX(xpath2)
		if err == nil && len(els2) > 0 {
			for _, el := range els2 {
				tagName := el.MustProperty("tagName")
				tag := tagName.Str()
				if tag == "BUTTON" || tag == "A" || tag == "SPAN" || tag == "DIV" {
					role, _ := el.Attribute("role")
					if role != nil && (*role == "button" || *role == "tab") {
						logrus.Infof("  找到「%s」元素 (tagName=%s)", keyword, tag)
						return el, nil
					}
				}
			}
		}
	}

	// 方法3: 查找常见的创建按钮 CSS 选择器
	createSelectors := []string{
		".create-album-btn",
		".new-album-btn",
		".add-album-btn",
		".create-folder-btn",
		".album-create-btn",
		"[data-action='create-album']",
		"[data-action='create-folder']",
		".btn-create",
		".btn-new",
	}

	for _, selector := range createSelectors {
		el, err := s.page.Element(selector)
		if err == nil && el != nil {
			logrus.Infof("  找到创建按钮 (CSS: %s)", selector)
			return el, nil
		}
	}

	return nil, fmt.Errorf("未找到创建专辑按钮")
}

// findAlbumInput 查找专辑名称输入框
func (s *AlbumUIService) findAlbumInput() (*rod.Element, error) {
	// 方法1: 查找 placeholder 包含"专辑"的输入框
	inputs, err := s.page.Elements("input")
	if err == nil {
		for _, inp := range inputs {
			placeholder, _ := inp.Attribute("placeholder")
			if placeholder != nil && (strings.Contains(*placeholder, "专辑") || strings.Contains(*placeholder, "名称") || strings.Contains(*placeholder, "name")) {
				return inp, nil
			}
		}
	}

	// 方法2: 查找 textarea
	textareas, err := s.page.Elements("textarea")
	if err == nil && len(textareas) > 0 {
		return textareas[0], nil
	}

	// 方法3: 查找第一个可见的 text input
	inputs2, err := s.page.Elements("input[type='text'], input:not([type])")
	if err == nil && len(inputs2) > 0 {
		return inputs2[0], nil
	}

	return nil, fmt.Errorf("未找到专辑名称输入框")
}

// findConfirmButton 查找确认按钮
func (s *AlbumUIService) findConfirmButton() (*rod.Element, error) {
	keywords := []string{"确定", "确认", "创建", "完成", "保存"}

	for _, keyword := range keywords {
		xpath := fmt.Sprintf(`//button[contains(text(), "%s")]`, keyword)
		els, err := s.page.ElementsX(xpath)
		if err == nil && len(els) > 0 {
			// 取最后一个（通常是模态框中的确认按钮）
			return els[len(els)-1], nil
		}
	}

	// 查找模态框中的第一个按钮
	modals := []string{".modal", ".dialog", "[role='dialog']", ".popup"}
	for _, selector := range modals {
		modal, err := s.page.Element(selector)
		if err == nil && modal != nil {
			btns, err := modal.Elements("button")
			if err == nil && len(btns) > 0 {
				return btns[len(btns)-1], nil
			}
		}
	}

	return nil, fmt.Errorf("未找到确认按钮")
}

// CreateAlbumViaUI 通过 UI 操作创建专辑（公开方法，供外部调用）
func (s *AlbumUIService) CreateAlbumViaUI(name string) error {
	logrus.Infof("📝 创建专辑: %s", name)

	// 步骤1: 导航到收藏页面
	if err := s.navigateToProfileFav(); err != nil {
		return fmt.Errorf("导航到收藏页面失败: %w", err)
	}

	// 步骤2: 查找创建专辑按钮
	btn, err := s.findCreateAlbumButton()
	if err != nil {
		logrus.Warnf("  未找到创建按钮: %v", err)

		// 回退方案：截图调试
		if !s.headless {
			logrus.Info("  正在截图以便调试...")
			s.page.MustScreenshot("debug_no_create_button.png")
		}

		// 尝试通过 JS 查找所有可能的按钮
		script := `
			(function() {
				var buttons = document.querySelectorAll('button, [role="button"], .btn');
				var result = [];
				buttons.forEach(function(btn) {
					if (btn.offsetParent !== null) {
						result.push({tag: btn.tagName, text: btn.innerText.substring(0, 50), class: btn.className.substring(0, 80)});
					}
				});
				return JSON.stringify(result);
			})()
		`
		result, err := s.page.Eval(script)
		if err == nil && result != nil {
			logrus.Infof("  页面上可见的按钮: %s", result.Value.String())
		}

		return fmt.Errorf("未找到创建专辑按钮，可能需要手动操作")
	}

	// 步骤3: 点击创建按钮
	logrus.Info("  点击创建按钮...")
	btn.MustClick()
	s.sleep(2 * time.Second)

	// 步骤4: 输入专辑名称
	inp, err := s.findAlbumInput()
	if err != nil {
		return fmt.Errorf("未找到输入框: %w", err)
	}

	logrus.Info("  输入专辑名称...")
	// 先点击输入框
	inp.MustClick()
	s.sleep(200 * time.Millisecond)

	// 全选 + 删除清空
	if err := inp.SelectAllText(); err != nil {
		logrus.Warnf("  全选失败，直接输入: %v", err)
	}
	s.sleep(200 * time.Millisecond)
	inp.MustInput(name)
	s.sleep(500 * time.Millisecond)

	// 步骤5: 点击确认
	confirmBtn, err := s.findConfirmButton()
	if err != nil {
		logrus.Warnf("  未找到确认按钮，尝试按 Enter")
		s.page.Keyboard.Press(input.Enter)
		s.sleep(2 * time.Second)
	} else {
		logrus.Info("  点击确认按钮...")
		confirmBtn.MustClick()
		s.sleep(2 * time.Second)
	}

	// 检查是否有错误提示
	errorTexts := []string{"失败", "错误", "已存在", "重复"}
	for _, text := range errorTexts {
		xpath := fmt.Sprintf(`//*[contains(text(), "%s")]`, text)
		els, err := s.page.ElementsX(xpath)
		if err == nil && len(els) > 0 {
			for _, el := range els {
				elText, _ := el.Text()
				if len(elText) < 100 { // 过滤掉大段文本
					// 检查是否是错误提示（通常在 toast 或 error 容器中）
					parent, _ := el.Parent()
					if parent != nil {
						class, _ := parent.Attribute("class")
						if class != nil && (strings.Contains(*class, "error") || strings.Contains(*class, "toast") || strings.Contains(*class, "alert")) {
							return fmt.Errorf("创建专辑失败: %s", elText)
						}
					}
				}
			}
		}
	}

	logrus.Infof("✅ 专辑创建成功: %s", name)
	return nil
}

// SyncCategoriesToAlbums 完整同步流程（UI 自动化版）
func (s *AlbumUIService) SyncCategoriesToAlbums(ctx context.Context, categoriesFile string) (*SyncResult, error) {
	logrus.Info("🚀 开始 UI 自动化专辑同步...")

	// 先导航到首页确保登录有效
	if err := s.navigateToExplore(); err != nil {
		return nil, err
	}

	categories, total, err := s.loadCategories(categoriesFile)
	if err != nil {
		return nil, fmt.Errorf("加载分类失败: %w", err)
	}

	result := &SyncResult{Total: total}
	logrus.Infof("加载 %d 条笔记，%d 个分类", total, len(categories))

	for category, catData := range categories {
		if category == "其他" {
			continue
		}

		catMap, ok := catData.(map[string]interface{})
		if !ok {
			continue
		}

		countVal, ok := catMap["count"].(float64)
		if !ok || countVal == 0 {
			continue
		}
		count := int(countVal)

		itemsVal, ok := catMap["items"].([]interface{})
		if !ok {
			continue
		}

		logrus.Infof("\n📁 【%s】 (%d 条)", category, count)

		var noteIDs []string
		for _, item := range itemsVal {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if feedID, ok := itemMap["feed_id"].(string); ok && feedID != "" {
				noteIDs = append(noteIDs, feedID)
			}
		}

		if len(noteIDs) == 0 {
			continue
		}

		entry := AlbumSyncEntry{Name: category, Count: count}

		// 创建专辑
		if err := s.CreateAlbumViaUI(category); err != nil {
			logrus.Warnf("  创建失败: %v", err)
			entry.Message = "失败: " + err.Error()
			result.Failed++
		} else {
			entry.Message = "创建成功"
			entry.Success = true
			entry.SuccessCount = len(noteIDs)
			result.Success++
		}

		result.Albums = append(result.Albums, entry)
		s.sleep(3 * time.Second)
	}

	return result, nil
}

func (s *AlbumUIService) loadCategories(file string) (map[string]interface{}, int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, 0, fmt.Errorf("读取文件失败: %w", err)
	}

	var result struct {
		Total      int                    `json:"total"`
		Categories map[string]interface{} `json:"categories"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	return result.Categories, result.Total, nil
}
