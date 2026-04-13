package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// ==================== 服务层方法 ====================

// GetFavoriteListService 获取收藏列表（复用已有方法）
func (s *XiaohongshuService) GetFavoriteListService(ctx context.Context, maxItems int) (*xiaohongshu.FavoriteListResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewFavoriteListAction(page)

	// 如果 maxItems > 0，需要滚动获取更多
	if maxItems > 0 {
		return action.GetFavoriteList(ctx, "", maxItems)
	}

	return action.GetFavoriteList(ctx, "", 20)
}

// ClassifyNoteResult 单条笔记分类结果
type ClassifyNoteResult struct {
	FeedID       string  `json:"feed_id"`
	XsecToken    string  `json:"xsec_token"`
	Title        string  `json:"title"`
	Desc         string  `json:"desc"`
	Category     string  `json:"category"`
	Confidence   float64 `json:"confidence"`
	UserNickname string  `json:"user_nickname"`
}

// ClassifyFavoritesResult 分类结果
type ClassifyFavoritesResult struct {
	Total      int                             `json:"total"`
	Categories map[string][]ClassifyNoteResult `json:"categories"`
	Stats      map[string]int                  `json:"stats"`
}

// ClassifyFavorites 使用 LLM API 对收藏笔记进行 AI 分类
func (s *XiaohongshuService) ClassifyFavorites(ctx context.Context, items []xiaohongshu.FavoriteItem, categories []string, batchSize int) (*ClassifyFavoritesResult, error) {
	apiKey := os.Getenv("XHS_CLASSIFY_API_KEY")
	apiURL := os.Getenv("XHS_CLASSIFY_API_URL")
	model := os.Getenv("XHS_CLASSIFY_MODEL")

	if apiKey == "" || apiURL == "" {
		// 回退到内置关键词分类
		logrus.Warn("未配置 XHS_CLASSIFY_API_KEY/XHS_CLASSIFY_API_URL，使用内置关键词分类")
		return s.classifyWithKeywords(items, categories)
	}

	if model == "" {
		model = "gpt-4o-mini"
	}

	if batchSize <= 0 {
		batchSize = 10
	}

	logrus.Infof("开始 AI 分类，共 %d 条笔记，批次大小 %d", len(items), batchSize)

	result := &ClassifyFavoritesResult{
		Total:      len(items),
		Categories: make(map[string][]ClassifyNoteResult),
		Stats:      make(map[string]int),
	}

	// 分批处理
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		logrus.Infof("处理第 %d/%d 批 (%d 条笔记)", i/batchSize+1, (len(items)+batchSize-1)/batchSize, len(batch))

		classified, err := s.callLLMForClassification(ctx, batch, categories, apiKey, apiURL, model)
		if err != nil {
			logrus.Warnf("LLM 分类失败，回退到关键词分类: %v", err)
			classified = s.classifyBatchWithKeywords(batch, categories)
		}

		for _, cr := range classified {
			result.Categories[cr.Category] = append(result.Categories[cr.Category], cr)
			result.Stats[cr.Category]++
		}

		// 避免 API 限速
		if end < len(items) {
			time.Sleep(2 * time.Second)
		}
	}

	return result, nil
}

