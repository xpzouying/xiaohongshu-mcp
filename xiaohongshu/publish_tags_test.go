package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopicTextMatches(t *testing.T) {
	cases := []struct {
		name string
		text string
		tag  string
		want bool
	}{
		{"带 # 前缀", "#gpt订阅", "gpt订阅", true},
		{"带浏览数后缀", "#gpt订阅 1.2万次浏览", "gpt订阅", true},
		{"大小写不同", "#GPT订阅", "gpt订阅", true},
		{"纯文本命中", "gpt订阅", "gpt订阅", true},
		{"不相关话题", "#chatgpt充值", "gpt订阅", false},
		{"只命中一部分", "#gpt", "gpt订阅", false},
		{"联想文本为空", "", "gpt订阅", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, topicTextMatches(c.text, c.tag))
		})
	}
}
