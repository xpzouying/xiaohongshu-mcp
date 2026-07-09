package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

func TestSearchUsers(t *testing.T) {
	if os.Getenv("XHS_RUN_BROWSER_TESTS") != "1" {
		t.Skip("skip browser integration test; set XHS_RUN_BROWSER_TESTS=1 to enable")
	}

	b := browser.NewBrowser(false)
	defer b.Close()

	page := b.NewPage()
	defer func() {
		_ = page.Close()
	}()

	action := NewSearchUserAction(page)

	users, err := action.SearchUsers(context.Background(), "张雨霏")
	require.NoError(t, err)
	require.NotEmpty(t, users, "users should not be empty")

	fmt.Printf("成功获取到 %d 个用户\n", len(users))

	for _, user := range users {
		fmt.Printf("User ID: %s\n", user.ID)
		fmt.Printf("User Name: %s\n", user.Name)
	}
}

func TestSearchUserUnmarshal(t *testing.T) {
	raw := `{
		"id": "test_user_id",
		"xsecToken": "test_xsec_token",
		"redId": "test_red_id",
		"name": "测试用户",
		"subTitle": "小红书号：test_red_id",
		"profession": "运动员",
		"fans": "1.2万",
		"noteCount": 10,
		"redOfficialVerified": true,
		"redOfficialVerifyType": 1,
		"image": "https://example.com/avatar.webp"
	}`

	var user SearchUser
	err := json.Unmarshal([]byte(raw), &user)
	require.NoError(t, err)

	require.Equal(t, "test_user_id", user.ID)
	require.Equal(t, "test_xsec_token", user.XsecToken)
	require.Equal(t, "test_red_id", user.RedID)
	require.Equal(t, "测试用户", user.Name)
	require.Equal(t, "小红书号：test_red_id", user.SubTitle)
	require.Equal(t, "运动员", user.Profession)
	require.Equal(t, "1.2万", user.Fans)
	require.Equal(t, 10, user.NoteCount)
	require.True(t, user.Verified)
	require.Equal(t, 1, user.VerificationType)
	require.Equal(t, "https://example.com/avatar.webp", user.AvatarURL)
}
