package xiaohongshu

import (
	"context"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

type NavigateAction struct {
	page *rod.Page
}

func NewNavigate(page *rod.Page) *NavigateAction {
	return &NavigateAction{page: page}
}

func (n *NavigateAction) ToExplorePage(ctx context.Context) error {
	page := n.page.Context(ctx)

	// 导航到探索页
	if err := page.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return errors.Wrap(err, "导航到探索页失败")
	}

	// 等待页面加载完成
	if err := page.WaitLoad(); err != nil {
		return errors.Wrap(err, "等待页面加载失败")
	}

	// 等待关键元素加载
	appElem, err := page.Element(`div#app`)
	if err != nil {
		return errors.Wrap(err, "等待应用容器元素失败，页面可能没有正常加载")
	}

	// 检查元素是否存在
	if appElem == nil {
		return errors.New("应用容器元素为空，页面加载可能失败")
	}

	return nil
}
