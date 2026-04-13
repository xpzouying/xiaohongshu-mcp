package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// FavoriteItem 收藏笔记项
type FavoriteItem struct {
	FeedID         string    `json:"feed_id"`
	XsecToken      string    `json:"xsec_token"`
	Title          string    `json:"title"`
	Desc           string    `json:"desc"`
	CoverURL       string    `json:"cover_url"`
	UserNickname   string    `json:"user_nickname"`
	UserID         string    `json:"user_id"`
	UserAvatar     string    `json:"user_avatar"`
	LikedCount     int       `json:"liked_count"`
	CollectedCount int       `json:"collected_count"`
	CommentCount   int       `json:"comment_count"`
	CollectTime    time.Time `json:"collect_time"`
	NoteType       string    `json:"note_type"` // "video" or "image"
}

// FavoriteListResponse 收藏列表响应
type FavoriteListResponse struct {
	Items   []FavoriteItem `json:"items"`
	Count   int            `json:"count"`
	HasMore bool           `json:"has_more"`
	Cursor  string         `json:"cursor"`
}

// FavoriteListAction 收藏列表操作
type FavoriteListAction struct {
	page *rod.Page
}

// NewFavoriteListAction 创建收藏列表操作实例
func NewFavoriteListAction(page *rod.Page) *FavoriteListAction {
	return &FavoriteListAction{page: page}
}

// GetFavoriteList 获取收藏列表
// cursor: 分页游标，第一次调用传空字符串
// pageSize: 每页数量，建议 20-50
// GetFavoriteList 获取收藏列表
func (a *FavoriteListAction) GetFavoriteList(ctx context.Context, cursor string, pageSize int) (*FavoriteListResponse, error) {
	page := a.page.Context(ctx).Timeout(120 * time.Second)
	logrus.Infof("开始获取收藏列表，cursor=%s, pageSize=%d", cursor, pageSize)

	// 1. 获取用户 ID
	userID := os.Getenv("XHS_USER_ID")
	if userID == "" {
		userID = "620923cd000000002102474c"
	}
	logrus.Infof("使用用户 ID: %s", userID)

	// 2. 导航到收藏页面
	collectURL := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?tab=fav&subTab=note", userID)
	logrus.Infof("导航到收藏页面：%s", collectURL)

	if err := page.Timeout(120 * time.Second).Navigate(collectURL); err != nil {
		return nil, fmt.Errorf("导航失败：%w", err)
	}

	// 等待页面加载
	logrus.Info("等待收藏页面加载...")
	time.Sleep(8 * time.Second)

	// 3. 边滚动边收集数据（使用 ID 去重 + 多重停止条件）
	logrus.Info("开始滚动加载所有收藏...")
	seenIDs := make(map[string]bool)
	allItems := make([]FavoriteItem, 0)
	noNewCount := 0
	consecutiveNoNewScrolls := 0

	for i := 0; i < 100; i++ {
		// 记录滚动前的位置
		scrollTop := page.MustEval(`() => document.documentElement.scrollTop`).Int()

		// 滚动
		page.MustEval(`() => window.scrollBy(0, 500)`)
		time.Sleep(1 * time.Second)

		// 检查是否真的滚动了（判断是否已经到底部）
		newScrollTop := page.MustEval(`() => document.documentElement.scrollTop`).Int()

		// 如果滚动距离很小，说明可能已经到底部
		if newScrollTop-scrollTop < 100 {
			consecutiveNoNewScrolls++
			logrus.Debugf("第 %d 次滚动：滚动距离很小 (%dpx)", i+1, newScrollTop-scrollTop)
		} else {
			consecutiveNoNewScrolls = 0
		}

		// 每 2 次滚动后解析一次数据
		if (i+1)%2 == 0 {
			items, err := a.parseFavoriteItems(page)
			if err != nil {
				logrus.Warnf("解析失败：%v", err)
				continue
			}

			newCount := 0
			for _, item := range items {
				if item.FeedID != "" && !seenIDs[item.FeedID] {
					seenIDs[item.FeedID] = true
					allItems = append(allItems, item)
					newCount++
				}
			}

			if newCount > 0 {
				logrus.Infof("第 %d 次滚动：新增 %d 条，累计 %d 条", i+1, newCount, len(allItems))
				noNewCount = 0
			} else {
				noNewCount++
				logrus.Debugf("第 %d 次滚动：无新增 (累计 %d 条，无新增 %d 次)", i+1, len(allItems), noNewCount)
			}

			// 多重停止条件：
			// 1. 连续 5 次滚动无新增数据
			// 2. 或者连续 3 次滚动距离很小（已经到底部）
			if noNewCount >= 5 || consecutiveNoNewScrolls >= 3 {
				logrus.Infof("✅ 已加载到底部，共 %d 条收藏", len(allItems))
				logrus.Infof("   停止条件：无新增 %d 次，滚动距离小 %d 次", noNewCount, consecutiveNoNewScrolls)
				break
			}
		}

		// 如果连续 10 次滚动距离都很小，强制停止
		if consecutiveNoNewScrolls >= 10 {
			logrus.Infof("✅ 已滚动到底部，共 %d 条收藏", len(allItems))
			break
		}
	}

	if len(allItems) == 0 {
		items, err := a.parseFavoriteItems(page)
		if err == nil {
			allItems = items
			logrus.Infof("最终解析到 %d 条收藏", len(allItems))
		}
	}

	return &FavoriteListResponse{
		Items:   allItems,
		Count:   len(allItems),
		HasMore: false,
		Cursor:  "",
	}, nil
}

