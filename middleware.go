package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// errorHandlingMiddleware 错误处理中间件
func errorHandlingMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logrus.WithField("event", "request_panic").Error("服务器内部错误")

		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR",
			"服务器内部错误", nil)
	})
}
