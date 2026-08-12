package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newAuthTestRouter(token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(authMiddleware(token))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func TestAuthMiddlewareAllowsRequestWhenDisabled(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)

	newAuthTestRouter("").ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthMiddlewareAllowsValidBearerToken(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer secret-token")

	newAuthTestRouter("secret-token").ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthMiddlewareRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
	}{
		{name: "missing header"},
		{name: "wrong scheme", authorization: "Basic secret-token"},
		{name: "lowercase bearer scheme", authorization: "bearer secret-token"},
		{name: "missing token", authorization: "Bearer "},
		{name: "wrong token", authorization: "Bearer wrong-token"},
		{name: "extra whitespace after scheme", authorization: "Bearer  secret-token"},
		{name: "leading whitespace", authorization: " Bearer secret-token"},
		{name: "trailing whitespace", authorization: "Bearer secret-token "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}

			newAuthTestRouter("secret-token").ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.Equal(t, "Bearer", recorder.Header().Get("WWW-Authenticate"))
			assert.JSONEq(t, `{"error":"未授权","code":"UNAUTHORIZED"}`, recorder.Body.String())
		})
	}
}
