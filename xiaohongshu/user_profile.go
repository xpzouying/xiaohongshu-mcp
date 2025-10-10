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

	// 获取 window.__INITIAL_STATE__ 并转换为 JSON 字符串
	result := page.MustEval(`() => {
			if (window.__INITIAL_STATE__) {
				return JSON.stringify(window.__INITIAL_STATE__);
			}
			return "";
		}`).String()

	if result == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}
	// 定义响应结构并直接反序列化
	var initialState = struct {
		User struct {
			UserPageData UserPageData `json:"userPageData"`
			Notes        struct {
				Feeds [][]Feed `json:"_rawValue"` // 帖子为双重数组
			} `json:"notes"`
		} `json:"user"`
	}{}
	if err := json.Unmarshal([]byte(result), &initialState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}
	response := &UserProfileResponse{
		UserBasicInfo: initialState.User.UserPageData.RawValue.BasicInfo,
		Interactions:  initialState.User.UserPageData.RawValue.Interactions,
	}
	// 添加用户贴子
	for _, feeds := range initialState.User.Notes.Feeds {
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
