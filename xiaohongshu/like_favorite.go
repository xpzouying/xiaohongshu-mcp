package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
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
	logrus.Infof("Opening feed detail page for %s: %s", actionType, url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	return page
}

func (a *interactAction) performClick(page *rod.Page, selector string) {
	element := page.MustElement(selector)
	element.MustClick()
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
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
		return a.toggleLike(page, feedID, targetLiked, actionType)
	}

	if targetLiked && liked {
		logrus.Infof("feed %s already liked, skip clicking", feedID)
		return nil
	}
	if !targetLiked && !liked {
		logrus.Infof("feed %s not liked yet, skip clicking", feedID)
		return nil
	}

	return a.toggleLike(page, feedID, targetLiked, actionType)
}

func (a *LikeAction) toggleLike(page *rod.Page, feedID string, targetLiked bool, actionType interactActionType) error {
	a.performClick(page, SelectorLikeButton)
	time.Sleep(3 * time.Second)

	liked, _, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
		return nil
	}
	if liked == targetLiked {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	logrus.Warnf("feed %s %s可能未成功，状态未变化，尝试再次点击", feedID, actionType)
	a.performClick(page, SelectorLikeButton)
	time.Sleep(2 * time.Second)

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
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
		return a.toggleFavorite(page, feedID, targetCollected, actionType)
	}

	if targetCollected && collected {
		logrus.Infof("feed %s already favorited, skip clicking", feedID)
		return nil
	}
	if !targetCollected && !collected {
		logrus.Infof("feed %s not favorited yet, skip clicking", feedID)
		return nil
	}

	return a.toggleFavorite(page, feedID, targetCollected, actionType)
}

func (a *FavoriteAction) toggleFavorite(page *rod.Page, feedID string, targetCollected bool, actionType interactActionType) error {
	a.performClick(page, SelectorCollectButton)
	time.Sleep(3 * time.Second)

	_, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
		return nil
	}
	if collected == targetCollected {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	logrus.Warnf("feed %s %s可能未成功，状态未变化，尝试再次点击", feedID, actionType)
	a.performClick(page, SelectorCollectButton)
	time.Sleep(2 * time.Second)

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

	// 直接解析为 noteDetailMap
	var noteDetailMap map[string]struct {
		Note struct {
			InteractInfo struct {
				Liked     bool `json:"liked"`
				Collected bool `json:"collected"`
			} `json:"interactInfo"`
		} `json:"note"`
	}
	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return false, false, errors.Wrap(err, "unmarshal noteDetailMap failed")
	}

	detail, ok := noteDetailMap[feedID]
	if !ok {
		return false, false, fmt.Errorf("feed %s not in noteDetailMap", feedID)
	}
	return detail.Note.InteractInfo.Liked, detail.Note.InteractInfo.Collected, nil
}
