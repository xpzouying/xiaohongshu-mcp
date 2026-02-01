package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

type UserProfileAction struct {
	page *rod.Page
}

func NewUserProfileAction(page *rod.Page) *UserProfileAction {
	pp := page.Timeout(60 * time.Second)
	return &UserProfileAction{page: pp}
}

// UserProfile 获取用户基本信息及帖子
func (u *UserProfileAction) UserProfile(ctx context.Context, userID, xsecToken string) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	searchURL := makeUserProfileURL(userID, xsecToken)
	page.MustNavigate(searchURL)
	page.MustWaitStable()

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

// MyNoteList 获取我的小红书笔记内容（简化版，只需要 userID）
//
// 与 UserProfile 的区别：
//   - UserProfile: 需要 userID + xsec_token 两个参数，用于访问他人主页
//   - MyNoteList: 只需要 userID 一个参数，用于获取我的小红书笔记内容
//
// 原理：小红书允许用户访问自己的主页时，URL 中不需要 xsec_token 参数
// 例如：https://www.xiaohongshu.com/user/profile/{userID}
//
// 参数：
//   - userID: 小红书用户 ID（自己的用户 ID）
//
// 返回：
//   - *UserProfileResponse: 包含用户基本信息、互动数据（关注/粉丝/获赞）和笔记列表
//   - error: 如果获取失败则返回错误
func (u *UserProfileAction) MyNoteList(ctx context.Context, userID string) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	profileURL := makeMyNoteListURL(userID)
	page.MustNavigate(profileURL)
	page.MustWaitStable()

	return u.extractUserProfileData(page)
}

// makeMyNoteListURL 生成获取我的小红书笔记内容的 URL（不需要 xsec_token）
//
// 与 makeUserProfileURL 的区别：
//   - makeUserProfileURL: 生成带 xsec_token 的 URL，用于访问他人主页
//   - makeMyNoteListURL: 生成不带 xsec_token 的 URL，用于获取我的小红书笔记内容
//
// 参数：
//   - userID: 小红书用户 ID
//
// 返回：
//   - string: 格式为 https://www.xiaohongshu.com/user/profile/{userID}
func makeMyNoteListURL(userID string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s", userID)
}

// OwnProfile 获取自己的主页信息（基本信息 + 互动数据，不包含笔记）
//
// 与其他方法的区别：
//   - UserProfile: 需要 userID + xsec_token，返回完整信息（基本信息 + 互动数据 + 笔记）
//   - MyNoteList: 只需要 userID，只返回笔记内容
//   - OwnProfile: 只需要 userID，只返回基本信息 + 互动数据
//
// 原理：访问自己的主页不需要 xsec_token
//
// 参数：
//   - userID: 小红书用户 ID（自己的用户 ID）
//
// 返回：
//   - *UserProfileResponse: 包含用户基本信息和互动数据（关注/粉丝/获赞）
//   - error: 如果获取失败则返回错误
func (u *UserProfileAction) OwnProfile(ctx context.Context, userID string) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	profileURL := makeMyNoteListURL(userID)
	page.MustNavigate(profileURL)
	page.MustWaitStable()

	return u.extractUserProfileData(page)
}
