package xiaohongshu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLongArticleMarkdown(t *testing.T) {
	md := `# 一级标题

## 二级标题

普通段落，包含 ==高亮== 和表情 :smile: 。

- 无序1
- 无序2

1. 有序1
2. 有序2

> 引用内容

![alt](images/a.png)
`

	blocks, err := parseLongArticleMarkdown(md)
	require.NoError(t, err)
	require.NotEmpty(t, blocks)

	require.Equal(t, blockH1, blocks[0].Type)
	require.Equal(t, "一级标题", blocks[0].Inlines[0].Text)

	require.Equal(t, blockH2, blocks[1].Type)
	require.Equal(t, "二级标题", blocks[1].Inlines[0].Text)

	// 段落
	require.Equal(t, blockParagraph, blocks[2].Type)
	// 至少包含一个高亮片段和一个 emoji 片段
	foundHL := false
	foundEmoji := false
	for _, inl := range blocks[2].Inlines {
		if inl.Highlight && inl.Text == "高亮" {
			foundHL = true
		}
		if inl.Emoji == "😄" {
			foundEmoji = true
		}
	}
	require.True(t, foundHL)
	require.True(t, foundEmoji)

	require.Equal(t, blockUnorderedList, blocks[3].Type)
	require.Len(t, blocks[3].Items, 2)
	require.Equal(t, "无序1", blocks[3].Items[0][0].Text)

	require.Equal(t, blockOrderedList, blocks[4].Type)
	require.Len(t, blocks[4].Items, 2)
	require.Equal(t, "有序1", blocks[4].Items[0][0].Text)

	require.Equal(t, blockQuote, blocks[5].Type)
	require.Equal(t, "引用内容", blocks[5].Inlines[0].Text)

	require.Equal(t, blockImage, blocks[6].Type)
	require.Equal(t, "images/a.png", blocks[6].Image)
}
