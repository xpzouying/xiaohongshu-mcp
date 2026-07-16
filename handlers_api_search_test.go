package main

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func TestSearchErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name: "timeout",
			err: &xiaohongshu.SearchError{
				Code: "SEARCH_TIMEOUT", Stage: "click_filter_option", Err: context.DeadlineExceeded,
			},
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   "SEARCH_TIMEOUT",
		},
		{
			name: "canceled",
			err: &xiaohongshu.SearchError{
				Code: "SEARCH_CANCELED", Stage: "wait_initial_state", Err: context.Canceled,
			},
			wantStatus: http.StatusRequestTimeout,
			wantCode:   "SEARCH_CANCELED",
		},
		{
			name:       "generic",
			err:        errors.New("failed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "SEARCH_FEEDS_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, details := searchErrorResponse(tt.err)
			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, tt.wantCode, code)
			require.NotNil(t, details)
		})
	}
}
