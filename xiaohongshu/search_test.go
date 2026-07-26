package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterValidation(t *testing.T) {
	// 测试有效的筛选选项转换
	validFilter := FilterOption{
		NoteType:    "图文",
		PublishTime: "一天内",
	}
	internalFilters, err := convertToInternalFilters(validFilter)
	require.NoError(t, err)
	require.Len(t, internalFilters, 2)

	// 转换结果应带上正确的组号与原文本
	require.Equal(t, 2, internalFilters[0].FiltersIndex)
	require.Equal(t, "图文", internalFilters[0].Text)
	require.Equal(t, 3, internalFilters[1].FiltersIndex)
	require.Equal(t, "一天内", internalFilters[1].Text)

	// 测试无效的筛选值
	invalidFilter := FilterOption{
		NoteType: "不存在的类型",
	}
	_, err = convertToInternalFilters(invalidFilter)
	require.Error(t, err)
	require.Contains(t, err.Error(), "未找到文本")

	// 测试所有有效的筛选选项
	allFilters := FilterOption{
		SortBy:      "最新",
		NoteType:    "视频",
		PublishTime: "一周内",
		SearchScope: "已关注",
		Location:    "同城",
	}
	internalFilters, err = convertToInternalFilters(allFilters)
	require.NoError(t, err)
	require.Len(t, internalFilters, 5)
}