func (a *FavoriteListAction) findCollectTab(page *rod.Page) (*rod.Element, error) {
	// 等待页面加载完成
	time.Sleep(3 * time.Second)

	// 尝试多种选择器
	selectors := []string{
		// 小红书常用选择器
		".tabs .tab",
		".tab",
		"[role='tab']",
		".user-tab",
		".account-tab",
		// 尝试新的选择器
		"[class*='tab']",
		"button[class*='Tab']",
	}

	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil || len(elements) == 0 {
			continue
		}

		// 遍历查找包含"收藏"的标签
		for _, el := range elements {
			text, err := el.Text()
			if err != nil {
				continue
			}
			if strings.Contains(text, "收藏") {
				logrus.Infof("找到收藏标签：%s (text=%s)", selector, text)
				return el, nil
			}
		}
	}

	// 尝试 XPath
	xpaths := []string{
		"//*[contains(text(), '收藏')]",
		"//button[contains(text(), '收藏')]",
		"//div[contains(text(), '收藏')]",
		"//span[contains(text(), '收藏')]",
	}

	for _, xpath := range xpaths {
		elements, err := page.ElementsX(xpath)
		if err != nil || len(elements) == 0 {
			continue
		}
		logrus.Infof("通过 XPath 找到收藏标签：%s", xpath)
		return elements[0], nil
	}

	// 截图调试（可选）
	// page.MustScreenshot("debug_tab.png")
	// html, _ := page.HTML()
	// logrus.Infof("页面 HTML 片段：%s", html[:500])

	return nil, fmt.Errorf("未找到收藏标签页")
}

// parseFavoriteItems 解析收藏笔记列表
func (a *FavoriteListAction) parseFavoriteItems(page *rod.Page) ([]FavoriteItem, error) {
	// 尝试从 __INITIAL_STATE__ 获取数据
	data, err := a.parseFromInitialState(page)
	if err == nil && len(data) > 0 {
		return data, nil
	}

	logrus.Warnf("从 __INITIAL_STATE__ 解析失败：%v，尝试从 DOM 解析", err)

	// 回退到从 DOM 解析
	return a.parseFromDOM(page)
}

