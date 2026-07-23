package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

type SearchUserAction struct {
	page *rod.Page
}

// NewSearchUserAction 创建用户搜索动作
func NewSearchUserAction(page *rod.Page) *SearchUserAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchUserAction{page: pp}
}

// SearchUsers 根据关键词搜索用户
func (s *SearchUserAction) SearchUsers(ctx context.Context, keyword string) ([]SearchUser, error) {
	page := s.page.Context(ctx)

	// 打开搜索页，默认进入笔记搜索结果
	searchURL := makeSearchURL(keyword)
	page.MustNavigate(searchURL)
	page.MustWaitStable()
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 切换到用户搜索结果
	userTab := page.MustElement(`div#user.channel`)
	userTab.MustClick()

	// 等待用户列表加载完成，或者页面明确显示没有相关用户
	page.MustWait(`() => {
		const bodyText = document.body ? document.body.innerText : '';
		if (bodyText.includes('没找到相关用户')) return true;

		const search = window.__INITIAL_STATE__?.search;
		const userLists = search?.userLists;
		const users = userLists ? (userLists.value !== undefined ? userLists.value : userLists._value) : undefined;

		return Array.isArray(users) && users.length > 0;
	}`)

	// 从页面初始状态中读取用户列表
	result := page.MustEval(`() => {
		const userLists = window.__INITIAL_STATE__?.search?.userLists;
		const users = userLists?.value !== undefined ? userLists.value : userLists?._value;
		if (users) {
			return JSON.stringify(users);
		}
		return "";
	}`).String()

	if result == "" {
		return nil, fmt.Errorf("search.userLists.value not found in __INITIAL_STATE__")
	}

	var users []SearchUser
	if err := json.Unmarshal([]byte(result), &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}

	return users, nil
}
