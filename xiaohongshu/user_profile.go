package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// ========== 配置常量 ==========
const (
	defaultProfileMaxAttempts = 20
	profileStagnantLimit      = 15
	profileMinScrollDelta     = 10
	profileLargeScrollTrigger = 5
	profileFinalSprintPush    = 10
)

// ========== 数据结构 ==========

// ProfileLoadConfig 用户主页笔记加载配置
type ProfileLoadConfig struct {
	MaxNoteItems int    // 最大加载笔记数量，0表示加载全部
	ScrollSpeed  string // 滚动速度: slow, normal, fast
}

// DefaultProfileLoadConfig 返回默认配置
func DefaultProfileLoadConfig() ProfileLoadConfig {
	return ProfileLoadConfig{
		MaxNoteItems: 0,
		ScrollSpeed:  "normal",
	}
}

type UserProfileAction struct {
	page *rod.Page
}

func NewUserProfileAction(page *rod.Page) *UserProfileAction {
	pp := page.Timeout(60 * time.Second)
	return &UserProfileAction{page: pp}
}

// UserProfile 获取用户基本信息及帖子（使用默认配置）
func (u *UserProfileAction) UserProfile(ctx context.Context, userID, xsecToken string) (*UserProfileResponse, error) {
	return u.UserProfileWithConfig(ctx, userID, xsecToken, false, DefaultProfileLoadConfig())
}

// UserProfileWithConfig 获取用户基本信息及帖子（支持自定义配置）
func (u *UserProfileAction) UserProfileWithConfig(ctx context.Context, userID, xsecToken string, loadAllNotes bool, config ProfileLoadConfig) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	searchURL := makeUserProfileURL(userID, xsecToken)
	logrus.Infof("打开用户主页: %s", searchURL)
	logrus.Infof("配置: 加载全部=%v, 最大笔记数=%d, 滚动速度=%s",
		loadAllNotes, config.MaxNoteItems, config.ScrollSpeed)

	page.MustNavigate(searchURL)
	page.MustWaitStable()

	// 如果需要加载全部笔记，执行滚动加载
	if loadAllNotes {
		if err := u.loadAllNotesWithConfig(page, config); err != nil {
			logrus.Warnf("加载全部笔记失败: %v", err)
		}
	}

	return u.extractUserProfileData(page)
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

// ========== 笔记加载器 ==========

type noteLoader struct {
	page   *rod.Page
	config ProfileLoadConfig
	stats  *noteLoadStats
	state  *noteLoadState
}

type noteLoadStats struct {
	attempts int
}

type noteLoadState struct {
	lastCount      int
	lastScrollTop  int
	stagnantChecks int
}

func (u *UserProfileAction) loadAllNotesWithConfig(page *rod.Page, config ProfileLoadConfig) error {
	loader := &noteLoader{
		page:   page,
		config: config,
		stats:  &noteLoadStats{},
		state:  &noteLoadState{},
	}

	return loader.load()
}

func (nl *noteLoader) load() error {
	maxAttempts := nl.calculateMaxAttempts()
	scrollInterval := getScrollInterval(nl.config.ScrollSpeed)

	logrus.Info("开始加载笔记...")
	sleepRandom(humanDelayRange.min, humanDelayRange.max)

	for nl.stats.attempts = 0; nl.stats.attempts < maxAttempts; nl.stats.attempts++ {
		logrus.Debugf("=== 尝试 %d/%d ===", nl.stats.attempts+1, maxAttempts)

		if nl.checkComplete() {
			return nil
		}

		currentCount := nl.getNoteCount()
		nl.updateState(currentCount)

		if nl.shouldStopAtTarget(currentCount) {
			return nil
		}

		// 如果笔记数量长时间不变，且已经尝试过大冲刺，则认为已加载完成
		if nl.state.stagnantChecks > profileStagnantLimit+5 {
			logrus.Infof("✓ 笔记数量长时间无变化 (停滞%d次)，认为已加载完成", nl.state.stagnantChecks)
			logrus.Infof("✓ 最终笔记数: %d", currentCount)
			return nil
		}

		nl.performScroll()
		nl.handleStagnation()

		time.Sleep(scrollInterval)
	}

	nl.performFinalSprint()
	return nil
}

func (nl *noteLoader) calculateMaxAttempts() int {
	if nl.config.MaxNoteItems > 0 {
		return nl.config.MaxNoteItems * 2
	}
	return defaultProfileMaxAttempts
}

func (nl *noteLoader) checkComplete() bool {
	// 用户详情页面没有明确的结束标识
	// 主要通过笔记数量停滞来判断是否加载完成
	// 这里只做辅助检查：如果滚动到底部且笔记数量停滞，则认为完成
	if nl.state.stagnantChecks >= stagnantCheckThreshold && nl.checkEndOfNotes() {
		currentCount := nl.getNoteCount()
		logrus.Infof("✓ 检测到已滚动到底部且笔记数量停滞")
		sleepRandom(humanDelayRange.min, humanDelayRange.max)
		logrus.Infof("✓ 加载完成: %d 条笔记, 尝试次数: %d",
			currentCount, nl.stats.attempts+1)
		return true
	}
	return false
}

func (nl *noteLoader) updateState(currentCount int) {
	logrus.Debugf("当前笔记: %d", currentCount)

	if currentCount != nl.state.lastCount {
		logrus.Infof("✓ 笔记增加: %d -> %d (+%d)",
			nl.state.lastCount, currentCount, currentCount-nl.state.lastCount)
		nl.state.lastCount = currentCount
		nl.state.stagnantChecks = 0
	} else {
		nl.state.stagnantChecks++
		if nl.state.stagnantChecks%5 == 0 {
			logrus.Debugf("笔记停滞 %d 次", nl.state.stagnantChecks)
		}
	}
}

