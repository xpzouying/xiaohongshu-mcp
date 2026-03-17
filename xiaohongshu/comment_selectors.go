package xiaohongshu

import (
	"fmt"
	"regexp"
	"strings"
)

// validIDPattern 校验 ID 是否为安全的十六进制字符串，防止选择器注入
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// isValidSelectorID 检查 ID 是否可以安全用于 CSS 选择器
func isValidSelectorID(id string) bool {
	return len(id) > 0 && validIDPattern.MatchString(id)
}

// commentSelectorsByID 根据评论 ID 生成多备用 CSS 选择器
// 小红书前端可能使用不同属性存储评论 ID，按优先级尝试
func commentSelectorsByID(commentID string) []string {
	if !isValidSelectorID(commentID) {
		return nil
	}
	return []string{
		fmt.Sprintf("#comment-%s", commentID),
		fmt.Sprintf(`[data-comment-id="%s"]`, commentID),
		fmt.Sprintf(`[data-id="%s"]`, commentID),
		fmt.Sprintf(`[id*="%s"]`, commentID),
	}
}

// commentSelectorsByUser 根据用户 ID 生成多备用 CSS 选择器
func commentSelectorsByUser(userID string) []string {
	if !isValidSelectorID(userID) {
		return nil
	}
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
	return strings.Join(commentContainerSelectors(), ", ")
}
