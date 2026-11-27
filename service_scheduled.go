package main

import (
	"context"

	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func (s *XiaohongshuService) ListScheduledNotes(ctx context.Context) (*xiaohongshu.ScheduledNotesResponse, error) {
	b := newBrowser()
	defer b.Close()
	page := b.NewPage()
	defer page.Close()
	return xiaohongshu.ListScheduledNotes(ctx, page)
}
