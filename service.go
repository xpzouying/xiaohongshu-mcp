package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/downloader"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/xhsutil"
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

// PublishRequest 发布请求
type PublishRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	Images     []string `json:"images" binding:"required,min=1"`
	Tags       []string `json:"tags,omitempty"`
	ScheduleAt string   `json:"schedule_at,omitempty"` // 定时发布时间，ISO8601格式，为空则立即发布
	IsOriginal bool     `json:"is_original,omitempty"` // 是否声明原创
	Visibility string   `json:"visibility,omitempty"`  // 可见范围: "公开可见"(默认), "仅自己可见", "仅互关好友可见"
	Products   []string `json:"products,omitempty"`    // 商品关键词列表，用于绑定带货商品
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

// PublishResponse 发布响应
type PublishResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Images  int    `json:"images"`
	Status  string `json:"status"`
}

// PublishVideoRequest 发布视频请求（仅支持本地单个视频文件）
type PublishVideoRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	Video      string   `json:"video" binding:"required"`
	Tags       []string `json:"tags,omitempty"`
	ScheduleAt string   `json:"schedule_at,omitempty"` // 定时发布时间，ISO8601格式，为空则立即发布
	Visibility string   `json:"visibility,omitempty"`  // 可见范围: "公开可见"(默认), "仅自己可见", "仅互关好友可见"
	Products   []string `json:"products,omitempty"`    // 商品关键词列表，用于绑定带货商品
}

// PublishVideoResponse 发布视频响应
type PublishVideoResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Video   string `json:"video"`
	Status  string `json:"status"`
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

// DeleteCookies 删除 cookies 文件，用于登录重置
func (s *XiaohongshuService) DeleteCookies(ctx context.Context) error {
	if err := assertWritable("delete_cookies"); err != nil {
		return err
	}
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)
	if err := cookieLoader.DeleteCookies(); err != nil {
		return err
	}
	// 登录态文件变了，常驻浏览器必须重建
	sharedBrowser.Invalidate()
	return nil
}

