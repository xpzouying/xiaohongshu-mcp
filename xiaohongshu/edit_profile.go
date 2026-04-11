package xiaohongshu

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// EditProfileRequest 编辑个人资料请求参数
type EditProfileRequest struct {
	Nickname string `json:"nickname,omitempty"` // 昵称
	Bio      string `json:"bio,omitempty"`      // 简介
}

type EditProfileAction struct {
	page *rod.Page
}

func NewEditProfileAction(page *rod.Page) *EditProfileAction {
	pp := page.Timeout(120 * time.Second)
	return &EditProfileAction{page: pp}
}

// EditProfile 编辑个人资料
func (e *EditProfileAction) EditProfile(ctx context.Context, edits EditProfileRequest) error {
	page := e.page.Context(ctx)

	// 1. 使用现有的导航逻辑到个人主页
	navigate := NewNavigate(page)
	if err := navigate.ToProfilePage(ctx); err != nil {
		return fmt.Errorf("failed to navigate to profile page: %w", err)
	}

	// 等待页面稳定
	page.MustWaitStable()

	// 2. 找到并点击编辑按钮
	editBtn, err := page.Element(`button[class*="edit"], .edit-profile, [data-testid="edit-profile"]`)
	if err != nil {
		return fmt.Errorf("failed to find edit button: %w", err)
	}
	if editBtn == nil {
		return fmt.Errorf("edit button not found")
	}
	editBtn.MustClick()

	// 3. 等待编辑弹窗或页面加载
	page.MustWaitStable()

	// 4. 按需填充各个字段
	if edits.Nickname != "" {
		if err := e.fillNickname(edits.Nickname); err != nil {
			return fmt.Errorf("failed to fill nickname: %w", err)
		}
	}

	if edits.Bio != "" {
		if err := e.fillBio(edits.Bio); err != nil {
			return fmt.Errorf("failed to fill bio: %w", err)
		}
	}

	// 5. 点击保存按钮
	saveBtn, err := page.Element(`button[type="submit"], .save-btn, [data-testid="save-profile"]`)
	if err != nil {
		return fmt.Errorf("failed to find save button: %w", err)
	}
	if saveBtn == nil {
		return fmt.Errorf("save button not found")
	}
	saveBtn.MustClick()

	// 6. 等待保存完成
	page.MustWaitStable()

	// 7. 验证保存结果
	return e.verifySaveResult()
}

// fillNickname 填充昵称
func (e *EditProfileAction) fillNickname(nickname string) error {
	// 尝试多种选择器查找昵称输入框
	selectors := []string{
		`input[name="nickname"], input[placeholder*="昵称"], .nickname-input`,
		`[data-testid="nickname-input"]`,
	}

	for _, selector := range selectors {
		input, err := e.page.Element(selector)
		if err == nil && input != nil {
			input.MustSelectAllText().MustInput(nickname)
			return nil
		}
	}

	return fmt.Errorf("nickname input field not found")
}

// fillBio 填充简介
func (e *EditProfileAction) fillBio(bio string) error {
	// 尝试多种选择器查找简介输入框
	selectors := []string{
		`textarea[name="bio"], textarea[placeholder*="简介"], .bio-textarea`,
		`[data-testid="bio-input"]`,
	}

	for _, selector := range selectors {
		textarea, err := e.page.Element(selector)
		if err == nil && textarea != nil {
			textarea.MustSelectAllText().MustInput(bio)
			return nil
		}
	}

	return fmt.Errorf("bio textarea field not found")
}

// verifySaveResult 验证保存结果
func (e *EditProfileAction) verifySaveResult() error {
	// 检查是否有成功提示或错误提示
	// 这里可以添加更详细的验证逻辑
	
	// 简单等待一下确保操作完成
	e.page.MustWaitStable()
	
	// 检查是否有错误提示
	errorMsg, err := e.page.Element(`.error-message, .ant-message-error, [role="alert"]`)
	if err == nil && errorMsg != nil {
		text := errorMsg.MustText()
		if text != "" {
			return fmt.Errorf("save failed: %s", text)
		}
	}

	// 检查是否有成功提示
	successMsg, err := e.page.Element(`.success-message, .ant-message-success`)
	if err == nil && successMsg != nil {
		// 有成功提示，说明保存成功
		return nil
	}

	// 如果没有明确的提示，假设保存成功
	return nil
}