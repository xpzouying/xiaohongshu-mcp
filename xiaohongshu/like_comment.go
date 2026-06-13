package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

// LikeCommentAction 评论点赞动作
type LikeCommentAction struct {
	page *rod.Page
}

func NewLikeCommentAction(page *rod.Page) *LikeCommentAction {
	return &LikeCommentAction{page: page}
}

// LikeComment 点赞指定评论
func (a *LikeCommentAction) LikeComment(ctx context.Context, feedID, xsecToken, commentID string) error {
	page := a.page.Timeout(2 * time.Minute)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("打开帖子页面进行评论点赞: %s", url)

	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(2 * time.Second)

	if err := checkPageAccessible(page); err != nil {
		return err
	}

	// 等待评论加载到DOM（等待.parent-comment出现）
	loaded := false
	for i := 0; i < 15; i++ {
		count, err := page.Eval(`() => document.querySelectorAll('.parent-comment').length`)
		if err == nil && count != nil && count.Value.Int() > 0 {
			logrus.Infof("评论已加载: %d 条 (等待 %d 秒)", count.Value.Int(), i)
			loaded = true
			break
		}
		// 触发懒加载
		page.MustEval(`() => {
			const c = document.querySelector('.comments-container');
			if (c) c.scrollIntoView();
			let t = document.querySelector('.note-scroller') || document.documentElement;
			t.dispatchEvent(new WheelEvent('wheel', {deltaY: 300, deltaMode: 0, bubbles: true, cancelable: true, view: window}));
			window.scrollBy(0, 300);
		}`)
		time.Sleep(1 * time.Second)
	}
	if !loaded {
		return fmt.Errorf("评论未加载，无法点赞")
	}

	// 用JS在__INITIAL_STATE__中找到评论的索引，然后点击对应的like按钮
	js := fmt.Sprintf(`() => {
		const state = window.__INITIAL_STATE__;
		if (!state || !state.note) return JSON.stringify({error: "no state"});
		const inner = state.note._rawValue || state.note;
		const dm = inner.noteDetailMap;
		if (!dm) return JSON.stringify({error: "no noteDetailMap"});

		let comments = null;
		for (const key of Object.keys(dm)) {
			if (dm[key] && dm[key].comments && dm[key].comments.list) {
				comments = dm[key].comments.list;
				break;
			}
		}
		if (!comments) return JSON.stringify({error: "no comments list"});

		// 查找目标评论（主评论或子评论）
		const targetId = "%s";
		for (let i = 0; i < comments.length; i++) {
			if (comments[i].id === targetId) {
				return JSON.stringify({found: true, type: "parent", index: i, isLiked: comments[i].liked || false});
			}
			const subs = comments[i].subComments || [];
			for (let j = 0; j < subs.length; j++) {
				if (subs[j].id === targetId) {
					return JSON.stringify({found: true, type: "sub", parentIndex: i, subIndex: j, isLiked: subs[j].liked || false});
				}
			}
		}
		return JSON.stringify({error: "comment not found in state", totalComments: comments.length, ids: comments.map(c => c.id)});
	}`, commentID)

	result, err := page.Eval(js)
	if err != nil {
		return fmt.Errorf("查找评论失败: %w", err)
	}

	var findResult struct {
		Found       bool     `json:"found"`
		Error       string   `json:"error"`
		Type        string   `json:"type"`
		Index       int      `json:"index"`
		ParentIndex int      `json:"parentIndex"`
		SubIndex    int      `json:"subIndex"`
		IsLiked     bool     `json:"isLiked"`
		IDs         []string `json:"ids"`
	}
	json.Unmarshal([]byte(result.Value.String()), &findResult)

	if findResult.Error != "" {
		return fmt.Errorf("评论未找到: %s (已有评论IDs: %v)", findResult.Error, findResult.IDs)
	}

	if findResult.IsLiked {
		logrus.Infof("评论 %s 已经点赞过了", commentID)
		return nil
	}

	// 根据类型找到对应的DOM元素并点击
	var likeBtn *rod.Element
	if findResult.Type == "parent" {
		// 主评论：找第N个.parent-comment的.like-wrapper
		elements, err := page.Elements(".parent-comment")
		if err != nil || findResult.Index >= len(elements) {
			return fmt.Errorf("找不到第 %d 个评论元素", findResult.Index)
		}
		commentEl := elements[findResult.Index]
		commentEl.MustScrollIntoView()
		time.Sleep(300 * time.Millisecond)

		// 主评论的like-wrapper是第一个（子评论的在后面）
		likeBtn, err = commentEl.Element(".comment-inner .like-wrapper")
		if err != nil {
			// fallback: 找第一个like-wrapper
			likeBtn, err = commentEl.Element(".like-wrapper")
			if err != nil {
				return fmt.Errorf("找不到评论的点赞按钮: %w", err)
			}
		}
	} else {
		// 子评论：找第parentIndex个.parent-comment下的第subIndex个子评论的like-wrapper
		elements, err := page.Elements(".parent-comment")
		if err != nil || findResult.ParentIndex >= len(elements) {
			return fmt.Errorf("找不到第 %d 个评论元素", findResult.ParentIndex)
		}
		parentEl := elements[findResult.ParentIndex]
		parentEl.MustScrollIntoView()
		time.Sleep(300 * time.Millisecond)

		// 子评论在.reply-container里
		subComments, err := parentEl.Elements(".reply-container .comment-item-sub, .reply-container .sub-comment")
		if err != nil || findResult.SubIndex >= len(subComments) {
			return fmt.Errorf("找不到第 %d 个子评论元素", findResult.SubIndex)
		}
		subEl := subComments[findResult.SubIndex]
		subEl.MustScrollIntoView()
		time.Sleep(300 * time.Millisecond)

		likeBtn, err = subEl.Element(".like-wrapper")
		if err != nil {
			return fmt.Errorf("找不到子评论的点赞按钮: %w", err)
		}
	}

	// 检查是否已点赞（DOM层面）
	cls, _ := likeBtn.Attribute("class")
	if cls != nil && strings.Contains(*cls, "like-active") {
		logrus.Infof("评论 %s 已经点赞过了（DOM确认）", commentID)
		return nil
	}

	// 点击点赞
	if err := likeBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("点击点赞按钮失败: %w", err)
	}

	time.Sleep(2 * time.Second)

	// 验证点赞是否成功
	cls, _ = likeBtn.Attribute("class")
	if cls != nil && strings.Contains(*cls, "like-active") {
		logrus.Infof("评论 %s 点赞成功（已验证）", commentID)
		return nil
	}

	// 第一次没成功，再点一次
	logrus.Warnf("评论 %s 点赞状态未变化，重试一次", commentID)
	if err := likeBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("重试点击点赞按钮失败: %w", err)
	}
	time.Sleep(2 * time.Second)

	cls, _ = likeBtn.Attribute("class")
	if cls != nil && strings.Contains(*cls, "like-active") {
		logrus.Infof("评论 %s 第二次点击点赞成功", commentID)
		return nil
	}

	return fmt.Errorf("评论 %s 点赞可能未成功，状态未变化", commentID)
}
