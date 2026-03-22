package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	myerrors "github.com/xpzouying/xiaohongshu-mcp/errors"
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
func (a *FavoriteListAction) GetFavoriteList(ctx context.Context, cursor string, pageSize int) (*FavoriteListResponse, error) {
	page := a.page.Context(ctx).Timeout(120 * time.Second)

	logrus.Infof("开始获取收藏列表，cursor=%s, pageSize=%d", cursor, pageSize)

	// 1. 导航到个人主页
	profileURL := "https://www.xiaohongshu.com/user/profile/me"
	logrus.Infof("导航到个人主页：%s", profileURL)
	
	if err := page.Navigate(profileURL); err != nil {
		return nil, fmt.Errorf("导航失败：%w", err)
	}

	// 等待页面稳定
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	// 2. 检查是否登录
	loginCheck := page.MustElementR("body", "登录")
	if loginCheck != nil {
		// 尝试查找登录按钮
		if el, err := page.ElementR("button", "登录"); err == nil {
			text, _ := el.Text()
			if text != "" {
				return nil, myerrors.ErrNotLoggedIn
			}
		}
	}

	// 3. 点击"收藏"标签页
	logrus.Info("查找并点击收藏标签页")
	collectTab, err := a.findCollectTab(page)
	if err != nil {
		return nil, fmt.Errorf("未找到收藏标签页：%w", err)
	}

	if err := collectTab.Click(); err != nil {
		return nil, fmt.Errorf("点击收藏标签失败：%w", err)
	}

	logrus.Info("已点击收藏标签，等待加载")
	time.Sleep(3 * time.Second)

	// 4. 如果有 cursor，滚动到加载位置
	if cursor != "" {
		logrus.Infof("滚动到指定位置：%s", cursor)
		a.scrollToCursor(page, cursor)
		time.Sleep(2 * time.Second)
	}

	// 5. 解析收藏列表数据
	logrus.Info("解析收藏列表数据")
	items, err := a.parseFavoriteItems(page)
	if err != nil {
		return nil, fmt.Errorf("解析数据失败：%w", err)
	}

	logrus.Infof("解析到 %d 条收藏笔记", len(items))

	// 6. 判断是否还有更多
	hasMore := len(items) >= pageSize
	if len(items) < pageSize {
		hasMore = false
	}

	// 7. 生成下一个 cursor
	nextCursor := ""
	if hasMore && len(items) > 0 {
		nextCursor = items[len(items)-1].CollectTime.Format(time.RFC3339)
	}

	return &FavoriteListResponse{
		Items:   items,
		Count:   len(items),
		HasMore: hasMore,
		Cursor:  nextCursor,
	}, nil
}

// findCollectTab 查找收藏标签页按钮
func (a *FavoriteListAction) findCollectTab(page *rod.Page) (*rod.Element, error) {
	// 尝试多种选择器
	selectors := []string{
		".tabs .tab:contains('收藏')",
		".tab:contains('收藏')",
		"[role='tab']:contains('收藏')",
		".user-tab:contains('收藏')",
	}

	for _, selector := range selectors {
		el, err := page.Element(selector)
		if err == nil && el != nil {
			text, _ := el.Text()
			if text == "收藏" || text == "收藏" {
				logrus.Infof("找到收藏标签：%s", selector)
				return el, nil
			}
		}
	}

	// 如果精确匹配失败，尝试模糊匹配
	elements, err := page.Elements(".tab")
	if err != nil {
		return nil, err
	}

	for _, el := range elements {
		text, _ := el.Text()
		if text == "收藏" || text == "收藏" {
			logrus.Info("通过遍历找到收藏标签")
			return el, nil
		}
	}

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
	script := `() => {
		try {
			if (window.__INITIAL_STATE__ && 
			    window.__INITIAL_STATE__.user && 
			    window.__INITIAL_STATE__.user.collectData) {
				return JSON.stringify(window.__INITIAL_STATE__.user.collectData);
			}
			if (window.__INITIAL_STATE__ && 
			    window.__INITIAL_STATE__.user && 
			    window.__INITIAL_STATE__.user.collect) {
				return JSON.stringify(window.__INITIAL_STATE__.user.collect);
			}
			return "null";
		} catch(e) {
			return "null";
		}
	}`

	result := page.MustEval(script).String()
	if result == "null" || result == "" {
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
	cards, err := page.Elements(".note-item")
	if err != nil {
		return nil, fmt.Errorf("未找到笔记卡片：%w", err)
	}

	for _, card := range cards {
		item := FavoriteItem{}

		// 获取标题
		if titleEl, err := card.Element(".title"); err == nil {
			item.Title, _ = titleEl.Text()
		}

		// 获取封面图
		if imgEl, err := card.Element("img"); err == nil {
			item.CoverURL, _ = imgEl.Attribute("src")
		}

		// 获取用户信息
		if userEl, err := card.Element(".user-name"); err == nil {
			item.UserNickname, _ = userEl.Text()
		}

		// 获取互动数据
		if likeEl, err := card.Element(".like-count"); err == nil {
			text, _ := likeEl.Text()
			// 解析点赞数...
			_ = text
		}

		if item.FeedID != "" || item.Title != "" {
			items = append(items, item)
		}
	}

	return items, nil
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
