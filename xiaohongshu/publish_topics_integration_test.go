//go:build integration

package xiaohongshu

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestTopicNodeFixtures(t *testing.T) {
	page := fixturePage(t)
	for _, tt := range []struct {
		name, html string
		want       int
	}{
		{"plain text", `#大模型`, 0},
		{"ordinary link", `<a href="https://example.com">#大模型</a>`, 0},
		{"readonly node", `<span contenteditable="false">#大模型</span>`, 0},
		{"generic class", `<span class="topic hashtag">#大模型</span>`, 0},
		{"topic marker", `<span data-type="topic">#大模型</span>`, 1},
		{"topic suffix", `<span data-type="topic">#大模型[话题]#</span>`, 1},
		{"nested topic", `<span data-type="topic"><span data-type="topic">#大模型</span></span>`, 1},
		{"wrong topic", `<span data-type="topic">#大模型应用</span>`, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			page.MustSetDocumentContent(`<html><body><div id="editor" contenteditable="true">` + tt.html + `</div></body></html>`)
			got, err := editorTopicCount(page.MustElement("#editor"), "大模型")
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestTopicSelectionFixtures(t *testing.T) {
	page := fixturePage(t)
	for _, tt := range []struct {
		name, attrs string
		wantError   bool
	}{
		{"actual topic", `a.setAttribute('data-type','topic');`, false},
		{"ordinary link", `a.href='https://example.com';`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			page.MustSetDocumentContent(`<html><body><div id="editor" contenteditable="true" style="width:500px;height:100px"></div><div id="creator-editor-topic-container"><button class="item" onclick="var a=document.createElement('a');` + tt.attrs + `a.textContent='#大模型';document.getElementById('editor').appendChild(a)">#大模型</button></div></body></html>`)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			err := inputTag(ctx, page.MustElement("#editor"), "大模型")
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