// callLLMForClassification 调用 LLM API 进行批量分类
func (s *XiaohongshuService) callLLMForClassification(ctx context.Context, items []xiaohongshu.FavoriteItem, categories []string, apiKey, apiURL, model string) ([]ClassifyNoteResult, error) {
	// 构建笔记摘要
	var notes []string
	for _, item := range items {
		title := item.Title
		if title == "" {
			title = "(无标题)"
		}
		desc := item.Desc
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		notes = append(notes, fmt.Sprintf("[%s] 标题: %s | 描述: %s", item.FeedID, title, desc))
	}

	catList := strings.Join(categories, "、")

	systemPrompt := `你是一个小红书内容分类专家。请将给定的笔记分类到最合适的类别中。
要求：
1. 只使用提供的类别名称，不要创建新类别
2. 如果不确定，分到"其他"
3. 返回严格的 JSON 数组格式，不要有其他内容
4. 每条评论的 index 对应输入笔记的顺序（从0开始）`

	userPrompt := fmt.Sprintf(`类别列表: %s

请对以下笔记进行分类:

%s

返回格式（JSON 数组，每个元素包含 index、category、confidence）:
[
  {"index": 0, "category": "类别名", "confidence": 0.8},
  {"index": 1, "category": "类别名", "confidence": 0.6}
]`, catList, strings.Join(notes, "\n"))

	// 构建请求
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
		"max_tokens":  4000,
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 LLM API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LLM API 返回错误 (status=%d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &llmResp); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回空响应")
	}

	content := llmResp.Choices[0].Message.Content

	// 提取 JSON 数组（去除 markdown code block）
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 3 {
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	// 解析分类结果
	var classifications []struct {
		Index      int     `json:"index"`
		Category   string  `json:"category"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(content), &classifications); err != nil {
		return nil, fmt.Errorf("解析分类结果失败: %w, content: %s", err, content)
	}

	// 映射回 ClassifyNoteResult
	results := make([]ClassifyNoteResult, 0, len(items))
	for _, c := range classifications {
		if c.Index < 0 || c.Index >= len(items) {
			continue
		}
		item := items[c.Index]
		category := c.Category
		if category == "" {
			category = "其他"
		}

		results = append(results, ClassifyNoteResult{
			FeedID:       item.FeedID,
			XsecToken:    item.XsecToken,
			Title:        item.Title,
			Desc:         item.Desc,
			Category:     category,
			Confidence:   c.Confidence,
			UserNickname: item.UserNickname,
		})
	}

	logrus.Infof("LLM 分类完成，成功分类 %d/%d 条", len(results), len(items))
	return results, nil
}

// classifyWithKeywords 使用内置关键词分类
func (s *XiaohongshuService) classifyWithKeywords(items []xiaohongshu.FavoriteItem, categories []string) (*ClassifyFavoritesResult, error) {
	result := &ClassifyFavoritesResult{
		Total:      len(items),
		Categories: make(map[string][]ClassifyNoteResult),
		Stats:      make(map[string]int),
	}

	classified := s.classifyBatchWithKeywords(items, categories)
	for _, cr := range classified {
		result.Categories[cr.Category] = append(result.Categories[cr.Category], cr)
		result.Stats[cr.Category]++
	}

	return result, nil
}

// classifyBatchWithKeywords 关键词分类一批笔记
func (s *XiaohongshuService) classifyBatchWithKeywords(items []xiaohongshu.FavoriteItem, userCategories []string) []ClassifyNoteResult {
	results := make([]ClassifyNoteResult, 0, len(items))

	// 如果有用户自定义分类，使用自定义规则；否则使用默认规则
	if len(userCategories) > 0 {
		for _, item := range items {
			results = append(results, ClassifyNoteResult{
				FeedID:       item.FeedID,
				XsecToken:    item.XsecToken,
				Title:        item.Title,
				Desc:         item.Desc,
				Category:     "其他",
				Confidence:   0,
				UserNickname: item.UserNickname,
			})
		}
		return results
	}

	// 使用默认关键词分类
	for _, item := range items {
		cat, conf := keywordClassify(item.Title, item.Desc)
		results = append(results, ClassifyNoteResult{
			FeedID:       item.FeedID,
			XsecToken:    item.XsecToken,
			Title:        item.Title,
			Desc:         item.Desc,
			Category:     cat,
			Confidence:   conf,
			UserNickname: item.UserNickname,
		})
	}

	return results
}

// ==================== 关键词分类规则 ====================

var keywordRules = map[string][]string{
	"美食烹饪": {"吃", "菜", "汤", "早餐", "煎", "美食", "做饭", "炒", "炖", "煮", "烘焙", "食谱", "好吃", "美味", "做法"},
	"育儿母婴": {"宝宝", "育儿", "婴儿", "儿童", "母婴", "发型", "月龄", "咳嗽", "哄睡", "新生儿", "孕妈", "宝妈", "带娃", "辅食"},
	"汽车交通": {"车", "汽车", "提车", "驾驶", "停车", "驾照", "车主", "砍价", "选号"},
	"股票理财": {"股票", "炒股", "基金", "理财", "涨停", "投资", "etf", "证券", "开户", "赚钱"},
	"电商创业": {"拼多多", "电商", "无货源", "店", "创业", "副业", "开店", "变现"},
	"健康医疗": {"健康", "医生", "医院", "治疗", "症状", "药物", "疫苗", "体检", "心理"},
	"家居生活": {"床", "家居", "装修", "收纳", "整理"},
	"技能学习": {"教程", "学习", "技巧", "方法", "指南", "新手"},
	"娱乐搞笑": {"搞笑", "幽默", "笑死", "哈哈"},
}

func keywordClassify(title, desc string) (string, float64) {
	text := strings.ToLower(title + " " + desc)
	titleLower := strings.ToLower(title)

	type score struct {
		category string
		score    float64
		matched  int
	}

	var scores []score
	for category, keywords := range keywordRules {
		s := score{category: category}
		for _, kw := range keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				s.score += 2
				s.matched++
			}
			if strings.Contains(titleLower, strings.ToLower(kw)) {
				s.score *= 1.3
			}
		}
		if s.matched >= 2 {
			s.score *= 1.5
		}
		if s.matched >= 3 {
			s.score *= 1.5
		}
		if s.score > 0 {
			scores = append(scores, s)
		}
	}

	if len(scores) == 0 {
		return "其他", 0.0
	}

	best := scores[0]
	for _, s := range scores {
		if s.score > best.score {
			best = s
		}
	}

	conf := best.score / 30.0
	if conf > 1.0 {
		conf = 1.0
	}

	return best.category, conf
}

// ==================== 专辑同步 ====================

// AlbumSyncStats 专辑同步统计
type AlbumSyncStats struct {
	TotalAlbums  int            `json:"total_albums"`
	SuccessCount int            `json:"success_count"`
	FailedAlbums map[string]int `json:"failed_albums"`
}

// ApplyClassificationToAlbums 将预分类结果同步到专辑
// 使用 UI 自动化方式创建专辑（绕过 API 签名问题）
func (s *XiaohongshuService) ApplyClassificationToAlbums(ctx context.Context, classified *ClassifyFavoritesResult, autoCreateAlbums bool) (*AlbumSyncStats, error) {
	stats := &AlbumSyncStats{
		TotalAlbums:  len(classified.Categories),
		FailedAlbums: make(map[string]int),
	}

	if !autoCreateAlbums {
		logrus.Info("⏭️  autoCreateAlbums=false，跳过专辑创建")
		return stats, nil
	}

	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	// 使用 UI 自动化服务创建专辑
	uiService := xiaohongshu.NewAlbumUIService(page)

	for category, notes := range classified.Categories {
		if category == "其他" {
			logrus.Infof("  ⏭️  跳过「其他」分类 (%d 条)", len(notes))
			continue
		}

		logrus.Infof("  📁 【%s】 (%d 条)", category, len(notes))

		// 通过 UI 自动化创建专辑
		if err := uiService.CreateAlbumViaUI(category); err != nil {
			logrus.Warnf("    ⚠️ 创建专辑失败: %v", err)
			stats.FailedAlbums[category] = len(notes)
		} else {
			logrus.Infof("    ✅ 专辑创建成功: %s", category)
			stats.SuccessCount++
		}

		time.Sleep(3 * time.Second)
	}

	return stats, nil
}

// ==================== 专辑管理 ====================

// AlbumListResult 专辑列表结果
type AlbumListResult struct {
	Albums []xiaohongshu.AlbumInfo `json:"albums"`
	Count  int                     `json:"count"`
}

// GetAlbumList 获取专辑列表
func (s *XiaohongshuService) GetAlbumList(ctx context.Context) (*AlbumListResult, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	albumService := xiaohongshu.NewAlbumService(page)
	albums, err := albumService.GetAlbumList(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取专辑列表失败: %w", err)
	}

	return &AlbumListResult{
		Albums: albums,
		Count:  len(albums),
	}, nil
}

// CreateAlbumResult 创建专辑结果
type CreateAlbumResult struct {
	AlbumID string `json:"album_id"`
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CreateAlbumService 创建专辑
func (s *XiaohongshuService) CreateAlbumService(ctx context.Context, name string) (*CreateAlbumResult, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	albumService := xiaohongshu.NewAlbumService(page)
	albumID, err := albumService.CreateAlbum(ctx, name)
	if err != nil {
		return &CreateAlbumResult{Name: name, Success: false, Message: err.Error()}, err
	}

	return &CreateAlbumResult{
		AlbumID: albumID,
		Name:    name,
		Success: true,
		Message: "专辑创建成功",
	}, nil
}
