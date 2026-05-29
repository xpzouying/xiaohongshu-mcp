package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

const (
	maxNoteScrollAttempts = 200 // 最大滚动尝试次数
	noteStagnantLimit     = 10  // 连续无新笔记时停止
)

// NoteScrollConfig 笔记滚动加载配置
type NoteScrollConfig struct {
	LoadAllNotes bool
	ScrollSpeed  string
}

func DefaultNoteScrollConfig() NoteScrollConfig {
	return NoteScrollConfig{
		LoadAllNotes: false,
		ScrollSpeed:  "normal",
	}
}

type UserProfileAction struct {
	page *rod.Page
}

func NewUserProfileAction(page *rod.Page) *UserProfileAction {
	pp := page.Timeout(5 * time.Minute)
	return &UserProfileAction{page: pp}
}

// UserProfile 获取用户基本信息及帖子
func (u *UserProfileAction) UserProfile(ctx context.Context, userID, xsecToken string, config NoteScrollConfig) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	searchURL := makeUserProfileURL(userID, xsecToken)
	page.MustNavigate(searchURL)
	page.MustWaitStable()

	if config.LoadAllNotes {
		u.scrollToLoadAllNotes(page, config.ScrollSpeed)
	}

	return u.extractUserProfileData(page)
}

// scrollToLoadAllNotes 滚动页面加载全部笔记
func (u *UserProfileAction) scrollToLoadAllNotes(page *rod.Page, scrollSpeed string) {
	logrus.Info("开始滚动加载全部笔记...")

	scrollInterval := getScrollInterval(scrollSpeed)
	lastCount := getNoteCount(page)
	stagnantChecks := 0
	lastScrollTop := 0

	logrus.Infof("初始笔记数: %d", lastCount)

	for attempt := 0; attempt < maxNoteScrollAttempts; attempt++ {
		_, scrollDelta, currentScrollTop := humanScroll(page, scrollSpeed, false, 1)
		time.Sleep(scrollInterval)

		currentCount := getNoteCount(page)

		if currentCount > lastCount {
			logrus.Infof("加载新笔记: %d -> %d", lastCount, currentCount)
			lastCount = currentCount
			stagnantChecks = 0
		} else if scrollDelta < minScrollDelta || currentScrollTop == lastScrollTop {
			stagnantChecks++
		}

		lastScrollTop = currentScrollTop

		if stagnantChecks >= noteStagnantLimit {
			logrus.Infof("笔记加载完成，共 %d 条", currentCount)
			return
		}

		// 停滞时尝试大幅滚动
		if stagnantChecks > 0 && stagnantChecks%5 == 0 {
			humanScroll(page, scrollSpeed, true, 3)
			time.Sleep(scrollInterval)
		}
	}

	logrus.Infof("达到最大滚动次数，已加载 %d 条笔记", getNoteCount(page))
}

// getNoteCount 通过 __INITIAL_STATE__ 获取当前已加载的笔记数
func getNoteCount(page *rod.Page) int {
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.user &&
		    window.__INITIAL_STATE__.user.notes) {
			const notes = window.__INITIAL_STATE__.user.notes;
			const data = notes.value !== undefined ? notes.value : notes._value;
			if (data && Array.isArray(data)) {
				let count = 0;
				for (const arr of data) {
					if (Array.isArray(arr)) {
						count += arr.length;
					}
				}
				return count;
			}
		}
		return 0;
	}`)
	return result.Int()
}

// extractUserProfileData 从页面中提取用户资料数据的通用方法
func (u *UserProfileAction) extractUserProfileData(page *rod.Page) (*UserProfileResponse, error) {
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	userDataResult := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.user &&
		    window.__INITIAL_STATE__.user.userPageData) {
			const userPageData = window.__INITIAL_STATE__.user.userPageData;
			const data = userPageData.value !== undefined ? userPageData.value : userPageData._value;
			if (data) {
				return JSON.stringify(data);
			}
		}
		return "";
	}`).String()

	if userDataResult == "" {
		return nil, fmt.Errorf("user.userPageData.value not found in __INITIAL_STATE__")
	}

	// 2. 获取用户帖子：window.__INITIAL_STATE__.user.notes.value
	notesResult := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.user &&
		    window.__INITIAL_STATE__.user.notes) {
			const notes = window.__INITIAL_STATE__.user.notes;
			// 优先使用 value（getter），如果不存在则使用 _value（内部字段）
			const data = notes.value !== undefined ? notes.value : notes._value;
			if (data) {
				return JSON.stringify(data);
			}
		}
		return "";
	}`).String()

	if notesResult == "" {
		return nil, fmt.Errorf("user.notes.value not found in __INITIAL_STATE__")
	}

	// 解析用户信息
	var userPageData struct {
		Interactions []UserInteractions `json:"interactions"`
		BasicInfo    UserBasicInfo      `json:"basicInfo"`
	}
	if err := json.Unmarshal([]byte(userDataResult), &userPageData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal userPageData: %w", err)
	}

	// 解析帖子数据（帖子为双重数组）
	var notesFeeds [][]Feed
	if err := json.Unmarshal([]byte(notesResult), &notesFeeds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notes: %w", err)
	}

	// 组装响应
	response := &UserProfileResponse{
		UserBasicInfo: userPageData.BasicInfo,
		Interactions:  userPageData.Interactions,
	}

	// 添加用户帖子（展平双重数组）
	for _, feeds := range notesFeeds {
		if len(feeds) != 0 {
			response.Feeds = append(response.Feeds, feeds...)
		}
	}

	return response, nil
}

func makeUserProfileURL(userID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?xsec_token=%s&xsec_source=pc_note", userID, xsecToken)
}

func (u *UserProfileAction) GetMyProfileViaSidebar(ctx context.Context) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	// 创建导航动作
	navigate := NewNavigate(page)

	// 通过侧边栏导航到个人主页
	if err := navigate.ToProfilePage(ctx); err != nil {
		return nil, fmt.Errorf("failed to navigate to profile page via sidebar: %w", err)
	}

	// 等待页面加载完成并获取 __INITIAL_STATE__
	page.MustWaitStable()

	return u.extractUserProfileData(page)
}