// parseFromInitialState 从 __INITIAL_STATE__ 解析数据
func (a *FavoriteListAction) parseFromInitialState(page *rod.Page) ([]FavoriteItem, error) {
	// 调试：打印 __INITIAL_STATE__ 的完整结构
	debugScript := `() => {
		try {
			if (window.__INITIAL_STATE__) {
				const state = window.__INITIAL_STATE__;
				const user = state.user || {};
				const userPageData = user.userPageData || {};
				const result = {
					hasUser: !!user,
					userKeys: Object.keys(user),
					collectData: user.collectData ? 'exists' : 'missing',
					collect: user.collect ? 'exists' : 'missing',
					notes: user.notes ? 'exists' : 'missing',
					userPageDataKeys: Object.keys(userPageData),
					hasCollectList: !!userPageData.collectList
				};
				return JSON.stringify(result);
			}
			return "no __INITIAL_STATE__";
		} catch(e) {
			return "error: " + e.message;
		}
	}`
	debugResult := page.MustEval(debugScript).String()
	logrus.Infof("DEBUG __INITIAL_STATE__ 结构：%s", debugResult)

	script := `() => {
		try {
			const state = window.__INITIAL_STATE__ || {};
			const user = state.user || {};
			const userPageData = user.userPageData || {};
			
			// 尝试多个可能的路径
			if (user.collectData) {
				return JSON.stringify(user.collectData);
			}
			if (user.collect) {
				return JSON.stringify(user.collect);
			}
			if (userPageData.collectList) {
				return JSON.stringify(userPageData.collectList);
			}
			if (userPageData.notes) {
				return JSON.stringify(userPageData.notes);
			}
			if (user.notes) {
				return JSON.stringify(user.notes);
			}
			
			// 尝试从 _rawValue 获取（Vue 3 响应式对象）
			if (userPageData._rawValue && userPageData._rawValue.collectList) {
				return JSON.stringify(userPageData._rawValue.collectList);
			}
			if (userPageData._value && userPageData._value.collectList) {
				return JSON.stringify(userPageData._value.collectList);
			}
			
			return "null";
		} catch(e) {
			return "null";
		}
	}`

	result := page.MustEval(script).String()
	logrus.Infof("DEBUG 收藏数据结果长度：%d 字符", len(result))
	if len(result) > 0 && len(result) < 1000 {
		end := 500
		if len(result) < end {
			end = len(result)
		}
		logrus.Infof("DEBUG 收藏数据预览：%s", result[:end])
	}

	if result == "null" || result == "" || len(result) < 10 {
		return nil, fmt.Errorf("__INITIAL_STATE__ 中没有收藏数据")
	}

	// 尝试解析多种数据结构
	var collectData interface{}
	if err := json.Unmarshal([]byte(result), &collectData); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败：%w", err)
	}

	// 根据实际数据结构解析
	return a.parseCollectData(collectData)
}

// parseCollectData 解析收藏数据（支持多种格式）
func (a *FavoriteListAction) parseCollectData(data interface{}) ([]FavoriteItem, error) {
	items := make([]FavoriteItem, 0)

	// 转换为 map
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("数据格式错误")
	}

	// 尝试获取 items 数组
	var itemsData []interface{}

	if data, ok := dataMap["items"]; ok {
		itemsData, ok = data.([]interface{})
	} else if data, ok := dataMap["data"]; ok {
		itemsData, ok = data.([]interface{})
	} else if data, ok := dataMap["noteList"]; ok {
		itemsData, ok = data.([]interface{})
	}

	if itemsData == nil {
		return nil, fmt.Errorf("未找到 items 数据")
	}

	// 解析每个 item
	for _, item := range itemsData {
		favItem, err := a.parseFavoriteItem(item)
		if err != nil {
			logrus.Warnf("解析单个收藏项失败：%v", err)
			continue
		}
		if favItem != nil {
			items = append(items, *favItem)
		}
	}

	return items, nil
}

