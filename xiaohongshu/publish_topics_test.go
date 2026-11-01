package xiaohongshu

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTopicCandidateExactMatch(t *testing.T) {
	for _, tt := range []struct {
		label string
		want  bool
	}{
		{"#大模型", true}, {"大模型\n123次浏览", true}, {"#大模型 新建话题", true},
		{"#大模型应用", false}, {"大模型应用\n123次浏览", false}, {"", false},
	} {
		assert.Equal(t, tt.want, matchesTopicCandidate(tt.label, "大模型"), tt.label)
	}
	assert.Equal(t, "大模型", normalizeTopicName(" ##大模型 "))
}
