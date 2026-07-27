package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsExpandRepliesButton 只有读得出「展开…回复」的按钮才允许自动点。
//
// 两种文案都必须认：大楼层点开一次后，「展开 49 条回复」会变成不带数字的
// 「展开更多回复」，只匹配数字那一种会在大楼层上半途停住。
func TestIsExpandRepliesButton(t *testing.T) {
	accepted := []string{
		"展开 49 条回复",
		"展开1条回复",
		" 展开 2 条回复 ",
		"展开更多回复",
	}
	for _, text := range accepted {
		assert.True(t, isExpandRepliesButton(text), "应认得: %q", text)
	}

	rejected := []string{
		"",
		"收起",
		"展开",
		"查看全部评论",
		"展开 2 条回复并关注", // 多余后缀，不是这个按钮
		"点击领取优惠券",     // 同 class 的其它元素
		"展开 abc 条回复",
	}
	for _, text := range rejected {
		assert.False(t, isExpandRepliesButton(text), "不该认: %q", text)
	}
}

// TestCommentLoadConfig_normalize 零值一律按「未设置」处理，回落到默认上限。
//
// 回归的是这个坑：HTTP 详情接口要的是嵌套的 comment_config，调用方传扁平字段
// 会被 ShouldBindJSON 静默丢掉 → MaxCommentItems=0 → 以前 0 表示无上限 →
// 一次请求滚满 defaultMaxAttempts(500) 轮。
func TestCommentLoadConfig_normalize(t *testing.T) {
	t.Run("空配置回落到默认", func(t *testing.T) {
		got := CommentLoadConfig{}.normalize()
		assert.Equal(t, defaultMaxCommentItems, got.MaxCommentItems)
		assert.Equal(t, defaultMaxRepliesThreshold, got.MaxRepliesThreshold)
		assert.Equal(t, defaultScrollSpeed, got.ScrollSpeed)
	})

	t.Run("负值同样按未设置处理", func(t *testing.T) {
		got := CommentLoadConfig{MaxCommentItems: -1, MaxRepliesThreshold: -5}.normalize()
		assert.Equal(t, defaultMaxCommentItems, got.MaxCommentItems)
		assert.Equal(t, defaultMaxRepliesThreshold, got.MaxRepliesThreshold)
	})

	t.Run("显式值不被覆盖", func(t *testing.T) {
		got := CommentLoadConfig{
			MaxCommentItems:     200,
			MaxRepliesThreshold: 3,
			ScrollSpeed:         "slow",
			ClickMoreReplies:    true,
		}.normalize()
		assert.Equal(t, 200, got.MaxCommentItems)
		assert.Equal(t, 3, got.MaxRepliesThreshold)
		assert.Equal(t, "slow", got.ScrollSpeed)
		assert.True(t, got.ClickMoreReplies)
	})

	t.Run("默认配置本身已规范化且有上限", func(t *testing.T) {
		d := DefaultCommentLoadConfig()
		assert.Equal(t, d, d.normalize())
		assert.Greater(t, d.MaxCommentItems, 0, "默认配置不能是无上限")
	})
}

// TestCalculateMaxAttempts 规范化之后不该再落到 500 轮那条兜底上。
func TestCalculateMaxAttempts(t *testing.T) {
	cl := &commentLoader{config: CommentLoadConfig{}.normalize()}
	assert.Equal(t, defaultMaxCommentItems*3, cl.calculateMaxAttempts())
	assert.Less(t, cl.calculateMaxAttempts(), defaultMaxAttempts,
		"默认配置的滚动轮数应远小于无上限时的 %d", defaultMaxAttempts)
}
