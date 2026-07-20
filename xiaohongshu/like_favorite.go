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
	"github.com/xpzouying/xiaohongshu-mcp/humanize"
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
	humanize.Delay(ctx, humanize.AfterNavigate)

	return page
}

func (a *interactAction) performClick(page *rod.Page, selector string) error {
	element, err := page.Element(selector)
	if err != nil {
		return fmt.Errorf("未找到交互元素 %s: %w", selector, err)
	}
	return humanize.Click(element)
}

// stateOf 从交互状态里取目标字段（点赞取 liked，收藏取 collected）。
type stateOf func(liked, collected bool) bool

// toggleInteract 点击交互按钮并轮询校验状态是否变为 want；最多两次点击。
// 到达即成功；始终未变或无法读状态则返回 error——消除"点了没报错就算成功"的假阳性。
func (a *interactAction) toggleInteract(page *rod.Page, feedID, selector string, want bool, actionType interactActionType, pick stateOf) error {
	for attempt := 1; attempt <= 2; attempt++ {
		if err := a.performClick(page, selector); err != nil {
			return fmt.Errorf("%s点击失败: %w", actionType, err)
		}

		ok, err := a.waitInteractState(page, feedID, want, pick, 4*time.Second)
		if err != nil {
			return fmt.Errorf("%s后无法确认状态: %w", actionType, err)
		}
		if ok {
			logrus.Infof("feed %s %s成功（第%d次点击）", feedID, actionType, attempt)
			return nil
		}
		logrus.Warnf("feed %s %s第%d次点击后状态未变，重试", feedID, actionType, attempt)
	}
	return fmt.Errorf("feed %s %s失败：点击后状态始终未变为预期", feedID, actionType)
}

// waitInteractState 轮询 __INITIAL_STATE__ 的交互状态，直到 pick()==want 或超时。
// 状态回写快则立即返回、慢则等满 timeout。
func (a *interactAction) waitInteractState(page *rod.Page, feedID string, want bool, pick stateOf, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		liked, collected, err := a.getInteractState(page, feedID)
		if err != nil {
			return false, err
		}
		if pick(liked, collected) == want {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
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
		return a.toggleInteract(page, feedID, SelectorLikeButton, targetLiked, actionType,
			func(liked, collected bool) bool { return liked })
	}

	if targetLiked && liked {
		logrus.Infof("feed %s already liked, skip clicking", feedID)
		return nil
	}
	if !targetLiked && !liked {
		logrus.Infof("feed %s not liked yet, skip clicking", feedID)
		return nil
	}

	return a.toggleInteract(page, feedID, SelectorLikeButton, targetLiked, actionType,
		func(liked, collected bool) bool { return liked })
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
		return a.toggleInteract(page, feedID, SelectorCollectButton, targetCollected, actionType,
			func(liked, collected bool) bool { return collected })
	}

	if targetCollected && collected {
		logrus.Infof("feed %s already favorited, skip clicking", feedID)
		return nil
	}
	if !targetCollected && !collected {
		logrus.Infof("feed %s not favorited yet, skip clicking", feedID)
		return nil
	}

	return a.toggleInteract(page, feedID, SelectorCollectButton, targetCollected, actionType,
		func(liked, collected bool) bool { return collected })
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
