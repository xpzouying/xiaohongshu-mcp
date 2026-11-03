package xiaohongshu

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
)

func TestInputTagDoesNotWaitForPageTimeoutWhenSuggestionIsMissing(t *testing.T) {
	bin, err := browser.EnsureBrowser()
	require.NoError(t, err)

	u := launcher.New().Bin(bin).Headless(true).MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	defer b.MustClose()

	page := b.MustPage("about:blank").Timeout(8 * time.Second)
	page.MustSetDocumentContent(`<div id="content" role="textbox" contenteditable="true"></div>`)
	content := page.MustElement("#content")

	started := time.Now()
	err = inputTag(context.Background(), content, "x")
	elapsed := time.Since(started)

	require.NoError(t, err)
	assert.Less(t, elapsed, 6*time.Second)
}
