package main

import (
	"context"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// XiaohongshuService 小红书业务服务
type XiaohongshuService struct {
	logins loginSessions
}

// NewXiaohongshuService 创建小红书服务实例
func NewXiaohongshuService() *XiaohongshuService {
	return &XiaohongshuService{}
}

// LoginStatusResponse 登录状态响应
type LoginStatusResponse struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	Username   string `json:"username,omitempty"` // 当前登录账号的昵称
	UserID     string `json:"user_id,omitempty"`  // 用户唯一标识（个人主页 URL 中的 ID）
}

// LoginQrcodeResponse 登录扫码二维码
type LoginQrcodeResponse struct {
	Timeout    string `json:"timeout"`
	IsLoggedIn bool   `json:"is_logged_in"`
	Img        string `json:"img,omitempty"`
}

// FeedsListResponse Feeds列表响应
type FeedsListResponse struct {
	Feeds []xiaohongshu.Feed `json:"feeds"`
	Count int                `json:"count"`
}

// UserProfileResponse 用户主页响应
type UserProfileResponse struct {
	UserBasicInfo xiaohongshu.UserBasicInfo      `json:"userBasicInfo"`
	Interactions  []xiaohongshu.UserInteractions `json:"interactions"`
	Feeds         []xiaohongshu.Feed             `json:"feeds"`
}

