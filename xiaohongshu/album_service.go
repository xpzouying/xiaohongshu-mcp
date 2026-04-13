package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// AlbumService 专辑同步服务
// 原理：在浏览器页面上下文中执行 fetch 调用专辑 API
// 浏览器自动携带 cookies 和签名，无需手动处理
type AlbumService struct {
	page *rod.Page
}

// NewAlbumService 创建专辑服务
func NewAlbumService(page *rod.Page) *AlbumService {
	return &AlbumService{page: page}
}

// AlbumResult 专辑操作结果
type AlbumResult struct {
	Success bool        `json:"success"`
	Code    float64     `json:"code"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data"`
}

// evalInPage 在页面上下文中执行 JS（通过 proto.RuntimeEvaluate 直接注入 script 标签）
func (s *AlbumService) evalInPage(js string) error {
	// 使用 proto.RuntimeEvaluate 直接执行，绕过 page.Eval 的参数传递问题
	_, err := proto.RuntimeEvaluate{
		Expression: fmt.Sprintf(`(function() {
			var s = document.createElement('script');
			s.textContent = %s;
			document.head.appendChild(s);
		})()`, jsonString(js)),
	}.Call(s.page)
	return err
}

// jsonString 将 Go 字符串转为 JS 字符串字面量
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// waitForResult 等待页面脚本执行结果
func (s *AlbumService) waitForResult(key string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := proto.RuntimeEvaluate{
			Expression: fmt.Sprintf(`window.%s || ""`, key),
		}.Call(s.page)
		if err == nil && res.Result != nil {
			val := res.Result.Value.String()
			if val != "" && val != "null" {
				return val, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("等待结果超时 (key=%s)", key)
}

// callAPIScript 生成页面上下文中执行 fetch 的 JS 代码（使用 XMLHttpRequest 绕过页面的 monkey-patch fetch）
func callAPIScript(key, method, url, bodyJSON string) string {
	return fmt.Sprintf(`
		(function() {
			window.%s = null;
			(function() {
				try {
					var xhr = new XMLHttpRequest();
					xhr.open('%s', '%s', true);
					xhr.setRequestHeader('Content-Type', 'application/json;charset=UTF-8');
					xhr.withCredentials = true;
					xhr.onreadystatechange = function() {
						if (xhr.readyState === 4) {
							try {
								var d = JSON.parse(xhr.responseText);
								window.%s = JSON.stringify({httpStatus: xhr.status, success: d.success, code: d.code, msg: d.msg || d.code_msg || '', data: d.data});
							} catch(e) {
								window.%s = JSON.stringify({httpStatus: xhr.status, success: false, code: -1, msg: 'parse error: ' + xhr.responseText, data: null});
							}
						}
					};
					xhr.onerror = function() {
						window.%s = JSON.stringify({httpStatus: 0, success: false, code: -1, msg: 'network error: status=' + xhr.status, data: null});
					};
					if (%s && %s !== 'null') {
						xhr.send(%s);
					} else {
						xhr.send();
					}
				} catch(e) {
					window.%s = JSON.stringify({httpStatus: 0, success: false, code: -1, msg: e.message, data: null});
				}
			})();
		})()
	`, key, method, url, key, key, key, bodyJSON, bodyJSON, bodyJSON, key)
}

// ensurePageReady 确保页面已正确加载（非 Security Verification 页面）
func (s *AlbumService) ensurePageReady(ctx context.Context) error {
	// 导航到首页
	if err := s.page.Timeout(30 * time.Second).Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}

	// 等待页面加载
	time.Sleep(3 * time.Second)

	// 检查是否是 Security Verification 页面
	title := s.page.MustInfo().Title
	if strings.Contains(title, "Security") || strings.Contains(title, "验证") {
		return fmt.Errorf("页面显示安全验证，cookies 可能已过期 (标题: %s)", title)
	}

	// 检查登录状态（使用 XHR 绕过页面的 monkey-patch fetch）
	script := `
		(function() {
			window.__loginCheck = null;
			(function() {
				try {
					var xhr = new XMLHttpRequest();
					xhr.open('GET', 'https://edith.xiaohongshu.com/api/sns/web/v2/user/me', true);
					xhr.withCredentials = true;
					xhr.onreadystatechange = function() {
						if (xhr.readyState === 4) {
							try {
								var d = JSON.parse(xhr.responseText);
								window.__loginCheck = JSON.stringify({success: d.success, code: d.code, msg: d.msg || d.code_msg});
							} catch(e) {
								window.__loginCheck = JSON.stringify({error: e.message});
							}
						}
					};
					xhr.send();
				} catch(e) {
					window.__loginCheck = JSON.stringify({error: e.message});
				}
			})();
		})();
	`
	if err := s.evalInPage(script); err != nil {
		return fmt.Errorf("注入登录检查脚本失败: %w", err)
	}

	result, err := s.waitForResult("__loginCheck", 10*time.Second)
	if err != nil {
		return fmt.Errorf("等待登录检查结果超时: %w", err)
	}

	var loginCheck map[string]interface{}
	json.Unmarshal([]byte(result), &loginCheck)

	if code, ok := loginCheck["code"].(float64); ok && code == -101 {
		return fmt.Errorf("未登录或 cookies 已过期: %v", loginCheck["msg"])
	}

	logrus.Info("✅ 页面就绪，登录状态正常")
	return nil
}

// callAPI 在页面上下文中调用 API
func (s *AlbumService) callAPI(method, url string, body interface{}) (*AlbumResult, error) {
	bodyJSON := "null"
	if body != nil {
		b, _ := json.Marshal(body)
		bodyJSON = string(b)
	}

	key := "__apiResult"
	script := callAPIScript(key, method, url, bodyJSON)

	if err := s.evalInPage(script); err != nil {
		return nil, fmt.Errorf("注入 API 调用脚本失败: %w", err)
	}

	result, err := s.waitForResult(key, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("等待 API 响应超时: %w", err)
	}

	var apiResult AlbumResult
	if err := json.Unmarshal([]byte(result), &apiResult); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, raw: %s", err, result)
	}

	return &apiResult, nil
}