// parseFavoriteItem 解析单个收藏项
func (a *FavoriteListAction) parseFavoriteItem(item interface{}) (*FavoriteItem, error) {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("item 格式错误")
	}

	favItem := &FavoriteItem{}

	// 获取 noteId 或 id
	if noteID, ok := getStringField(itemMap, "noteId"); ok {
		favItem.FeedID = noteID
	} else if id, ok := getStringField(itemMap, "id"); ok {
		favItem.FeedID = id
	}

	// 获取 xsecToken
	if token, ok := getStringField(itemMap, "xsecToken"); ok {
		favItem.XsecToken = token
	}

	// 获取 note 信息
	if noteData, ok := itemMap["note"].(map[string]interface{}); ok {
		favItem.Title = getStringFieldOrDefault(noteData, "title", "")
		favItem.Desc = getStringFieldOrDefault(noteData, "desc", "")
		favItem.NoteType = getStringFieldOrDefault(noteData, "type", "image")

		// 获取用户信息
		if userData, ok := noteData["user"].(map[string]interface{}); ok {
			favItem.UserNickname = getStringFieldOrDefault(userData, "nickname", "")
			favItem.UserID = getStringFieldOrDefault(userData, "userId", "")
			favItem.UserAvatar = getStringFieldOrDefault(userData, "avatar", "")
		}

		// 获取互动数据
		if interactData, ok := noteData["interactInfo"].(map[string]interface{}); ok {
			favItem.LikedCount = getIntFieldOrDefault(interactData, "likedCount", 0)
			favItem.CollectedCount = getIntFieldOrDefault(interactData, "collectedCount", 0)
			favItem.CommentCount = getIntFieldOrDefault(interactData, "commentCount", 0)
		}

		// 获取封面图
		if images, ok := noteData["images"].([]interface{}); ok && len(images) > 0 {
			if imgMap, ok := images[0].(map[string]interface{}); ok {
				favItem.CoverURL = getStringFieldOrDefault(imgMap, "urlDefault", "")
			}
		}
	}

	// 获取收藏时间
	if collectTime, ok := getFloatField(itemMap, "collectTime"); ok {
		favItem.CollectTime = time.Unix(int64(collectTime)/1000, 0)
	} else if collectTimeStr, ok := getStringField(itemMap, "collectTime"); ok {
		if t, err := time.Parse(time.RFC3339, collectTimeStr); err == nil {
			favItem.CollectTime = t
		}
	}

	// 如果必填字段缺失，返回错误
	if favItem.FeedID == "" {
		return nil, fmt.Errorf("缺少 feed_id")
	}

	return favItem, nil
}

// parseFromDOM 从 DOM 解析收藏列表（备用方案）
func (a *FavoriteListAction) parseFromDOM(page *rod.Page) ([]FavoriteItem, error) {
	items := make([]FavoriteItem, 0)

	// 查找所有笔记卡片
	cards, err := page.Elements("section.note-item")
	if err != nil {
		logrus.Warnf("未找到笔记卡片：%v", err)
		return nil, fmt.Errorf("未找到笔记卡片：%w", err)
	}

	logrus.Infof("DOM 解析：找到 %d 个笔记卡片", len(cards))

	for i, card := range cards {
		item := FavoriteItem{}

		// 获取标题
		if titleEl, err := card.Element(".title"); err == nil && titleEl != nil {
			item.Title, _ = titleEl.Text()
		}

		// 获取封面图
		if imgEl, err := card.Element("img"); err == nil && imgEl != nil {
			if src, err := imgEl.Attribute("src"); err == nil && src != nil {
				item.CoverURL = *src
			}
		}

		// 获取 feed_id 和 xsec_token（从所有 a 标签中提取）
		links, _ := card.Elements("a")
		for _, linkEl := range links {
			if href, err := linkEl.Attribute("href"); err == nil && href != nil {
				hrefStr := *href
				// 提取 feed_id（优先使用）
				if item.FeedID == "" && strings.Contains(hrefStr, "/explore/") {
					parts := strings.Split(hrefStr, "/explore/")
					if len(parts) > 1 {
						idPart := strings.Split(parts[1], "?")[0]
						item.FeedID = idPart
					}
				}
				// 提取 xsec_token（从带参数的 URL 中）
				if item.XsecToken == "" && strings.Contains(hrefStr, "xsec_token=") {
					tokenParts := strings.Split(hrefStr, "xsec_token=")
					if len(tokenParts) > 1 {
						token := strings.Split(tokenParts[1], "&")[0]
						item.XsecToken = token
					}
				}
			}
		}

		// 获取用户信息
		if userEl, err := card.Element(".nickname"); err == nil && userEl != nil {
			item.UserNickname, _ = userEl.Text()
		}

		// 获取描述
		if descEl, err := card.Element(".desc"); err == nil && descEl != nil {
			item.Desc, _ = descEl.Text()
		}

		logrus.Debugf("笔记 %d: ID=%s, Token=%s, Title=%s", i+1, item.FeedID, item.XsecToken, item.Title)

		if item.FeedID != "" || item.Title != "" {
			items = append(items, item)
		}
	}

	return items, nil
}

