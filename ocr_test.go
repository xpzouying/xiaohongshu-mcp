package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func imageTextFixture() *FeedDetailResponse {
	return &FeedDetailResponse{Data: &xiaohongshu.FeedDetailResponse{
		Note: xiaohongshu.FeedDetail{
			Title: "Java 面经", Desc: "重复问题",
			ImageList: []xiaohongshu.DetailImageInfo{
				{URLDefault: "https://a.xhscdn.com/1"},
				{URLPre: "https://a.xhscdn.com/2"},
				{URLDefault: "https://a.xhscdn.com/3"},
			},
		},
	}}
}

func TestImageTextPreservesOrderDuplicatesAndPartialFailure(t *testing.T) {
	response := imageTextFixture()
	enrichFeedText(context.Background(), response, true, func(_ context.Context, urls []string) ([]ImageText, error) {
		require.Equal(t, []string{"https://a.xhscdn.com/1", "https://a.xhscdn.com/2", "https://a.xhscdn.com/3"}, urls)
		return []ImageText{{Text: "重复问题"}, {Error: "下载失败"}, {Text: "重复问题"}}, nil
	})
	require.Len(t, response.ImageTexts, 3)
	require.Equal(t, 2, response.ImageTexts[1].Index)
	require.Equal(t, "https://a.xhscdn.com/2", response.ImageTexts[1].URL)
	require.Equal(t, 3, strings.Count(response.TextContent, "重复问题"))
	require.Contains(t, response.TextContent, "## 图片 2\n识别失败：下载失败")
	require.Less(t, strings.Index(response.TextContent, "## 图片 2"), strings.Index(response.TextContent, "## 图片 3"))
	require.NotEmpty(t, response.OCRError)
}

func TestImageTextEngineFailureKeepsOriginalBody(t *testing.T) {
	response := imageTextFixture()
	enrichFeedText(context.Background(), response, true, func(context.Context, []string) ([]ImageText, error) {
		return nil, errors.New("OCR不可用")
	})
	require.Contains(t, response.TextContent, "## 正文\n重复问题")
	require.Contains(t, response.TextContent, "识别未完成")
	require.Equal(t, "OCR不可用", response.OCRError)
}

func TestImageTextIncompleteBatchIsNotReportedAsComplete(t *testing.T) {
	response := imageTextFixture()
	enrichFeedText(context.Background(), response, true, func(context.Context, []string) ([]ImageText, error) {
		return []ImageText{{Text: "只有一张"}}, nil
	})
	require.Contains(t, response.OCRError, "应返回3张，实际1张")
	require.Empty(t, response.ImageTexts)
}

func TestImageTextDisabledOrNoImagesSkipsEngine(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		response := imageTextFixture()
		if enabled {
			response.Data.(*xiaohongshu.FeedDetailResponse).Note.ImageList = nil
		}
		enrichFeedText(context.Background(), response, enabled, func(context.Context, []string) ([]ImageText, error) {
			t.Fatal("不应调用OCR")
			return nil, nil
		})
		require.Equal(t, "# Java 面经\n\n## 正文\n重复问题", response.TextContent)
		require.Empty(t, response.OCRError)
	}
}

func TestImageTextRequestDefaultsToDisabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		payload := `{"feed_id":"example","xsec_token":"example"}`
		if enabled {
			payload = `{"feed_id":"example","xsec_token":"example","include_image_text":true}`
		}
		var api FeedDetailRequest
		var mcp FeedDetailArgs
		require.NoError(t, json.Unmarshal([]byte(payload), &api))
		require.NoError(t, json.Unmarshal([]byte(payload), &mcp))
		require.Equal(t, enabled, api.IncludeImageText)
		require.Equal(t, enabled, mcp.IncludeImageText)
	}
}

func TestImageTextUnconfiguredEngineReturnsActionableError(t *testing.T) {
	t.Setenv("XHS_OCR_PYTHON", "")
	t.Setenv("XHS_OCR_SCRIPT", "")
	_, err := runLocalOCR(context.Background(), []string{"https://a.xhscdn.com/1"})
	require.ErrorContains(t, err, "XHS_OCR_PYTHON")
	require.ErrorContains(t, err, "XHS_OCR_SCRIPT")
}
