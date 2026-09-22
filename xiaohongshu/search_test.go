package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectFilters(t *testing.T) {
	t.Run("只展开非空字段", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{{
			NoteType:    "图文",
			PublishTime: "一天内",
		}})
		require.NoError(t, err)
		require.Equal(t, []pendingFilter{
			{group: "笔记类型", option: "图文"},
			{group: "发布时间", option: "一天内"},
		}, pending)
	})

	t.Run("五个非默认字段全给", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{{
			SortBy:      "最新",
			NoteType:    "视频",
			PublishTime: "一周内",
			SearchScope: "已关注",
			Location:    "同城",
		}})
		require.NoError(t, err)
		require.Len(t, pending, 5)
		require.Equal(t, pendingFilter{group: "排序依据", option: "最新"}, pending[0])
		require.Equal(t, pendingFilter{group: "位置距离", option: "同城"}, pending[4])
	})

	t.Run("默认值不需要重复点击", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{
			{
				SortBy:      "综合",
				NoteType:    "不限",
				PublishTime: "不限",
				SearchScope: "不限",
				Location:    "不限",
			},
		})
		require.NoError(t, err)
		require.Empty(t, pending)
	})

	t.Run("只保留非默认筛选项", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{
			{
				SortBy:      "最新",
				NoteType:    "不限",
				PublishTime: "半年内",
				SearchScope: "不限",
				Location:    "不限",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []pendingFilter{
			{group: "排序依据", option: "最新"},
			{group: "发布时间", option: "半年内"},
		}, pending)
	})

	t.Run("同组最后一个默认值会清除前面的筛选", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{
			{NoteType: "图文"},
			{NoteType: "不限"},
		})
		require.NoError(t, err)
		require.Empty(t, pending)
	})

	t.Run("同组最后一个非默认值生效", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{
			{NoteType: "图文"},
			{NoteType: "视频"},
		})
		require.NoError(t, err)
		require.Equal(t, []pendingFilter{
			{group: "笔记类型", option: "视频"},
		}, pending)
	})

	t.Run("全空则无待应用项", func(t *testing.T) {
		pending, err := collectFilters([]FilterOption{{}})
		require.NoError(t, err)
		require.Empty(t, pending)
	})

	t.Run("非法取值在打开页面之前就报错", func(t *testing.T) {
		_, err := collectFilters([]FilterOption{{NoteType: "不存在的类型"}})
		require.Error(t, err)
		// 错误里要带上可选值，调用方才知道该怎么改
		require.Contains(t, err.Error(), "笔记类型")
		require.Contains(t, err.Error(), "图文")
	})

	t.Run("取值不能跨组", func(t *testing.T) {
		// 「视频」是笔记类型的选项，不能当排序依据
		_, err := collectFilters([]FilterOption{{SortBy: "视频"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "排序依据")
	})
}

// TestFilterGroupsCoverFilterOption 组表必须覆盖 FilterOption 的每个字段，
// 否则以后新增字段会被静默忽略。
func TestFilterGroupsCoverFilterOption(t *testing.T) {
	all := FilterOption{
		SortBy:      "sort_by",
		NoteType:    "note_type",
		PublishTime: "publish_time",
		SearchScope: "search_scope",
		Location:    "location",
	}

	var picked []string
	for _, g := range filterGroups {
		picked = append(picked, g.pick(all))
	}
	require.ElementsMatch(t, []string{
		"sort_by", "note_type", "publish_time", "search_scope", "location",
	}, picked, "组表漏了 FilterOption 的字段")

	for _, g := range filterGroups {
		require.NotEmpty(t, g.label)
		require.NotEmpty(t, g.defaultValue, "%s 没有默认值", g.label)
		require.NotEmpty(t, g.allowed, "%s 没有合法取值清单", g.label)
		require.Contains(t, g.allowed, g.defaultValue, "%s 的默认值不在合法取值中", g.label)
		require.NotNil(t, g.pick, "%s 没有取值函数", g.label)
	}
}
