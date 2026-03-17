package xiaohongshu

import (
	"testing"
)

// #533/#504 评论选择器相关测试

// TestCommentSelectorsByID 验证按评论 ID 生成多备用选择器
func TestCommentSelectorsByID(t *testing.T) {
	commentID := "69ad291a000000001500915b"
	selectors := commentSelectorsByID(commentID)

	if len(selectors) < 3 {
		t.Errorf("应至少生成 3 个备用选择器，实际 %d 个", len(selectors))
	}

	// 应包含多种查找策略
	expected := []string{
		"#comment-69ad291a000000001500915b",
		`[data-comment-id="69ad291a000000001500915b"]`,
		`[data-id="69ad291a000000001500915b"]`,
	}
	for _, exp := range expected {
		found := false
		for _, sel := range selectors {
			if sel == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("缺少选择器: %s", exp)
		}
	}
}

// TestCommentSelectorsByUser 验证按用户 ID 生成备用选择器
func TestCommentSelectorsByUser(t *testing.T) {
	userID := "5ca9a7600000000012016267"
	selectors := commentSelectorsByUser(userID)

	if len(selectors) < 2 {
		t.Errorf("应至少生成 2 个备用选择器，实际 %d 个", len(selectors))
	}

	expected := []string{
		`[data-user-id="5ca9a7600000000012016267"]`,
		`[data-author-id="5ca9a7600000000012016267"]`,
	}
	for _, exp := range expected {
		found := false
		for _, sel := range selectors {
			if sel == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("缺少选择器: %s", exp)
		}
	}
}

// TestCommentContainerSelectors 验证评论容器选择器列表
func TestCommentContainerSelectors(t *testing.T) {
	selectors := commentContainerSelectors()

	if len(selectors) == 0 {
		t.Error("评论容器选择器不能为空")
	}

	// 应包含 .parent-comment
	found := false
	for _, sel := range selectors {
		if sel == ".parent-comment" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应包含 .parent-comment 选择器")
	}
}
