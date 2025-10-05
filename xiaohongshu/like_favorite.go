package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// LikeFavoriteAction 点赞/收藏 动作
// 提供在笔记详情页执行点赞和收藏的能力，并在可能的情况下避免重复点击
// 通过读取 window.__INITIAL_STATE__ 判断当前状态
// 并尽量采用多种选择器/文案做回退，避免因页面样式变更导致失败
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

	// 依次尝试多种选择器或按文案匹配
	selectors := []string{
		"span.like-lottie",          // 页面提供的喜欢图标容器 (根据您提供的HTML)
		".like-lottie",              // 页面提供的喜欢图标容器
		"button.like",               // 常见按钮类名
		"div.interaction-bar .like", // 交互区域 like
		"div.footer .like",          // 底部工具栏
		".side-action .like",        // 侧边操作栏
		".like-wrapper",             // 包裹元素
		".interactions .like",       // 通用交互区
	}
	// 同时尝试 SVG use 的 like 图标
	selectors = append(selectors,
		"svg.like-icon", "use[href='#like']", "use[xlink\\:href='#like']",
	)
	textCandidates := []string{"点赞", "赞", "喜欢"}
	if err := clickFirstMatch(page, selectors, textCandidates); err != nil {
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
		if err := clickFirstMatch(page, selectors, textCandidates); err != nil {
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

	selectors := []string{
		"#note-page-collect-board-guide",           // 直接通过ID点击收藏按钮容器
		".collect-wrapper",                         // 收藏按钮的包裹容器
		".collect-wrapper svg",                     // 容器内的SVG
		".collect-wrapper .reds-icon.collect-icon", // 容器内的收藏图标
		".collect-wrapper use",                     // 容器内的use元素
		"use[xlink:href='#collect']",               // 直接点击SVG内部的use元素
		"use[href='#collect']",                      // 备用use选择器
		"svg.reds-icon.collect-icon use",            // SVG内部的use元素
		"svg.reds-icon.collect-icon",                // SVG容器（可能需要点击父容器）
		".reds-icon.collect-icon use",               // 类组合的use元素
		".reds-icon.collect-icon",                   // 类组合的容器
		"svg.collect-icon use",                      // 通用SVG收藏图标内部的use
		"svg.collect-icon",                          // 通用SVG收藏图标
		".collect-icon",                             // 通用收藏图标类
		"button.collect",                            // 常见按钮类名（收藏/收藏夹）
		"button.favorite",
		"div.interaction-bar .collect",
		"div.footer .collect",
		".side-action .collect",
		".interactions .collect",
	}
	textCandidates := []string{"收藏", "收藏夹", "喜欢"}
	if err := clickFirstMatch(page, selectors, textCandidates); err != nil {
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
		if err := clickFirstMatch(page, selectors, textCandidates); err != nil {
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

// clickFirstMatch 依次尝试选择器点击；若失败，尝试按按钮/链接文本模糊匹配
func clickFirstMatch(page *rod.Page, selectors []string, textCandidates []string) error {
	// 1) 尝试按选择器查找多个元素并点击（优先点击最后一个，即笔记的点赞按钮）
	for _, sel := range selectors {
		if els, err := page.Elements(sel); err == nil && len(els) > 0 {
			// 从最后一个元素开始尝试（笔记的点赞按钮通常在评论区之前）
			for i := len(els) - 1; i >= 0; i-- {
				if tryClickChain(els[i]) {
					return nil
				}
			}
		}
		// 单个元素回退
		if el, err := page.Element(sel); err == nil && el != nil {
			if tryClickChain(el) {
				return nil
			}
		}
	}
	// 2) 文案匹配：在按钮/链接/容器中查找包含文案的元素
	for _, txt := range textCandidates {
		if els, err := page.Elements("button, a, div, span, svg, use"); err == nil && len(els) > 0 {
			// 从最后一个元素开始尝试匹配文本
			for i := len(els) - 1; i >= 0; i-- {
				text, _ := els[i].Text()
				if strings.Contains(strings.ToLower(text), strings.ToLower(txt)) {
					if tryClickChain(els[i]) {
						return nil
					}
				}
			}
		}
		// 单个元素回退
		if el, err := page.ElementR("button, a, div, span, svg, use", fmt.Sprintf("(?i)%s", regexpEscape(txt))); err == nil && el != nil {
			if tryClickChain(el) {
				return nil
			}
		}
	}
	return errors.New("no clickable element matched for selectors/text")
}

// tryClickChain 对元素自身及其若干父级尝试点击（scrollIntoView + js click + rod click）
func tryClickChain(el *rod.Element) bool {
	current := el
	for i := 0; i < 6 && current != nil; i++ {
		if clickElement(current) {
			return true
		}
		parent, _ := current.Parent()
		current = parent
	}
	return false
}

func clickElement(el *rod.Element) bool {
	defer func() { _ = recover() }()
	// 滚动到可见区域
	_, _ = el.Eval(`() => { try { this.scrollIntoView({block: "center", inline: "center", behavior: "instant"}); } catch (e) {} return true }`)
	
	// 检查元素类型，对SVG元素使用特殊处理 - 简化处理，直接尝试所有方法
	// 不检查元素类型，直接尝试多种点击方式
	
	// 1. 尝试触发MouseEvent（对SVG元素特别有效）
	_, jsErr := el.Eval(`() => { 
		try { 
			const event = new MouseEvent('click', {
				view: window,
				bubbles: true,
				cancelable: true
			});
			this.dispatchEvent(event);
			return true; 
		} catch (e) { 
			console.error('MouseEvent click error:', e);
			return false; 
		} 
	}`)
	if jsErr == nil {
		return true
	}
	
	// 优先尝试标准 JS click
	_, jsErr2 := el.Eval(`() => { 
		try { 
			this.click(); 
			return true; 
		} catch (e) { 
			console.error('JS click error:', e);
			return false; 
		} 
	}`)
	if jsErr2 == nil {
		return true
	}
	
	// 再尝试 rod 的 Click
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return false
	}
	
	return true
}

// regexpEscape 对用户文案做正则转义，避免特殊字符
func regexpEscape(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		".", "\\.",
		"+", "\\+",
		"*", "\\*",
		"?", "\\?",
		"(", "\\(",
		")", "\\)",
		"[", "\\[",
		"]", "\\]",
		"{", "\\{",
		"}", "\\}",
		"^", "\\^",
		"$", "\\$",
		"|", "\\|",
	)
	return replacer.Replace(s)
}
