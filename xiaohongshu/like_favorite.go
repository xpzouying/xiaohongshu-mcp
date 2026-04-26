package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	myerrors "github.com/xpzouying/xiaohongshu-mcp/errors"
)

// ActionResult 通用动作响应（点赞/收藏等）
type ActionResult struct {
	FeedID  string `json:"feed_id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// 选择器常量
const (
	SelectorLikeButton    = ".interact-container .left .like-lottie"
	SelectorCollectButton = ".interact-container .left .reds-icon.collect-icon"
)

// interactActionType 交互动作类型
type interactActionType string

const (
	actionLike       interactActionType = "点赞"
	actionFavorite   interactActionType = "收藏"
	actionUnlike     interactActionType = "取消点赞"
	actionUnfavorite interactActionType = "取消收藏"
)

type interactAction struct {
	page *rod.Page
}

func newInteractAction(page *rod.Page) *interactAction {
	return &interactAction{page: page}
}

func (a *interactAction) preparePage(ctx context.Context, actionType interactActionType, feedID, xsecToken string) *rod.Page {
	page := a.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开 feed 详情页进行 %s: %s", actionType, url)

	// 拟人导航：先访问首页，再跳转到目标页
	_ = NavigateWithLanding(page, url, false)
	return page
}

func (a *interactAction) performClick(page *rod.Page, selector string) {
	element := page.MustElement(selector)
	// 使用拟人点击替代直接点击
	if err := ClickWithHumanBehavior(page, element); err != nil {
		logrus.Warnf("拟人点击失败，回退到直接点击: %v", err)
		element.MustClick()
	}
	// 点击后等待状态更新
	RandomDelay(2000, 3500)
}

// LikeAction 负责处理点赞相关交互
type LikeAction struct {
	*interactAction
}

func NewLikeAction(page *rod.Page) *LikeAction {
	return &LikeAction{interactAction: newInteractAction(page)}
}

// Like 点赞指定笔记，如果已点赞则直接返回
func (a *LikeAction) Like(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, true)
}

// Unlike 取消点赞指定笔记，如果未点赞则直接返回
func (a *LikeAction) Unlike(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, false)
}

func (a *LikeAction) perform(ctx context.Context, feedID, xsecToken string, targetLiked bool) error {
	actionType := actionLike
	if !targetLiked {
		actionType = actionUnlike
	}

	page := a.preparePage(ctx, actionType, feedID, xsecToken)

	liked, _, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("读取交互状态失败: %v（继续尝试点击）", err)
		return a.toggleLike(page, feedID, targetLiked, actionType)
	}

	if targetLiked && liked {
		logrus.Infof("feed %s 已点赞，跳过", feedID)
		return nil
	}
	if !targetLiked && !liked {
		logrus.Infof("feed %s 未点赞，跳过", feedID)
		return nil
	}

	return a.toggleLike(page, feedID, targetLiked, actionType)
}

func (a *LikeAction) toggleLike(page *rod.Page, feedID string, targetLiked bool, actionType interactActionType) error {
	a.performClick(page, SelectorLikeButton)

	liked, _, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
		return nil
	}
	if liked == targetLiked {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	// 等待后重试一次
	logrus.Warnf("feed %s %s可能未成功，状态未变化，稍后重试", feedID, actionType)
	RandomDelay(1500, 2500)
	a.performClick(page, SelectorLikeButton)

	liked, _, err = a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("第二次验证%s状态失败: %v", actionType, err)
		return nil
	}
	if liked == targetLiked {
		logrus.Infof("feed %s 第二次点击%s成功", feedID, actionType)
		return nil
	}

	return nil
}

// FavoriteAction 负责处理收藏相关交互
type FavoriteAction struct {
	*interactAction
}

func NewFavoriteAction(page *rod.Page) *FavoriteAction {
	return &FavoriteAction{interactAction: newInteractAction(page)}
}

// Favorite 收藏指定笔记，如果已收藏则直接返回
func (a *FavoriteAction) Favorite(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, true)
}

// Unfavorite 取消收藏指定笔记，如果未收藏则直接返回
func (a *FavoriteAction) Unfavorite(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, false)
}

func (a *FavoriteAction) perform(ctx context.Context, feedID, xsecToken string, targetCollected bool) error {
	actionType := actionFavorite
	if !targetCollected {
		actionType = actionUnfavorite
	}

	page := a.preparePage(ctx, actionType, feedID, xsecToken)

	_, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("读取交互状态失败: %v（继续尝试点击）", err)
		return a.toggleFavorite(page, feedID, targetCollected, actionType)
	}

	if targetCollected && collected {
		logrus.Infof("feed %s 已收藏，跳过", feedID)
		return nil
	}
	if !targetCollected && !collected {
		logrus.Infof("feed %s 未收藏，跳过", feedID)
		return nil
	}

	return a.toggleFavorite(page, feedID, targetCollected, actionType)
}

func (a *FavoriteAction) toggleFavorite(page *rod.Page, feedID string, targetCollected bool, actionType interactActionType) error {
	a.performClick(page, SelectorCollectButton)

	_, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
		return nil
	}
	if collected == targetCollected {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	// 等待后重试一次
	logrus.Warnf("feed %s %s可能未成功，状态未变化，稍后重试", feedID, actionType)
	RandomDelay(1500, 2500)
	a.performClick(page, SelectorCollectButton)

	_, collected, err = a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("第二次验证%s状态失败: %v", actionType, err)
		return nil
	}
	if collected == targetCollected {
		logrus.Infof("feed %s 第二次点击%s成功", feedID, actionType)
		return nil
	}

	return nil
}

// getInteractState 从 __INITIAL_STATE__ 读取笔记的点赞/收藏状态
func (a *interactAction) getInteractState(page *rod.Page, feedID string) (liked bool, collected bool, err error) {
	// 等待一会再读取状态，确保页面 JS 已执行完毕
	RandomDelay(500, 1200)

	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.note &&
		    window.__INITIAL_STATE__.note.noteDetailMap) {
			return JSON.stringify(window.__INITIAL_STATE__.note.noteDetailMap);
		}
		return "";
	}`).String()
	if result == "" {
		return false, false, myerrors.ErrNoFeedDetail
	}

	var noteDetailMap map[string]struct {
		Note struct {
			InteractInfo struct {
				Liked     bool `json:"liked"`
				Collected bool `json:"collected"`
			} `json:"interactInfo"`
		} `json:"note"`
	}
	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return false, false, fmt.Errorf("unmarshal noteDetailMap 失败: %w", err)
	}

	detail, ok := noteDetailMap[feedID]
	if !ok {
		return false, false, fmt.Errorf("feed %s 不在 noteDetailMap 中", feedID)
	}
	return detail.Note.InteractInfo.Liked, detail.Note.InteractInfo.Collected, nil
}
