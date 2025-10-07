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

// performInteractAction 执行交互动作的通用方法
func (a *LikeFavoriteAction) performInteractAction(ctx context.Context, feedID, xsecToken string, actionType interactActionType, selector string, getStateFunc func(bool, bool) bool) error {
	page := a.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("Opening feed detail page for %s: %s", actionType, url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	liked, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
	} else if getStateFunc(liked, collected) {
		// 根据动作类型显示不同的跳过信息
		if actionType == actionLike || actionType == actionFavorite {
			logrus.Infof("feed %s already %sd, skip clicking", feedID, actionType)
		} else {
			logrus.Infof("feed %s not %sd yet, skip clicking", feedID, string(actionType)[2:]) // 去掉"取消"前缀
		}
		return nil
	}

	// 点击按钮
	elem := page.MustElement(selector)
	elem.MustClick()

	time.Sleep(3 * time.Second) // 增加等待时间，确保状态更新

	// 定义验证函数
	var isSuccess func(bool, bool) bool
	switch actionType {
	case actionLike:
		isSuccess = func(l, c bool) bool { return l }
	case actionFavorite:
		isSuccess = func(l, c bool) bool { return c }
	case actionUnlike:
		isSuccess = func(l, c bool) bool { return !l }
	case actionUnfavorite:
		isSuccess = func(l, c bool) bool { return !c }
	}

	// 验证是否成功
	newLiked, newCollected, err := a.getInteractState(page, feedID)
	if err == nil && isSuccess(newLiked, newCollected) {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
	} else {
		logrus.Warnf("feed %s %s可能未成功，状态未变化，尝试再次点击", feedID, actionType)
		// 如果第一次点击失败，尝试再次点击
		elem := page.MustElement(selector)
		elem.MustClick()
		{
			time.Sleep(2 * time.Second)
			finalLiked, finalCollected, err2 := a.getInteractState(page, feedID)
			if err2 == nil && isSuccess(finalLiked, finalCollected) {
				logrus.Infof("feed %s 第二次点击%s成功", feedID, actionType)
				return nil
			} else if err2 == nil && actionType == actionLike && !finalLiked {
				logrus.Warnf("feed %s 第二次点击后取消了点赞，这是正常行为", feedID)
				return nil
			} 
		}
	}

	return nil
}

// Like 点赞指定笔记，如果已点赞则直接返回
func (a *LikeFavoriteAction) Like(ctx context.Context, feedID, xsecToken string) error {
	return a.performInteractAction(ctx, feedID, xsecToken, actionLike, ".left > :first-child", func(liked, collected bool) bool {
		return liked
	})
}

// Favorite 收藏指定笔记，如果已收藏则直接返回
func (a *LikeFavoriteAction) Favorite(ctx context.Context, feedID, xsecToken string) error {
	return a.performInteractAction(ctx, feedID, xsecToken, actionFavorite, ".collect-wrapper > :last-child", func(liked, collected bool) bool {
		return collected
	})
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

// Unlike 取消点赞指定笔记，如果未点赞则直接返回
func (a *LikeFavoriteAction) Unlike(ctx context.Context, feedID, xsecToken string) error {
	return a.performInteractAction(ctx, feedID, xsecToken, actionUnlike, ".like-wrapper > :last-child", func(liked, collected bool) bool {
		return !liked // 如果未点赞，则跳过
	})
}

// Unfavorite 取消收藏指定笔记，如果未收藏则直接返回
func (a *LikeFavoriteAction) Unfavorite(ctx context.Context, feedID, xsecToken string) error {
	return a.performInteractAction(ctx, feedID, xsecToken, actionUnfavorite, ".collect-wrapper > :last-child", func(liked, collected bool) bool {
		return !collected // 如果未收藏，则跳过
	})
}