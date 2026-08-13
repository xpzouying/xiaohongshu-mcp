package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// respondError 是全仓库共用的错误出口。分级只是一行 if，改回去编译和其他单测
// 都不会报错，但鉴权开启后被扫描器打的 401 会重新刷满 ERROR。
func TestRespondErrorLogLevel(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  logrus.Level
	}{
		{name: "401 记为 warning", statusCode: http.StatusUnauthorized, wantLevel: logrus.WarnLevel},
		{name: "400 记为 warning", statusCode: http.StatusBadRequest, wantLevel: logrus.WarnLevel},
		{name: "500 记为 error", statusCode: http.StatusInternalServerError, wantLevel: logrus.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logrustest.NewGlobal()
			defer hook.Reset()

			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/login/status", nil)

			respondError(c, tt.statusCode, "CODE", "消息", nil)

			require.Len(t, hook.Entries, 1)
			assert.Equal(t, tt.wantLevel, hook.LastEntry().Level)
		})
	}
}
