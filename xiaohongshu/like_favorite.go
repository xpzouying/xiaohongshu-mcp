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
	actionLike     interactActionType = "点赞"
	actionFavorite interactActionType = "收藏"
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
		logrus.Infof("feed %s already %sd, skip clicking", feedID, actionType)
		return nil
	}

	// 点击按钮
	elem := page.MustElement(selector)
	elem.MustClick()

	time.Sleep(3 * time.Second) // 增加等待时间，确保状态更新

	// 验证是否成功
	newLiked, newCollected, err := a.getInteractState(page, feedID)
	if err == nil && getStateFunc(newLiked, newCollected) {
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
			if err2 == nil && getStateFunc(finalLiked, finalCollected) {
				logrus.Infof("feed %s 第二次点击%s成功", feedID, actionType)
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