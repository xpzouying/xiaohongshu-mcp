package xiaohongshu

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSiteSelection(t *testing.T) {
	for _, tt := range []struct {
		site, web, creator string
		valid              bool
	}{
		{"", "https://www.rednote.com", "https://creator.rednote.com", true},
		{"rednote", "https://www.rednote.com", "https://creator.rednote.com", true},
		{" RedNote ", "https://www.rednote.com", "https://creator.rednote.com", true},
		{"xiaohongshu", "https://www.xiaohongshu.com", "https://creator.xiaohongshu.com", true},
		{" XIAOHONGSHU ", "https://www.xiaohongshu.com", "https://creator.xiaohongshu.com", true},
		{"rednot", "", "", false},
	} {
		t.Run(tt.site, func(t *testing.T) {
			t.Setenv("XHS_SITE", tt.site)
			if !tt.valid {
				assert.Error(t, ValidateSite())
				return
			}
			assert.NoError(t, ValidateSite())
			assert.Equal(t, tt.web+"/explore", webURL("/explore"))
			assert.Equal(t, tt.creator+"/publish/publish", creatorURL("/publish/publish"))
		})
	}
}

func TestCreatorPageRejectsWrongOriginAndLoginRedirects(t *testing.T) {
	t.Setenv("XHS_SITE", "rednote")
	for _, raw := range []string{
		"https://creator.rednote.com/login?lastUrl=/publish/success",
		"https://creator.xiaohongshu.com/publish/success",
		"https://creator.rednote.com.evil.example/publish/success",
		"http://creator.rednote.com/publish/success",
		"https://creator.rednote.com/publish/publish",
		"about:blank",
	} {
		assert.False(t, isCreatorPage(raw, "/publish/success"), raw)
	}
	assert.True(t, isCreatorPage("https://creator.rednote.com/publish/success?source=official", "/publish/success"))
	t.Setenv("XHS_SITE", "xiaohongshu")
	assert.True(t, isCreatorPage("https://creator.xiaohongshu.com/publish/success", "/publish/success"))
	assert.False(t, isCreatorPage("https://creator.rednote.com/publish/success", "/publish/success"))
}

func TestDraftSaveConfirmation(t *testing.T) {
	for _, text := range []string{"草稿已保存", "保存成功", "暂存成功", "已存入草稿箱", "Draft saved", "Saved to drafts"} {
		assert.True(t, draftSaveSucceeded(text), text)
	}
	for _, text := range []string{"草稿保存失败", "Failed to save draft", "Error: draft saved", "草稿箱(1)", "保存草稿", "暂存离开", "draft", ""} {
		assert.False(t, draftSaveSucceeded(text), text)
	}
}
