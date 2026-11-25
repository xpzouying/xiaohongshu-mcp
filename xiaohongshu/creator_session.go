package xiaohongshu

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	xhserrors "github.com/xpzouying/xiaohongshu-mcp/errors"
)

const (
	creatorHost        = "creator.xiaohongshu.com"
	creatorPublishPath = "/publish/publish"
	urlOfPublic        = "https://" + creatorHost + creatorPublishPath + "?source=official"

	creatorReadyTimeout = 20 * time.Second
)

func newPublishAction(page *rod.Page, tabName string) (*PublishAction, error) {
	setup := page.Timeout(creatorReadyTimeout)
	defer setup.CancelTimeout()

	if err := setup.Navigate(urlOfPublic); err != nil {
		return nil, errors.Wrap(err, "打开创作者中心发布页失败")
	}
	if err := waitCreatorPublishPage(setup); err != nil {
		return nil, err
	}
	if err := clickPublishTab(setup, tabName); err != nil {
		return nil, errors.Wrapf(err, "切换到%s失败", tabName)
	}

	// 等标签切换后再确认上传控件。
	select {
	case <-setup.GetContext().Done():
		return nil, setup.GetContext().Err()
	case <-time.After(time.Second):
	}
	if err := waitCreatorPublishPage(setup); err != nil {
		return nil, err
	}
	return &PublishAction{page: page}, nil
}

func waitCreatorPublishPage(page *rod.Page) error {
	ctx := page.GetContext()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var onLoginPage bool
	for ctx.Err() == nil {
		// 通过 CDP 读取主框架地址，使查询受页面超时约束。
		tree, err := (proto.PageGetFrameTree{}).Call(page)
		if err == nil && tree.FrameTree != nil && tree.FrameTree.Frame != nil {
			canUpload, blocked := creatorPublishRoute(tree.FrameTree.Frame.URL)
			if blocked == xhserrors.ErrVerificationRequired {
				return blocked
			}
			onLoginPage = blocked == xhserrors.ErrLoginRequired
			if canUpload && hasCreatorUploadPanel(page) {
				return nil
			}
		}

		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	// SSO 可能短暂经过登录页，只有超时后仍在登录页才提示扫码。
	if onLoginPage && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return xhserrors.ErrLoginRequired
	}
	return errors.Wrap(ctx.Err(), "创作者中心发布页未就绪，未找到可用的上传入口")
}

func creatorPublishRoute(rawURL string) (canUpload bool, blocked error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, nil
	}
	host, path := u.Hostname(), u.EscapedPath()
	if u.Scheme != "https" || (host != "xiaohongshu.com" && !strings.HasSuffix(host, ".xiaohongshu.com")) {
		return false, nil
	}
	if err := classifyBlockedPath(path); err != nil {
		return false, err
	}
	if host != creatorHost {
		return false, nil
	}
	if path == "/login" || strings.HasPrefix(path, "/login/") {
		return false, xhserrors.ErrLoginRequired
	}
	return strings.TrimSuffix(path, "/") == creatorPublishPath, nil
}

func hasCreatorUploadPanel(page *rod.Page) bool {
	found, panel, err := page.Has("div.upload-content")
	if err != nil || !found {
		return false
	}
	visible, err := panel.Visible()
	if err != nil || !visible {
		return false
	}
	found, _, err = panel.Has("input[type=file]")
	return err == nil && found
}
