package xiaohongshu

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
)

// 目标尚未渲染时，同一用户的其他评论不能替代指定评论。
func TestLookupCommentTargetMissingIDDoesNotFallback(t *testing.T) {
	otherComment := &rod.Element{}
	userLookupCalled := false
	got := lookupCommentTarget("test-child", "test-user",
		func(id string) *rod.Element {
			require.Equal(t, "test-child", id)
			return nil
		},
		func(id string) *rod.Element {
			userLookupCalled = true
			require.Equal(t, "test-user", id)
			return otherComment
		})

	require.Nil(t, got, "未找到指定评论时，应继续展开或滚动，不能回复该用户的其他评论")
	require.False(t, userLookupCalled, "提供评论 ID 时不能按用户查找")
}

func TestLookupCommentTargetIDTakesPriority(t *testing.T) {
	target := &rod.Element{}
	got := lookupCommentTarget("test-child", "test-user",
		func(id string) *rod.Element {
			require.Equal(t, "test-child", id)
			return target
		},
		func(string) *rod.Element {
			t.Fatal("提供评论 ID 时不能按用户查找")
			return nil
		})

	require.Same(t, target, got)
}

func TestLookupCommentTargetUserOnly(t *testing.T) {
	for _, found := range []bool{true, false} {
		name := "未找到用户评论"
		var target *rod.Element
		if found {
			name = "找到用户评论"
			target = &rod.Element{}
		}
		t.Run(name, func(t *testing.T) {
			got := lookupCommentTarget("", "test-user",
				func(string) *rod.Element {
					t.Fatal("未提供评论 ID 时不能按 ID 查找")
					return nil
				},
				func(id string) *rod.Element {
					require.Equal(t, "test-user", id)
					return target
				})

			if target == nil {
				require.Nil(t, got)
			} else {
				require.Same(t, target, got)
			}
		})
	}
}

func TestLookupCommentTargetWithoutIdentity(t *testing.T) {
	unexpectedLookup := func(string) *rod.Element {
		t.Fatal("未提供评论或用户 ID 时不应查询页面")
		return nil
	}
	got := lookupCommentTarget("", "", unexpectedLookup, unexpectedLookup)

	require.Nil(t, got)
}

// 目标加载后再次查找应命中目标，不能保留先前同作者的其他评论。
func TestLookupCommentTargetFindsIDOnRetry(t *testing.T) {
	target := &rod.Element{}
	loaded := false
	findByID := func(id string) *rod.Element {
		require.Equal(t, "test-child", id)
		if loaded {
			return target
		}
		return nil
	}
	findByUserID := func(string) *rod.Element {
		t.Fatal("重查指定评论时不能按用户查找")
		return nil
	}

	require.Nil(t, lookupCommentTarget("test-child", "test-user", findByID, findByUserID))
	loaded = true
	require.Same(t, target, lookupCommentTarget("test-child", "test-user", findByID, findByUserID))
}
