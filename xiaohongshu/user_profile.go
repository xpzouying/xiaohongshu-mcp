package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
)

// ProfileTab 个人主页的子 tab。
type ProfileTab string

const (
	TabNotes     ProfileTab = "note"
	TabFavorites ProfileTab = "fav"
	TabLiked     ProfileTab = "liked"
)

// ParseProfileTab 解析 tab 名，空值默认为「笔记」。
func ParseProfileTab(s string) (ProfileTab, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "", "note", "notes", "笔记":
		return TabNotes, nil
	case "fav", "favorites", "favorite", "收藏":
		return TabFavorites, nil
	case "liked", "like", "点赞":
		return TabLiked, nil
	}
	return "", fmt.Errorf("未知的主页 tab %q，可选：note / fav / liked", s)
}

// tabLabel 子 tab 对应的页面文字。
var tabLabel = map[ProfileTab]string{
	TabNotes:     "笔记",
	TabFavorites: "收藏",
	TabLiked:     "点赞",
}

type UserProfileAction struct {
	page *rod.Page
}

func NewUserProfileAction(page *rod.Page) *UserProfileAction {
	pp := page.Timeout(60 * time.Second)
	return &UserProfileAction{page: pp}
}

// UserProfile 获取用户基本信息及指定 tab 下的帖子
func (u *UserProfileAction) UserProfile(ctx context.Context, userID, xsecToken string, tab ProfileTab) (*UserProfileResponse, error) {
	page := u.page.Context(ctx).Timeout(60 * time.Second) // 重设被 .Context 清掉的 deadline

	searchURL := makeUserProfileURL(userID, xsecToken, tab)
	page.MustNavigate(searchURL)
	page.MustWaitStable()

	return u.extractUserProfileData(page, tab)
}

// extractUserProfileData 从页面中提取用户资料数据的通用方法
func (u *UserProfileAction) extractUserProfileData(page *rod.Page, tab ProfileTab) (*UserProfileResponse, error) {
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

	// 2. 获取用户帖子及当前 tab：window.__INITIAL_STATE__.user
	notesResult := page.MustEval(`() => {
		const u = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user;
		if (!u || !u.notes) return "";
		const unwrap = (o) => (o && o.value !== undefined) ? o.value : (o && o._value);
		const notes = unwrap(u.notes);
		if (!notes) return "";
		const active = unwrap(u.activeTab) || {};
		return JSON.stringify({notes: notes, index: active.index || 0, query: active.query || ""});
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

	var notesData struct {
		Notes [][]Feed `json:"notes"`
		Index int      `json:"index"`
		Query string   `json:"query"`
	}
	if err := json.Unmarshal([]byte(notesResult), &notesData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notes: %w", err)
	}

	// tab 不符时报错，避免把别的 tab 的内容当成结果返回
	want := tab
	if want == "" {
		want = TabNotes
	}
	if notesData.Query != "" && ProfileTab(notesData.Query) != want {
		return nil, fmt.Errorf("当前 tab 为 %q，与请求的 %q 不符", notesData.Query, want)
	}

	// 组装响应
	response := &UserProfileResponse{
		UserBasicInfo: userPageData.BasicInfo,
		Interactions:  userPageData.Interactions,
	}

	// 每个 tab 的内容存在各自的下标里，只取当前 tab 的，避免混入其他 tab
	if notesData.Index >= 0 && notesData.Index < len(notesData.Notes) {
		response.Feeds = append(response.Feeds, notesData.Notes[notesData.Index]...)
	}

	return response, nil
}

func makeUserProfileURL(userID, xsecToken string, tab ProfileTab) string {
	url := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?xsec_token=%s&xsec_source=pc_note", userID, xsecToken)
	if tab != "" && tab != TabNotes {
		url += fmt.Sprintf("&tab=%s&subTab=note", tab)
	}
	return url
}

func (u *UserProfileAction) GetMyProfileViaSidebar(ctx context.Context, tab ProfileTab) (*UserProfileResponse, error) {
	page := u.page.Context(ctx).Timeout(60 * time.Second) // 重设被 .Context 清掉的 deadline

	// 创建导航动作
	navigate := NewNavigate(page)

	// 通过侧边栏导航到个人主页
	if err := navigate.ToProfilePage(ctx); err != nil {
		return nil, fmt.Errorf("failed to navigate to profile page via sidebar: %w", err)
	}

	// 等待页面加载完成并获取 __INITIAL_STATE__
	page.MustWaitStable()

	if err := u.selectTab(ctx, page, tab); err != nil {
		return nil, err
	}

	return u.extractUserProfileData(page, tab)
}

// selectTab 切到目标子 tab。「笔记」是默认 tab，无需点击。
func (u *UserProfileAction) selectTab(ctx context.Context, page *rod.Page, tab ProfileTab) error {
	if tab == "" || tab == TabNotes {
		return nil
	}

	label := tabLabel[tab]
	elems, err := page.Elements(`.reds-tab-item.sub-tab-list`)
	if err != nil {
		return fmt.Errorf("未找到主页子 tab: %w", err)
	}

	for _, elem := range elems {
		text, err := elem.Text()
		if err != nil || strings.TrimSpace(text) != label {
			continue
		}
		humanize.Delay(ctx, humanize.BeforeClick)
		if err := humanize.Click(elem); err != nil {
			return fmt.Errorf("切换到 %s 失败: %w", label, err)
		}
		humanize.Delay(ctx, humanize.AfterClick)
		page.MustWaitStable()
		return nil
	}
	return fmt.Errorf("未找到子 tab %q", label)
}
