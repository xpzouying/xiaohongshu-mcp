package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *AppServer) handleListScheduledNotes(ctx context.Context) *MCPToolResult {
	result, err := s.xiaohongshuService.ListScheduledNotes(ctx)
	if err != nil {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "获取后台预约列表失败: " + err.Error()}}, IsError: true}
	}
	return marshalMCPResult(result, "获取后台预约列表")
}

func (s *AppServer) listScheduledNotesHandler(c *gin.Context) {
	result, err := s.xiaohongshuService.ListScheduledNotes(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_SCHEDULED_NOTES_FAILED", "获取后台预约列表失败", err.Error())
		return
	}
	respondSuccess(c, result, "获取后台预约列表成功")
}