// GetAlbumList 获取专辑列表
func (s *AlbumService) GetAlbumList(ctx context.Context) ([]AlbumInfo, error) {
	logrus.Info("获取专辑列表...")

	result, err := s.callAPI("GET", "https://edith.xiaohongshu.com/api/sns/web/v1/folder/list", nil)
	if err != nil {
		return nil, err
	}

	if !result.Success && result.Code != 0 {
		return nil, fmt.Errorf("获取专辑列表失败: %s", result.Msg)
	}

	var albums []AlbumInfo
	if result.Data != nil {
		dataBytes, _ := json.Marshal(result.Data)
		// API 返回结构: {folders: [...]} 或直接是数组
		var folderData struct {
			Folders []AlbumInfo `json:"folders"`
		}
		if err := json.Unmarshal(dataBytes, &folderData); err == nil {
			albums = folderData.Folders
		} else {
			// 尝试直接解析为数组
			json.Unmarshal(dataBytes, &albums)
		}
	}

	logrus.Infof("获取到 %d 个专辑", len(albums))
	return albums, nil
}

// CreateAlbum 创建专辑
func (s *AlbumService) CreateAlbum(ctx context.Context, name string) (string, error) {
	logrus.Infof("创建专辑: %s", name)

	result, err := s.callAPI("POST", "https://edith.xiaohongshu.com/api/sns/web/v1/folder", map[string]interface{}{
		"name": name,
		"type": "collect",
	})
	if err != nil {
		return "", err
	}

	if !result.Success && result.Code != 0 {
		// 可能专辑已存在
		if strings.Contains(result.Msg, "已存在") || strings.Contains(result.Msg, "exist") {
			logrus.Infof("专辑已存在: %s", name)
			return "", nil
		}
		return "", fmt.Errorf("创建专辑失败 (code=%.0f): %s", result.Code, result.Msg)
	}

	var albumID string
	if result.Data != nil {
		dataBytes, _ := json.Marshal(result.Data)
		var dataMap map[string]interface{}
		if err := json.Unmarshal(dataBytes, &dataMap); err == nil {
			if id, ok := dataMap["id"].(string); ok {
				albumID = id
			} else if id, ok := dataMap["folder_id"].(string); ok {
				albumID = id
			}
		}
	}

	logrus.Infof("✅ 专辑创建成功: %s (ID: %s)", name, albumID)
	return albumID, nil
}

// AddNotesToAlbum 批量添加笔记到专辑
func (s *AlbumService) AddNotesToAlbum(ctx context.Context, albumID string, noteIDs []string) (int, int, error) {
	logrus.Infof("添加 %d 条笔记到专辑 %s", len(noteIDs), albumID)

	successCount := 0
	failedCount := 0
	batchSize := 20

	for i := 0; i < len(noteIDs); i += batchSize {
		end := i + batchSize
		if end > len(noteIDs) {
			end = len(noteIDs)
		}
		batch := noteIDs[i:end]
		batchNum := i/batchSize + 1

		logrus.Debugf("  批次 %d: %d 条笔记", batchNum, len(batch))

		result, err := s.callAPI("POST", "https://edith.xiaohongshu.com/api/sns/web/v1/note/collect/batch", map[string]interface{}{
			"folder_id": albumID,
			"note_ids":  batch,
		})
		if err != nil {
			logrus.Warnf("  批次 %d 请求失败: %v", batchNum, err)
			failedCount += len(batch)
			continue
		}

		if result.Success || result.Code == 0 {
			successCount += len(batch)
			logrus.Infof("  批次 %d: 成功", batchNum)
		} else {
			failedCount += len(batch)
			logrus.Warnf("  批次 %d 失败: %s", batchNum, result.Msg)
		}

		// 避免请求过快
		time.Sleep(1500 * time.Millisecond)
	}

	return successCount, failedCount, nil
}

