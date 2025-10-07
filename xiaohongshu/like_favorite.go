package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

// LikeFavoriteAction 点赞/收藏 动作
// 提供在笔记详情页执行点赞和收藏的能力，并在可能的情况下避免重复点击
// 通过读取 window.__INITIAL_STATE__ 判断当前状态
// 注意：该实现依赖页面 DOM，可能随页面升级而变化

type LikeFavoriteAction struct {
	page *rod.Page
}

func NewLikeFavoriteAction(page *rod.Page) *LikeFavoriteAction {
	return &LikeFavoriteAction{page: page}
}

// interactActionType 交互动作类型
type interactActionType string

const (
	actionLike         interactActionType = "点赞"
	actionFavorite     interactActionType = "收藏"
	actionUnlike       interactActionType = "取消点赞"
	actionUnfavorite   interactActionType = "取消收藏"
)

// getValidationFunc 根据动作类型获取验证函数
func getValidationFunc(actionType interactActionType) func(bool, bool) bool {
	switch actionType {
	case actionLike:
		return func(l, c bool) bool { return l }
	case actionFavorite:
		return func(l, c bool) bool { return c }
	case actionUnlike:
		return func(l, c bool) bool { return !l }
	case actionUnfavorite:
		return func(l, c bool) bool { return !c }
	default:
		return func(l, c bool) bool { return false }
	}
}

// skipMessage 根据动作类型生成跳过消息
func skipMessage(actionType interactActionType, feedID string) string {
	if actionType == actionLike || actionType == actionFavorite {
		return fmt.Sprintf("feed %s already %sd, skip clicking", feedID, actionType)
	}
	return fmt.Sprintf("feed %s not %sd yet, skip clicking", feedID, string(actionType)[2:])
}

// performClick 执行点击操作
func (a *LikeFavoriteAction) performClick(page *rod.Page, selector string) {
	elem := page.MustElement(selector)
	elem.MustClick()
}

// interactConfig 交互动作配置
type interactConfig struct {
	feedID      string
	xsecToken   string
	actionType  interactActionType
	selector    string
	shouldSkip  func(bool, bool) bool
}

// performInteractAction 执行交互动作的通用方法
func (a *LikeFavoriteAction) performInteractAction(ctx context.Context, config interactConfig) error {
	page := a.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(config.feedID, config.xsecToken)
	logrus.Infof("Opening feed detail page for %s: %s", config.actionType, url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	// 检查当前状态
	liked, collected, err := a.getInteractState(page, config.feedID)
	if err != nil {
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
	} else if config.shouldSkip(liked, collected) {
		logrus.Infof(skipMessage(config.actionType, config.feedID))
		return nil
	}

	// 执行点击
	a.performClick(page, config.selector)
	time.Sleep(3 * time.Second)

	// 验证结果
	isSuccess := getValidationFunc(config.actionType)
	newLiked, newCollected, err := a.getInteractState(page, config.feedID)
	if err == nil && isSuccess(newLiked, newCollected) {
		logrus.Infof("feed %s %s成功", config.feedID, config.actionType)
		return nil
	}

	// 失败重试
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", config.actionType, err)
	} else {
		logrus.Warnf("feed %s %s可能未成功，状态未变化，尝试再次点击", config.feedID, config.actionType)
		a.performClick(page, config.selector)
		time.Sleep(2 * time.Second)
		
		finalLiked, finalCollected, err2 := a.getInteractState(page, config.feedID)
		if err2 == nil && isSuccess(finalLiked, finalCollected) {
			logrus.Infof("feed %s 第二次点击%s成功", config.feedID, config.actionType)
			return nil
		} 
	}

	return nil
}

// Like 点赞指定笔记，如果已点赞则直接返回
func (a *LikeFavoriteAction) Like(ctx context.Context, feedID, xsecToken string) error {
	config := interactConfig{
		feedID:     feedID,
		xsecToken:  xsecToken,
		actionType: actionLike,
		selector:   SelectorLikeButton,
		shouldSkip: func(liked, collected bool) bool { return liked },
	}
	return a.performInteractAction(ctx, config)
}

// Favorite 收藏指定笔记，如果已收藏则直接返回
func (a *LikeFavoriteAction) Favorite(ctx context.Context, feedID, xsecToken string) error {
	config := interactConfig{
		feedID:     feedID,
		xsecToken:  xsecToken,
		actionType: actionFavorite,
		selector:   SelectorCollectButton,
		shouldSkip: func(liked, collected bool) bool { return collected },
	}
	return a.performInteractAction(ctx, config)
}

// Unlike 取消点赞指定笔记，如果未点赞则直接返回
func (a *LikeFavoriteAction) Unlike(ctx context.Context, feedID, xsecToken string) error {
	config := interactConfig{
		feedID:     feedID,
		xsecToken:  xsecToken,
		actionType: actionUnlike,
		selector:   SelectorLikeButton,
		shouldSkip: func(liked, collected bool) bool { return !liked },
	}
	return a.performInteractAction(ctx, config)
}

// Unfavorite 取消收藏指定笔记，如果未收藏则直接返回
func (a *LikeFavoriteAction) Unfavorite(ctx context.Context, feedID, xsecToken string) error {
	config := interactConfig{
		feedID:     feedID,
		xsecToken:  xsecToken,
		actionType: actionUnfavorite,
		selector:   SelectorCollectButton,
		shouldSkip: func(liked, collected bool) bool { return !collected },
	}
	return a.performInteractAction(ctx, config)
}

// getInteractState 从 __INITIAL_STATE__ 读取笔记的点赞/收藏状态
func (a *LikeFavoriteAction) getInteractState(page *rod.Page, feedID string) (liked bool, collected bool, err error) {
	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__) {
			return JSON.stringify(window.__INITIAL_STATE__);
		}
		return "";
	}`).String()
	if result == "" {
		return false, false, fmt.Errorf("__INITIAL_STATE__ not found")
	}

	var state struct {
		Note struct {
			NoteDetailMap map[string]struct {
				Note struct {
					InteractInfo struct {
						Liked     bool `json:"liked"`
						Collected bool `json:"collected"`
					} `json:"interactInfo"`
				} `json:"note"`
			} `json:"noteDetailMap"`
		} `json:"note"`
	}
	if err := json.Unmarshal([]byte(result), &state); err != nil {
		return false, false, errors.Wrap(err, "unmarshal __INITIAL_STATE__ failed")
	}

	detail, ok := state.Note.NoteDetailMap[feedID]
	if !ok {
		return false, false, fmt.Errorf("feed %s not in noteDetailMap", feedID)
	}
	return detail.Note.InteractInfo.Liked, detail.Note.InteractInfo.Collected, nil
}

