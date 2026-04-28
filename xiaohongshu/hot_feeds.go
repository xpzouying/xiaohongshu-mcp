package xiaohongshu

import (
	"sort"
	"strconv"
	"strings"
)

// ParseInteractCount 解析小红书互动数字符串为 int
// 支持格式: "568", "999+", "1.2万", "10万+"
func ParseInteractCount(s string) int {
	if s == "" {
		return 0
	}

	// 去掉末尾的 "+"
	s = strings.TrimSuffix(s, "+")

	// 处理 "万" 单位
	if strings.HasSuffix(s, "万") {
		s = strings.TrimSuffix(s, "万")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int(f * 10000)
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// SortFeeds 按指定指标降序排列 feeds（客户端排序，避免 UI hover 交互超时）
func SortFeeds(feeds []Feed, sortBy string) []Feed {
	result := make([]Feed, len(feeds))
	copy(result, feeds)
	sort.Slice(result, func(i, j int) bool {
		ii := result[i].NoteCard.InteractInfo
		jj := result[j].NoteCard.InteractInfo
		switch sortBy {
		case "最多收藏":
			return ParseInteractCount(ii.CollectedCount) > ParseInteractCount(jj.CollectedCount)
		case "最多评论":
			return ParseInteractCount(ii.CommentCount) > ParseInteractCount(jj.CommentCount)
		default: // 最多点赞
			return ParseInteractCount(ii.LikedCount) > ParseInteractCount(jj.LikedCount)
		}
	})
	return result
}

// FilterByThreshold 按互动数阈值过滤 feeds，minXxx <= 0 表示不限
func FilterByThreshold(feeds []Feed, minLikes, minFavorites, minComments int) []Feed {
	var result []Feed
	for _, f := range feeds {
		info := f.NoteCard.InteractInfo
		if minLikes > 0 && ParseInteractCount(info.LikedCount) < minLikes {
			continue
		}
		if minFavorites > 0 && ParseInteractCount(info.CollectedCount) < minFavorites {
			continue
		}
		if minComments > 0 && ParseInteractCount(info.CommentCount) < minComments {
			continue
		}
		result = append(result, f)
	}
	return result
}