// FindAlbumID 查找专辑 ID（通过专辑名称）
func (s *AlbumService) FindAlbumID(ctx context.Context, name string) (string, error) {
	albums, err := s.GetAlbumList(ctx)
	if err != nil {
		return "", err
	}

	for _, album := range albums {
		if album.Name == name {
			return album.ID, nil
		}
	}
	return "", nil
}

// GetOrCreateAlbum 获取或创建专辑
func (s *AlbumService) GetOrCreateAlbum(ctx context.Context, name string) (string, error) {
	// 先查找
	albumID, err := s.FindAlbumID(ctx, name)
	if err != nil {
		logrus.Warnf("查找专辑失败: %v，尝试创建", err)
	}
	if albumID != "" {
		logrus.Infof("找到已有专辑: %s (ID: %s)", name, albumID)
		return albumID, nil
	}

	// 创建
	return s.CreateAlbum(ctx, name)
}

// SyncCategoriesToAlbums 完整同步流程
func (s *AlbumService) SyncCategoriesToAlbums(ctx context.Context, categoriesFile string) (*SyncResult, error) {
	logrus.Info("🚀 开始专辑同步...")

	// 确保页面就绪
	if err := s.ensurePageReady(ctx); err != nil {
		return nil, err
	}

	// 加载分类结果
	categories, total, err := s.loadCategories(categoriesFile)
	if err != nil {
		return nil, fmt.Errorf("加载分类失败: %w", err)
	}

	result := &SyncResult{
		Total: total,
	}

	logrus.Infof("加载 %d 条笔记，%d 个分类", total, len(categories))

	// 对每个分类执行同步
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

		// 提取笔记 ID
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
			logrus.Warn("  没有有效的笔记 ID")
			continue
		}

		entry := AlbumSyncEntry{
			Name:  category,
			Count: count,
		}

		// 获取或创建专辑
		logrus.Info("  获取/创建专辑...")
		albumID, err := s.GetOrCreateAlbum(ctx, category)
		if err != nil {
			logrus.Warnf("  专辑操作失败: %v", err)
			entry.Message = "失败: " + err.Error()
			result.Albums = append(result.Albums, entry)
			result.Failed++
			continue
		}

		if albumID == "" {
			// 专辑可能存在，再试一次查找
			albumID, err = s.FindAlbumID(ctx, category)
			if err != nil || albumID == "" {
				entry.Message = "无法获取专辑 ID"
				result.Albums = append(result.Albums, entry)
				result.Failed++
				continue
			}
		}

		entry.AlbumID = albumID

		// 添加笔记
		logrus.Info("  添加笔记...")
		successCount, failedCount, err := s.AddNotesToAlbum(ctx, albumID, noteIDs)
		entry.SuccessCount = successCount
		entry.FailedCount = failedCount
		entry.Success = successCount > len(noteIDs)*80/100

		if err != nil {
			entry.Message = "部分失败: " + err.Error()
		} else if entry.Success {
			entry.Message = fmt.Sprintf("成功 %d/%d 条", successCount, len(noteIDs))
			result.Success++
		} else {
			entry.Message = fmt.Sprintf("失败 %d/%d 条", failedCount, len(noteIDs))
			result.Failed++
		}

		result.Albums = append(result.Albums, entry)

		// 分类之间等待
		time.Sleep(2 * time.Second)
	}

	logrus.Infof("\n✅ 同步完成！成功: %d, 失败: %d", result.Success, result.Failed)
	return result, nil
}

// loadCategories 加载分类结果文件
func (s *AlbumService) loadCategories(file string) (map[string]interface{}, int, error) {
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

// ========== 以下类型与 favorite_list.go 等共享 ==========

// AlbumInfo 专辑信息
type AlbumInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SyncResult 同步结果
type SyncResult struct {
	Total   int              `json:"total"`
	Albums  []AlbumSyncEntry `json:"albums"`
	Success int              `json:"success"`
	Failed  int              `json:"failed"`
}

// AlbumSyncEntry 单个专辑同步结果
type AlbumSyncEntry struct {
	Name         string `json:"name"`
	Count        int    `json:"count"`
	AlbumID      string `json:"album_id"`
	Success      bool   `json:"success"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
	Message      string `json:"message"`
}

// CheckPageSecurity 检查页面是否被安全验证拦截
func CheckPageSecurity(page *rod.Page) (bool, string) {
	title := page.MustInfo().Title
	if strings.Contains(title, "Security") || strings.Contains(title, "验证") || strings.Contains(title, "captcha") {
		return true, title
	}
	return false, title
}
