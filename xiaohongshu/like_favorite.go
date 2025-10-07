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

// Like 点赞指定笔记，如果已点赞则直接返回
func (a *LikeFavoriteAction) Like(ctx context.Context, feedID, xsecToken string) error {
	page := a.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("Opening feed detail page for like: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	liked, _, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
	} else if liked {
		logrus.Infof("feed %s already liked, skip clicking", feedID)
		return nil
	}


	// 尝试点击点赞按钮
	if err := clickLastMatch(page, []string{".like-lottie"}); err != nil {
		return errors.Wrap(err, "点击点赞按钮失败")
	}

	time.Sleep(3 * time.Second) // 增加等待时间，确保状态更新
	
	// 验证点赞是否成功
	newLiked, _, err := a.getInteractState(page, feedID)
	if err == nil && newLiked {
		logrus.Infof("feed %s 点赞成功", feedID)
		return nil
	}
	
	if err != nil {
		logrus.Warnf("验证点赞状态失败: %v", err)
	} else {
		logrus.Warnf("feed %s 点赞可能未成功，状态未变化，尝试再次点击", feedID)
		// 如果第一次点击失败，尝试再次点击
		if err := clickLastMatch(page, []string{".like-lottie"}); err != nil {
			logrus.Warnf("第二次点击点赞按钮也失败: %v", err)
		} else {
			time.Sleep(2 * time.Second)
			newLiked2, _, err2 := a.getInteractState(page, feedID)
			if err2 == nil && newLiked2 {
				logrus.Infof("feed %s 第二次点击点赞成功", feedID)
				return nil
			} else if err2 == nil && !newLiked2 {
				logrus.Warnf("feed %s 第二次点击后取消了点赞，这是正常行为", feedID)
				return nil
			}
		}
	}
	
	return nil
}

// Favorite 收藏指定笔记，如果已收藏则直接返回
func (a *LikeFavoriteAction) Favorite(ctx context.Context, feedID, xsecToken string) error {
	page := a.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("Opening feed detail page for favorite: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	_, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
	} else if collected {
		logrus.Infof("feed %s already favorited, skip clicking", feedID)
		return nil
	}

	if err := clickLastMatch(page, []string{"#collect"}); err != nil {
		return errors.Wrap(err, "点击收藏按钮失败")
	}

	time.Sleep(3 * time.Second) // 增加等待时间，确保状态更新
	
	// 验证收藏是否成功
	_, newCollected, err := a.getInteractState(page, feedID)
	if err == nil && newCollected {
		logrus.Infof("feed %s 收藏成功", feedID)
		return nil
	}
	
	if err != nil {
		logrus.Warnf("验证收藏状态失败: %v", err)
	} else {
		logrus.Warnf("feed %s 收藏可能未成功，状态未变化，尝试再次点击", feedID)
		// 如果第一次点击失败，尝试再次点击
		if err := clickLastMatch(page, []string{"#collect"}); err != nil {
			logrus.Warnf("第二次点击收藏按钮也失败: %v", err)
		} else {
			time.Sleep(2 * time.Second)
			_, newCollected2, err2 := a.getInteractState(page, feedID)
			if err2 == nil && newCollected2 {
				logrus.Infof("feed %s 第二次点击收藏成功", feedID)
				return nil
			}
		}
	}
	
	return nil
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

// clickLastMatch 点击选择器匹配的最后一个元素
func clickLastMatch(page *rod.Page, selectors []string) error {
	for _, sel := range selectors {
		if els, err := page.Elements(sel); err == nil && len(els) > 0 {
			// 直接点击最后一个元素
			lastEl := els[len(els)-1]
			if err := lastEl.Click("left", 1); err == nil {
				return nil
			}
		}
		// 单个元素回退
		if el, err := page.Element(sel); err == nil && el != nil {
			if err := el.Click("left", 1); err == nil {
				return nil
			}
		}
	}
	return errors.New("no clickable element matched for selectors")
}