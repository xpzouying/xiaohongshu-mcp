package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func TestBuildSearchFeedsCompactResponse(t *testing.T) {
	result := &FeedsListResponse{
		Feeds: []xiaohongshu.Feed{
			{
				ID:        "feed-1",
				XsecToken: "token-1",
				ModelType: "note",
				Index:     0,
				NoteCard: xiaohongshu.NoteCard{
					DisplayTitle: "标题一",
					User: xiaohongshu.User{
						UserID:   "user-1",
						Nickname: "作者一",
					},
				},
			},
			{
				ID:        "feed-2",
				XsecToken: "token-2",
				ModelType: "note",
				Index:     1,
				NoteCard: xiaohongshu.NoteCard{
					DisplayTitle: "标题二",
				},
			},
		},
		Count: 2,
	}

	compact := buildSearchFeedsCompactResponse(result)

	require.Equal(t, 2, compact.Count)
	require.Len(t, compact.Feeds, 2)
	require.Equal(t, SearchFeedSummary{
		ID:        "feed-1",
		XsecToken: "token-1",
		Title:     "标题一",
	}, compact.Feeds[0])
	require.Equal(t, SearchFeedSummary{
		ID:        "feed-2",
		XsecToken: "token-2",
		Title:     "标题二",
	}, compact.Feeds[1])
}

func TestBuildSearchFeedsCompactResponseNil(t *testing.T) {
	compact := buildSearchFeedsCompactResponse(nil)

	require.Equal(t, 0, compact.Count)
	require.Empty(t, compact.Feeds)
}

func TestBuildFeedDetailCompactResponse(t *testing.T) {
	result := &FeedDetailResponse{
		FeedID: "feed-1",
		Data: &xiaohongshu.FeedDetailResponse{
			Note: xiaohongshu.FeedDetail{
				NoteID:    "feed-1",
				XsecToken: "token-1",
				Title:     "标题",
				Desc:      "正文",
				Type:      "normal",
				User: xiaohongshu.User{
					Nickname: "作者",
				},
				InteractInfo: xiaohongshu.InteractInfo{
					LikedCount:     "1.2万",
					CommentCount:   "20",
					CollectedCount: "300",
					SharedCount:    "66",
				},
				ImageList: []xiaohongshu.DetailImageInfo{
					{URLDefault: "https://example.com/1.jpg"},
					{URLDefault: "https://example.com/2.jpg"},
				},
			},
			Comments: xiaohongshu.CommentList{
				List: []xiaohongshu.Comment{
					{
						ID:      "comment-1",
						Content: "普通评论",
						UserInfo: xiaohongshu.User{
							Nickname: "用户1",
						},
						LikeCount:       "12",
						SubCommentCount: "1",
						SubComments: []xiaohongshu.Comment{
							{
								ID:      "reply-1",
								Content: "回复 1",
								UserInfo: xiaohongshu.User{
									Nickname: "回复用户1",
								},
								LikeCount: "1",
							},
							{
								ID:      "reply-2",
								Content: "回复 2",
								UserInfo: xiaohongshu.User{
									Nickname: "回复用户2",
								},
								LikeCount: "20",
							},
						},
					},
					{
						ID:      "comment-2",
						Content: "热评",
						UserInfo: xiaohongshu.User{
							Nickname: "用户2",
						},
						LikeCount: "1.5万",
						ShowTags:  []string{"热评"},
					},
					{
						ID:      "comment-3",
						Content: "次热评",
						UserInfo: xiaohongshu.User{
							Nickname: "用户3",
						},
						LikeCount: "300",
					},
				},
				HasMore: true,
			},
		},
	}

	compact, err := buildFeedDetailCompactResponse(result, true, true)

	require.NoError(t, err)
	require.Equal(t, "feed-1", compact.FeedID)
	require.Equal(t, "token-1", compact.XsecToken)
	require.Equal(t, "标题", compact.Title)
	require.Equal(t, "正文", compact.Desc)
	require.Equal(t, "作者", compact.Author)
	require.Equal(t, 2, compact.ImageCount)
	require.Equal(t, 3, compact.CommentsLoaded)
	require.True(t, compact.HasMoreComments)
	require.Equal(t, "like_count_desc", compact.CommentsSortBy)
	require.True(t, compact.LoadAllComments)
	require.True(t, compact.RepliesExpanded)
	require.Len(t, compact.Comments, 3)
	require.Equal(t, "comment-2", compact.Comments[0].ID)
	require.Equal(t, "comment-3", compact.Comments[1].ID)
	require.Equal(t, "comment-1", compact.Comments[2].ID)
	require.Len(t, compact.Comments[2].Replies, 2)
	require.Equal(t, "reply-2", compact.Comments[2].Replies[0].ID)
	require.Equal(t, "reply-1", compact.Comments[2].Replies[1].ID)
}

func TestBuildUserProfileCompactResponse(t *testing.T) {
	result := &UserProfileResponse{
		UserBasicInfo: xiaohongshu.UserBasicInfo{
			Nickname:   "测试用户",
			RedId:      "red123",
			Desc:       "这是简介",
			Gender:     1,
			IpLocation: "上海",
		},
		Interactions: []xiaohongshu.UserInteractions{
			{Type: "fans", Name: "粉丝", Count: "1000"},
		},
		Feeds: []xiaohongshu.Feed{
			{
				ID:        "feed-1",
				XsecToken: "token-1",
				NoteCard: xiaohongshu.NoteCard{
					DisplayTitle: "笔记标题",
					Cover: xiaohongshu.Cover{
						URL:    "https://example.com/cover.jpg",
						Width:  640,
						Height: 480,
					},
				},
			},
		},
	}

	compact := buildUserProfileCompactResponse(result)

	require.Equal(t, "测试用户", compact.Nickname)
	require.Equal(t, "red123", compact.RedId)
	require.Equal(t, "这是简介", compact.Desc)
	require.Equal(t, 1, compact.Gender)
	require.Equal(t, "上海", compact.IpLocation)
	require.Len(t, compact.Interactions, 1)
	require.Equal(t, 1, compact.FeedCount)
	require.Len(t, compact.Feeds, 1)
	require.Equal(t, "feed-1", compact.Feeds[0].ID)
	require.Equal(t, "笔记标题", compact.Feeds[0].Title)
}

func TestBuildUserProfileCompactResponseNil(t *testing.T) {
	compact := buildUserProfileCompactResponse(nil)

	require.Empty(t, compact.Nickname)
	require.Empty(t, compact.Feeds)
	require.Empty(t, compact.Interactions)
	require.Equal(t, 0, compact.FeedCount)
}

func TestParseLikeCount(t *testing.T) {
	require.Equal(t, 0.0, parseLikeCount(""))
	require.Equal(t, 12.0, parseLikeCount("12"))
	require.Equal(t, 12000.0, parseLikeCount("1.2万"))
	require.Equal(t, 23000.0, parseLikeCount("2.3w"))
	require.Equal(t, 3000.0, parseLikeCount("3千"))
}
