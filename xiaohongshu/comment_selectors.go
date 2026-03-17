package xiaohongshu

import "fmt"

// commentSelectorsByID 根据评论 ID 生成多备用 CSS 选择器
// 小红书前端可能使用不同属性存储评论 ID，按优先级尝试
func commentSelectorsByID(commentID string) []string {
	return []string{
		fmt.Sprintf("#comment-%s", commentID),
		fmt.Sprintf(`[data-comment-id="%s"]`, commentID),
		fmt.Sprintf(`[data-id="%s"]`, commentID),
		fmt.Sprintf(`[id*="%s"]`, commentID),
	}
}

// commentSelectorsByUser 根据用户 ID 生成多备用 CSS 选择器
func commentSelectorsByUser(userID string) []string {
	return []string{
		fmt.Sprintf(`[data-user-id="%s"]`, userID),
		fmt.Sprintf(`[data-author-id="%s"]`, userID),
		fmt.Sprintf(`[data-uid="%s"]`, userID),
	}
}

// commentContainerSelectors 返回评论容器的候选选择器列表
func commentContainerSelectors() []string {
	return []string{
		".parent-comment",
		".comment-item",
		".comment",
		".comment-inner-container",
	}
}

// commentContainerSelectorString 返回拼接好的选择器字符串（逗号分隔）
func commentContainerSelectorString() string {
	selectors := commentContainerSelectors()
	result := selectors[0]
	for _, s := range selectors[1:] {
		result += ", " + s
	}
	return result
}