// CheckLoginStatus 检查登录状态
func (s *XiaohongshuService) CheckLoginStatus(ctx context.Context) (*LoginStatusResponse, error) {
	var response *LoginStatusResponse
	err := sharedBrowser.Do(func(page playwright.Page) error {
		loginAction := xiaohongshu.NewLogin(page)

		isLoggedIn, err := loginAction.CheckLoginStatus(ctx)
		if err != nil {
			return err
		}

		response = &LoginStatusResponse{IsLoggedIn: isLoggedIn}

		// 已登录时从当前页读取真实账号信息；读不到只记 warn，不影响状态返回。
		if isLoggedIn {
			if user, err := loginAction.CurrentUser(ctx); err != nil {
				logrus.Warnf("failed to get current user info: %v", err)
			} else {
				response.Username = user.Nickname
				response.UserID = user.UserID
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// GetLoginQrcode 获取登录的扫码二维码
func (s *XiaohongshuService) GetLoginQrcode(ctx context.Context) (*LoginQrcodeResponse, error) {
	page, release, err := sharedBrowser.Lease()
	if err != nil {
		return nil, err
	}

	// 已登录或取码失败：立刻释放；未登录：后台持有 lease 直到扫码结束
	loginAction := xiaohongshu.NewLogin(page)

	img, loggedIn, err := loginAction.FetchQrcodeImage(ctx)
	if err != nil || loggedIn {
		release()
	}
	if err != nil {
		return nil, err
	}

	timeout := 4 * time.Minute

	if !loggedIn {
		s.waitScanInBackground(loginAction, page, release, timeout)
	}

	return &LoginQrcodeResponse{
		Timeout: func() string {
			if loggedIn {
				return "0s"
			}
			return timeout.String()
		}(),
		Img:        img,
		IsLoggedIn: loggedIn,
	}, nil
}

// waitScanInBackground 在后台等用户扫码，扫上了就存 cookie。
// release 必须关闭 page 并释放 sharedBrowser 串行锁。
func (s *XiaohongshuService) waitScanInBackground(
	loginAction *xiaohongshu.LoginAction, page playwright.Page, release func(), timeout time.Duration,
) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	seq := s.logins.start(cancel)
	logrus.Infof("等待扫码登录，会话 #%d，超时 %s", seq, timeout)

	go func() {
		defer release()
		defer cancel()
		defer s.logins.finish(seq)

		if loginAction.WaitForLogin(ctxTimeout) {
			if err := saveCookies(); err != nil {
				logrus.Errorf("扫码成功但保存 cookies 失败，会话 #%d: %v", seq, err)
				return
			}
			logrus.Infof("扫码登录成功，cookies 已保存，会话 #%d", seq)
			return
		}

		// 没等到扫码：要么超时，要么被新取的二维码取代
		logrus.Infof("登录会话 #%d 结束，未检测到扫码（超时或已被新的二维码取代）", seq)
	}()
}

// ListFeeds 获取Feeds列表
func (s *XiaohongshuService) ListFeeds(ctx context.Context) (*FeedsListResponse, error) {
	var response *FeedsListResponse
	err := sharedBrowser.Do(func(page playwright.Page) error {
		action := xiaohongshu.NewFeedsListAction(page)

		feeds, err := action.GetFeedsList(ctx)
		if err != nil {
			logrus.Errorf("获取 Feeds 列表失败: %v", err)
			return err
		}

		response = &FeedsListResponse{
			Feeds: feeds,
			Count: len(feeds),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *XiaohongshuService) SearchFeeds(ctx context.Context, keyword string, filters ...xiaohongshu.FilterOption) (*FeedsListResponse, error) {
	var response *FeedsListResponse
	err := sharedBrowser.Do(func(page playwright.Page) error {
		action := xiaohongshu.NewSearchAction(page)

		feeds, err := action.Search(ctx, keyword, filters...)
		if err != nil {
			return err
		}

		response = &FeedsListResponse{
			Feeds: feeds,
			Count: len(feeds),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// GetFeedDetail 获取Feed详情
func (s *XiaohongshuService) GetFeedDetail(ctx context.Context, feedID, xsecToken string, loadAllComments bool) (*FeedDetailResponse, error) {
	return s.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, xiaohongshu.DefaultCommentLoadConfig())
}

// GetFeedDetailWithConfig 使用配置获取Feed详情
func (s *XiaohongshuService) GetFeedDetailWithConfig(ctx context.Context, feedID, xsecToken string, loadAllComments bool, config xiaohongshu.CommentLoadConfig) (*FeedDetailResponse, error) {
	var response *FeedDetailResponse
	err := sharedBrowser.Do(func(page playwright.Page) error {
		action := xiaohongshu.NewFeedDetailAction(page)

		result, err := action.GetFeedDetailWithConfig(ctx, feedID, xsecToken, loadAllComments, config)
		if err != nil {
			return err
		}

		response = &FeedDetailResponse{
			FeedID: feedID,
			Data:   result,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// UserProfile 获取用户信息
func (s *XiaohongshuService) UserProfile(ctx context.Context, userID, xsecToken, tab string) (*UserProfileResponse, error) {
	parsed, err := xiaohongshu.ParseProfileTab(tab)
	if err != nil {
		return nil, err
	}

	var response *UserProfileResponse
	err = sharedBrowser.Do(func(page playwright.Page) error {
		action := xiaohongshu.NewUserProfileAction(page)

		result, err := action.UserProfile(ctx, userID, xsecToken, parsed)
		if err != nil {
			return err
		}
		response = &UserProfileResponse{
			UserBasicInfo: result.UserBasicInfo,
			Interactions:  result.Interactions,
			Feeds:         result.Feeds,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// saveCookies 把当前共享浏览器上下文的 cookie 落盘（CDP 线格式，与历史 cookies.json 一致）。
func saveCookies() error {
	b := sharedBrowser.current()
	if b == nil {
		return fmt.Errorf("browser not started")
	}
	data, err := b.Cookies()
	if err != nil {
		return err
	}
	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}

// withBrowserPage 执行需要浏览器页面的操作的通用函数（走常驻串行 session）
func withBrowserPage(fn func(playwright.Page) error) error {
	return sharedBrowser.Do(fn)
}

// GetMyProfile 获取当前登录用户的个人信息
func (s *XiaohongshuService) GetMyProfile(ctx context.Context, tab string) (*UserProfileResponse, error) {
	parsed, err := xiaohongshu.ParseProfileTab(tab)
	if err != nil {
		return nil, err
	}

	var result *xiaohongshu.UserProfileResponse

	err = withBrowserPage(func(page playwright.Page) error {
		action := xiaohongshu.NewUserProfileAction(page)
		result, err = action.GetMyProfileViaSidebar(ctx, parsed)
		return err
	})

	if err != nil {
		return nil, err
	}

	response := &UserProfileResponse{
		UserBasicInfo: result.UserBasicInfo,
		Interactions:  result.Interactions,
		Feeds:         result.Feeds,
	}

	return response, nil
}
