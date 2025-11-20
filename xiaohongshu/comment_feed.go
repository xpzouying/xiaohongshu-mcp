package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
)

// CommentFeedAction 表示 Feed 评论动作
type CommentFeedAction struct {
	page *rod.Page
}

// NewCommentFeedAction 创建 Feed 评论动作
func NewCommentFeedAction(page *rod.Page) *CommentFeedAction {
	return &CommentFeedAction{page: page}
}

// PostComment 发表评论到 Feed
func (f *CommentFeedAction) PostComment(ctx context.Context, feedID, xsecToken, content string) error {
	page := f.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("Opening feed detail page: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()

	time.Sleep(1 * time.Second)

	elem := page.MustElement("div.input-box div.content-edit span")
	elem.MustClick()

	elem2 := page.MustElement("div.input-box div.content-edit p.content-input")
	elem2.MustInput(content)

	time.Sleep(1 * time.Second)

	submitButton := page.MustElement("div.bottom button.submit")
	submitButton.MustClick()

	time.Sleep(1 * time.Second)

	return nil
}
