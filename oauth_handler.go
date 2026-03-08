package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OAuthMetadata represents the OAuth 2.0 Authorization Server Metadata
type OAuthMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                     string   `json:"token_endpoint,omitempty"`
	JWKSURI                           string   `json:"jwks_uri,omitempty"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	ResponseModesSupported            []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

// oauthMetadataHandler returns OAuth 2.0 Authorization Server Metadata
// This endpoint is required for Claude Code and other MCP clients that expect OAuth support
func oauthMetadataHandler(c *gin.Context) {
	// Get the base URL from the request
	// Check both TLS and X-Forwarded-Proto for reverse proxy support
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	baseURL := scheme + "://" + host

	metadata := OAuthMetadata{
		Issuer:                 baseURL,
		AuthorizationEndpoint:  baseURL + "/oauth/authorize",
		TokenEndpoint:          baseURL + "/oauth/token",
		ScopesSupported:        []string{"mcp", "read", "write"},
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported:    []string{"authorization_code", "client_credentials"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post"},
	}

	// Cache for 1 hour to reduce repeated requests
	c.Header("Cache-Control", "max-age=3600")
	c.JSON(http.StatusOK, metadata)
}
