package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// FeedDetailAction 表示 Feed 详情页动作
type FeedDetailAction struct {
	page *rod.Page
}

// NewFeedDetailAction 创建 Feed 详情页动作
func NewFeedDetailAction(page *rod.Page) *FeedDetailAction {
	return &FeedDetailAction{page: page}
}

// GetFeedDetail 获取 Feed 详情页数据
func (f *FeedDetailAction) GetFeedDetail(ctx context.Context, feedID, xsecToken string) (*FeedDetailResponse, error) {
	page := f.page.Context(ctx).Timeout(60 * time.Second)

	// 构建详情页 URL
	url := makeFeedDetailURL(feedID, xsecToken)

	logrus.Infof("打开 feed 详情页: %s", url)

	// 导航到详情页
	page.MustNavigate(url)
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	result := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.note &&
		    window.__INITIAL_STATE__.note.noteDetailMap) {
			const noteDetailMap = window.__INITIAL_STATE__.note.noteDetailMap;
			return JSON.stringify(noteDetailMap);
		}
		return "";
	}`).String()

	if result == "" {
		return nil, errors.ErrNoFeedDetail
	}

	var noteDetailMap map[string]struct {
		Note     FeedDetail  `json:"note"`
		Comments CommentList `json:"comments"`
	}

	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal noteDetailMap: %w", err)
	}

	noteDetail, exists := noteDetailMap[feedID]
	if !exists {
		return nil, fmt.Errorf("feed %s not found in noteDetailMap", feedID)
	}

	return &FeedDetailResponse{
		Note:     noteDetail.Note,
		Comments: noteDetail.Comments,
	}, nil
}

func makeFeedDetailURL(feedID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/explore/%s?xsec_token=%s&xsec_source=pc_feed", feedID, xsecToken)
}