func (nl *noteLoader) shouldStopAtTarget(currentCount int) bool {
	if nl.config.MaxNoteItems <= 0 || currentCount < nl.config.MaxNoteItems {
		return false
	}

	if nl.state.stagnantChecks >= stagnantCheckThreshold {
		logrus.Infof("✓ 已达到目标笔记数: %d/%d (停滞%d次), 停止加载",
			currentCount, nl.config.MaxNoteItems, nl.state.stagnantChecks)
		return true
	}

	if nl.state.stagnantChecks > 0 {
		logrus.Debugf("已达目标数 %d/%d，再确认 %d 次...",
			currentCount, nl.config.MaxNoteItems, stagnantCheckThreshold-nl.state.stagnantChecks)
	}

	return false
}

func (nl *noteLoader) performScroll() {
	largeMode := nl.state.stagnantChecks >= profileLargeScrollTrigger
	pushCount := 1
	if largeMode {
		pushCount = 3 + rand.Intn(3)
	}

	_, scrollDelta, currentScrollTop := humanScroll(nl.page, nl.config.ScrollSpeed, largeMode, pushCount)

	if scrollDelta < profileMinScrollDelta || currentScrollTop == nl.state.lastScrollTop {
		nl.state.stagnantChecks++
		if nl.state.stagnantChecks%5 == 0 {
			logrus.Debugf("滚动停滞 %d 次", nl.state.stagnantChecks)
		}
	} else {
		nl.state.stagnantChecks = 0
		nl.state.lastScrollTop = currentScrollTop
	}
}

func (nl *noteLoader) handleStagnation() {
	if nl.state.stagnantChecks >= profileStagnantLimit {
		logrus.Infof("笔记数量停滞 %d 次，尝试大冲刺...", nl.state.stagnantChecks)
		humanScroll(nl.page, nl.config.ScrollSpeed, true, 10)
		sleepRandom(postScrollRange.min, postScrollRange.max)

		// 检查冲刺后笔记数量是否有变化
		currentCount := nl.getNoteCount()
		if currentCount == nl.state.lastCount && nl.checkEndOfNotes() {
			logrus.Infof("✓ 大冲刺后笔记数量仍无变化且已到底部，认为已加载完成")
			logrus.Infof("✓ 最终笔记数: %d", currentCount)
			// 不重置 stagnantChecks，让主循环判断退出
		} else {
			nl.state.stagnantChecks = 0
		}
	}
}

func (nl *noteLoader) performFinalSprint() {
	logrus.Infof("达到最大尝试次数，最后冲刺...")
	humanScroll(nl.page, nl.config.ScrollSpeed, true, profileFinalSprintPush)

	currentCount := nl.getNoteCount()
	hasEnd := nl.checkEndOfNotes()
	logrus.Infof("✓ 加载结束: %d 条笔记, 到达底部: %v",
		currentCount, hasEnd)
}

func (nl *noteLoader) getNoteCount() int {
	result := nl.page.MustEval(`() => {
		// 从 JavaScript 状态中读取实际的笔记数据
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.user &&
		    window.__INITIAL_STATE__.user.notes) {
			const notes = window.__INITIAL_STATE__.user.notes;
			const data = notes.value !== undefined ? notes.value : notes._value;
			if (data && Array.isArray(data)) {
				// notes 是一个二维数组，需要展平后计数（与 extractUserProfileData 逻辑一致）
				let totalCount = 0;
				for (const feeds of data) {
					if (Array.isArray(feeds) && feeds.length !== 0) {
						totalCount += feeds.length;
					}
				}
				return totalCount;
			}
		}
		return 0;
	}`).Int()
	return result
}

func (nl *noteLoader) checkEndOfNotes() bool {
	// 用户详情页面底部没有明确的结束标识
	// 通过检测笔记数量是否停止增长来判断
	// 这个方法主要用于停滞检测，不作为主要的结束判断依据
	result := nl.page.MustEval(`() => {
		// 检查滚动是否到底
		const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
		const scrollHeight = document.documentElement.scrollHeight;
		const clientHeight = document.documentElement.clientHeight;
		return scrollTop + clientHeight >= scrollHeight - 50;
	}`).Bool()
	return result
}

// ========== 侧边栏导航（使用 navigate.go 中的统一方法）==========

// GetMyProfileViaSidebar 通过侧边栏导航到个人主页（使用默认配置）
func (u *UserProfileAction) GetMyProfileViaSidebar(ctx context.Context) (*UserProfileResponse, error) {
	return u.GetMyProfileViaSidebarWithConfig(ctx, false, DefaultProfileLoadConfig())
}

// GetMyProfileViaSidebarWithConfig 通过侧边栏导航到个人主页（支持自定义配置）
func (u *UserProfileAction) GetMyProfileViaSidebarWithConfig(ctx context.Context, loadAllNotes bool, config ProfileLoadConfig) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	logrus.Info("通过侧边栏导航到个人主页")
	logrus.Infof("配置: 加载全部=%v, 最大笔记数=%d, 滚动速度=%s",
		loadAllNotes, config.MaxNoteItems, config.ScrollSpeed)

	// 使用 navigate.go 中的统一导航方法
	navigator := NewNavigate(page)
	if err := navigator.ToProfilePage(ctx); err != nil {
		return nil, fmt.Errorf("failed to navigate to profile page via sidebar: %w", err)
	}

	// 等待页面加载完成
	page.MustWaitStable()

	// 如果需要加载全部笔记，执行滚动加载
	if loadAllNotes {
		if err := u.loadAllNotesWithConfig(page, config); err != nil {
			logrus.Warnf("加载全部笔记失败: %v", err)
		}
	}

	return u.extractUserProfileData(page)
}
