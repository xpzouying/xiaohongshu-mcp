package xiaohongshu

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProfileTab(t *testing.T) {
	cases := []struct {
		in      string
		want    ProfileTab
		wantErr bool
	}{
		{"", TabNotes, false},
		{"note", TabNotes, false},
		{"notes", TabNotes, false},
		{"笔记", TabNotes, false},
		{"fav", TabFavorites, false},
		{"FAVORITES", TabFavorites, false},
		{"收藏", TabFavorites, false},
		{"  liked  ", TabLiked, false},
		{"like", TabLiked, false},
		{"点赞", TabLiked, false},
		{"unknown", "", true},
		{"favourite", "", true},
	}

	for _, c := range cases {
		got, err := ParseProfileTab(c.in)
		if c.wantErr {
			assert.Error(t, err, "input=%q", c.in)
			continue
		}
		require.NoError(t, err, "input=%q", c.in)
		assert.Equal(t, c.want, got, "input=%q", c.in)
	}
}

func TestMakeUserProfileURL(t *testing.T) {
	base := makeUserProfileURL("uid1", "tok1", TabNotes)
	assert.Contains(t, base, "/user/profile/uid1")
	assert.Contains(t, base, "xsec_token=tok1")
	assert.NotContains(t, base, "tab=", "默认 tab 不应带 tab 参数")

	// 空值等同默认
	assert.Equal(t, base, makeUserProfileURL("uid1", "tok1", ""))

	fav := makeUserProfileURL("uid1", "tok1", TabFavorites)
	assert.Contains(t, fav, "tab=fav")
	assert.Contains(t, fav, "subTab=note")

	liked := makeUserProfileURL("uid1", "tok1", TabLiked)
	assert.Contains(t, liked, "tab=liked")
}

// tabLabel 要覆盖全部 tab，缺一个会让切换时找不到目标而报错。
func TestTabLabelCoverage(t *testing.T) {
	for _, tab := range []ProfileTab{TabNotes, TabFavorites, TabLiked} {
		label, ok := tabLabel[tab]
		assert.True(t, ok, "tab %q 缺少页面文字", tab)
		assert.NotEmpty(t, strings.TrimSpace(label))
	}
}