// CheckLoginStatus 检查登录状态
func (s *XiaohongshuService) CheckLoginStatus(ctx context.Context) (*LoginStatusResponse, error) {
	var response *LoginStatusResponse
	err := sharedBrowser.Do(func(page *rod.Page) error {
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
	loginAction *xiaohongshu.LoginAction, page *rod.Page, release func(), timeout time.Duration,
) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	seq := s.logins.start(cancel)
	logrus.Infof("等待扫码登录，会话 #%d，超时 %s", seq, timeout)

	go func() {
		defer release()
		defer cancel()
		defer s.logins.finish(seq)

		if loginAction.WaitForLogin(ctxTimeout) {
			if err := saveCookies(page); err != nil {
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

// PublishContent 发布内容
func (s *XiaohongshuService) PublishContent(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	if err := assertWritable("publish_content"); err != nil {
		return nil, err
	}
	// 验证标题长度（小红书限制：最大20个字）
	if xhsutil.CalcTitleLength(req.Title) > 20 {
		return nil, fmt.Errorf("标题长度超过限制")
	}

	imagePaths, err := s.processImages(req.Images)
	if err != nil {
		return nil, err
	}

	var scheduleTime *time.Time
	if req.ScheduleAt != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduleAt)
		if err != nil {
			return nil, fmt.Errorf("定时发布时间格式错误，请使用 ISO8601 格式: %v", err)
		}

		// 校验定时发布时间范围：1小时至14天
		now := time.Now()
		minTime := now.Add(1 * time.Hour)
		maxTime := now.Add(14 * 24 * time.Hour)

		if t.Before(minTime) {
			return nil, fmt.Errorf("定时发布时间必须至少在1小时后，当前设置: %s，最早可选: %s",
				t.Format("2006-01-02 15:04"), minTime.Format("2006-01-02 15:04"))
		}
		if t.After(maxTime) {
			return nil, fmt.Errorf("定时发布时间不能超过14天，当前设置: %s，最晚可选: %s",
				t.Format("2006-01-02 15:04"), maxTime.Format("2006-01-02 15:04"))
		}

		scheduleTime = &t
		logrus.Infof("设置定时发布时间: %s", t.Format("2006-01-02 15:04"))
	}

	content := xiaohongshu.PublishImageContent{
		Title:        req.Title,
		Content:      req.Content,
		Tags:         req.Tags,
		ImagePaths:   imagePaths,
		ScheduleTime: scheduleTime,
		IsOriginal:   req.IsOriginal,
		Visibility:   req.Visibility,
		Products:     req.Products,
	}

	if err := s.publishContent(ctx, content); err != nil {
		logrus.Errorf("发布内容失败: title=%s %v", content.Title, err)
		return nil, err
	}

	response := &PublishResponse{
		Title:   req.Title,
		Content: req.Content,
		Images:  len(imagePaths),
		Status:  "发布完成",
	}

	return response, nil
}

// processImages 处理图片列表，支持URL下载和本地路径
func (s *XiaohongshuService) processImages(images []string) ([]string, error) {
	processor := downloader.NewImageProcessor()
	return processor.ProcessImages(images)
}

// publishContent 执行内容发布
func (s *XiaohongshuService) publishContent(ctx context.Context, content xiaohongshu.PublishImageContent) error {
	return sharedBrowser.Do(func(page *rod.Page) error {
		action, err := xiaohongshu.NewPublishImageAction(page)
		if err != nil {
			return err
		}
		return action.Publish(ctx, content)
	})
}

// PublishVideo 发布视频（本地文件）
func (s *XiaohongshuService) PublishVideo(ctx context.Context, req *PublishVideoRequest) (*PublishVideoResponse, error) {
	if err := assertWritable("publish_video"); err != nil {
		return nil, err
	}
	// 标题长度校验（小红书限制：最大20个字）
	if xhsutil.CalcTitleLength(req.Title) > 20 {
		return nil, fmt.Errorf("标题长度超过限制")
	}

	// 本地视频文件校验
	if req.Video == "" {
		return nil, fmt.Errorf("必须提供本地视频文件")
	}
	if _, err := os.Stat(req.Video); err != nil {
		return nil, fmt.Errorf("视频文件不存在或不可访问: %v", err)
	}

	var scheduleTime *time.Time
	if req.ScheduleAt != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduleAt)
		if err != nil {
			return nil, fmt.Errorf("定时发布时间格式错误，请使用 ISO8601 格式: %v", err)
		}

		// 校验定时发布时间范围：1小时至14天
		now := time.Now()
		minTime := now.Add(1 * time.Hour)
		maxTime := now.Add(14 * 24 * time.Hour)

		if t.Before(minTime) {
			return nil, fmt.Errorf("定时发布时间必须至少在1小时后，当前设置: %s，最早可选: %s",
				t.Format("2006-01-02 15:04"), minTime.Format("2006-01-02 15:04"))
		}
		if t.After(maxTime) {
			return nil, fmt.Errorf("定时发布时间不能超过14天，当前设置: %s，最晚可选: %s",
				t.Format("2006-01-02 15:04"), maxTime.Format("2006-01-02 15:04"))
		}

		scheduleTime = &t
		logrus.Infof("设置定时发布时间: %s", t.Format("2006-01-02 15:04"))
	}

	content := xiaohongshu.PublishVideoContent{
		Title:        req.Title,
		Content:      req.Content,
		Tags:         req.Tags,
		VideoPath:    req.Video,
		ScheduleTime: scheduleTime,
		Visibility:   req.Visibility,
		Products:     req.Products,
	}

	if err := s.publishVideo(ctx, content); err != nil {
		return nil, err
	}

	resp := &PublishVideoResponse{
		Title:   req.Title,
		Content: req.Content,
		Video:   req.Video,
		Status:  "发布完成",
	}
	return resp, nil
}

// publishVideo 执行视频发布
func (s *XiaohongshuService) publishVideo(ctx context.Context, content xiaohongshu.PublishVideoContent) error {
	return sharedBrowser.Do(func(page *rod.Page) error {
		action, err := xiaohongshu.NewPublishVideoAction(page)
		if err != nil {
			return err
		}
		return action.PublishVideo(ctx, content)
	})
}

// ListFeeds 获取Feeds列表
func (s *XiaohongshuService) ListFeeds(ctx context.Context) (*FeedsListResponse, error) {
	var response *FeedsListResponse
	err := sharedBrowser.Do(func(page *rod.Page) error {
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
	err := sharedBrowser.Do(func(page *rod.Page) error {
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
	err := sharedBrowser.Do(func(page *rod.Page) error {
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
	err = sharedBrowser.Do(func(page *rod.Page) error {
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

// PostCommentToFeed 发表评论到Feed
func (s *XiaohongshuService) PostCommentToFeed(ctx context.Context, feedID, xsecToken, content string) (*PostCommentResponse, error) {
	if err := assertWritable("post_comment"); err != nil {
		return nil, err
	}
	err := sharedBrowser.Do(func(page *rod.Page) error {
		action := xiaohongshu.NewCommentFeedAction(page)
		return action.PostComment(ctx, feedID, xsecToken, content)
	})
	if err != nil {
		return nil, err
	}
	return &PostCommentResponse{FeedID: feedID, Success: true, Message: "评论发表成功"}, nil
}

// LikeFeed 点赞笔记
func (s *XiaohongshuService) LikeFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	if err := assertWritable("like"); err != nil {
		return nil, err
	}
	err := sharedBrowser.Do(func(page *rod.Page) error {
		action := xiaohongshu.NewLikeAction(page)
		return action.Like(ctx, feedID, xsecToken)
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "点赞成功或已点赞"}, nil
}

// UnlikeFeed 取消点赞笔记
func (s *XiaohongshuService) UnlikeFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	if err := assertWritable("unlike"); err != nil {
		return nil, err
	}
	err := sharedBrowser.Do(func(page *rod.Page) error {
		action := xiaohongshu.NewLikeAction(page)
		return action.Unlike(ctx, feedID, xsecToken)
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "取消点赞成功或未点赞"}, nil
}

// FavoriteFeed 收藏笔记
func (s *XiaohongshuService) FavoriteFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	if err := assertWritable("favorite"); err != nil {
		return nil, err
	}
	err := sharedBrowser.Do(func(page *rod.Page) error {
		action := xiaohongshu.NewFavoriteAction(page)
		return action.Favorite(ctx, feedID, xsecToken)
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "收藏成功或已收藏"}, nil
}

// UnfavoriteFeed 取消收藏笔记
func (s *XiaohongshuService) UnfavoriteFeed(ctx context.Context, feedID, xsecToken string) (*ActionResult, error) {
	if err := assertWritable("unfavorite"); err != nil {
		return nil, err
	}
	err := sharedBrowser.Do(func(page *rod.Page) error {
		action := xiaohongshu.NewFavoriteAction(page)
		return action.Unfavorite(ctx, feedID, xsecToken)
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{FeedID: feedID, Success: true, Message: "取消收藏成功或未收藏"}, nil
}

// ReplyCommentToFeed 回复指定评论
func (s *XiaohongshuService) ReplyCommentToFeed(ctx context.Context, feedID, xsecToken, commentID, userID, content string) (*ReplyCommentResponse, error) {
	if err := assertWritable("reply_comment"); err != nil {
		return nil, err
	}
	err := sharedBrowser.Do(func(page *rod.Page) error {
		action := xiaohongshu.NewCommentFeedAction(page)
		return action.ReplyToComment(ctx, feedID, xsecToken, commentID, userID, content)
	})
	if err != nil {
		return nil, err
	}

	return &ReplyCommentResponse{
		FeedID:          feedID,
		TargetCommentID: commentID,
		TargetUserID:    userID,
		Success:         true,
		Message:         "评论回复成功",
	}, nil
}

// GetUnreadCount 获取通知未读数
func (s *XiaohongshuService) GetUnreadCount(ctx context.Context) (*xiaohongshu.NotificationCount, error) {
	var result *xiaohongshu.NotificationCount
	err := sharedBrowser.Do(func(page *rod.Page) error {
		var err error
		result, err = xiaohongshu.NewNotificationAction(page).UnreadCount(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListNotifications 获取指定分区的通知列表
func (s *XiaohongshuService) ListNotifications(ctx context.Context, tab string, limit int) (*xiaohongshu.NotificationList, error) {
	parsed, err := xiaohongshu.ParseNotificationTab(tab)
	if err != nil {
		return nil, err
	}

	var result *xiaohongshu.NotificationList
	err = sharedBrowser.Do(func(page *rod.Page) error {
		var err error
		result, err = xiaohongshu.NewNotificationAction(page).List(ctx, parsed, limit)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// LikeNotification 给通知里的评论点赞或取消点赞
func (s *XiaohongshuService) LikeNotification(ctx context.Context, commentID string, unlike bool) (*xiaohongshu.NotificationLikeResult, error) {
	if err := assertWritable("like_notification"); err != nil {
		return nil, err
	}
	var result *xiaohongshu.NotificationLikeResult
	err := sharedBrowser.Do(func(page *rod.Page) error {
		var err error
		result, err = xiaohongshu.NewNotificationAction(page).Like(ctx, commentID, unlike)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReplyNotification 在通知页就地回复评论
func (s *XiaohongshuService) ReplyNotification(ctx context.Context, commentID, content string) (*xiaohongshu.NotificationReplyResult, error) {
	if err := assertWritable("reply_notification"); err != nil {
		return nil, err
	}
	var result *xiaohongshu.NotificationReplyResult
	err := sharedBrowser.Do(func(page *rod.Page) error {
		var err error
		result, err = xiaohongshu.NewNotificationAction(page).Reply(ctx, commentID, content)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}

// withBrowserPage 执行需要浏览器页面的操作的通用函数（走常驻串行 session）
func withBrowserPage(fn func(*rod.Page) error) error {
	return sharedBrowser.Do(fn)
}

// GetMyProfile 获取当前登录用户的个人信息
func (s *XiaohongshuService) GetMyProfile(ctx context.Context, tab string) (*UserProfileResponse, error) {
	parsed, err := xiaohongshu.ParseProfileTab(tab)
	if err != nil {
		return nil, err
	}

	var result *xiaohongshu.UserProfileResponse

	err = withBrowserPage(func(page *rod.Page) error {
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