// scrollAndLoadAll 滚动页面加载所有收藏笔记
func (a *FavoriteListAction) scrollAndLoadAll(page *rod.Page, pageSize int) {
	// 小红书使用虚拟滚动，需要滚动到底部加载所有数据
	// 或者至少滚动到能获取 pageSize 条数据

	maxScrolls := 10 // 最多滚动 10 次
	lastHeight := 0

	for i := 0; i < maxScrolls; i++ {
		// 获取当前笔记数量
		count := a.countNotesOnPage(page)
		logrus.Debugf("第 %d 次滚动后，当前笔记数：%d", i+1, count)

		// 如果已经获取到足够的数据，停止滚动
		if count >= pageSize {
			logrus.Infof("已加载 %d 条笔记，满足 pageSize=%d", count, pageSize)
			break
		}

		// 滚动到页面底部
		page.MustEval(`() => window.scrollTo(0, document.body.scrollHeight)`)
		time.Sleep(2 * time.Second)

		// 检查是否已经滚动到底部
		currentHeight := int(page.MustEval(`() => document.body.scrollHeight`).Int())
		if currentHeight == lastHeight {
			logrus.Info("已滚动到底部，没有更多数据")
			break
		}
		lastHeight = currentHeight
	}
}

// countNotesOnPage 计算当前页面上的笔记数量
func (a *FavoriteListAction) countNotesOnPage(page *rod.Page) int {
	// 尝试多种选择器
	selectors := []string{
		".note-item",
		"[class*='note-item']",
		"section article",
		"[data-type='note']",
		".feed-item",
		"[class*='feed']",
		".download-item",
	}

	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err == nil && len(elements) > 0 {
			logrus.Debugf("选择器 '%s' 找到 %d 个元素", selector, len(elements))
			return len(elements)
		}
	}

	// 如果都找不到，尝试获取所有可能的卡片元素
	html, _ := page.HTML()
	logrus.Debugf("页面 HTML 长度：%d", len(html))

	// 如果都找不到，返回 0
	return 0
}

// scrollToCursor 滚动到指定 cursor 位置
func (a *FavoriteListAction) scrollToCursor(page *rod.Page, cursor string) {
	// 简单的滚动实现，可以优化为更精确的定位
	for i := 0; i < 5; i++ {
		page.MustEval(`() => window.scrollBy(0, window.innerHeight)`)
		time.Sleep(500 * time.Millisecond)
	}
}

// 辅助函数：获取字符串字段
func getStringField(m map[string]interface{}, key string) (string, bool) {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// 辅助函数：获取字符串字段（带默认值）
func getStringFieldOrDefault(m map[string]interface{}, key string, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// 辅助函数：获取数字字段
func getFloatField(m map[string]interface{}, key string) (float64, bool) {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f, true
		}
		if i, ok := v.(int); ok {
			return float64(i), true
		}
		if i, ok := v.(int64); ok {
			return float64(i), true
		}
	}
	return 0, false
}

// 辅助函数：获取数字字段（带默认值）
func getIntFieldOrDefault(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
		if i, ok := v.(int); ok {
			return i
		}
	}
	return defaultVal
}
