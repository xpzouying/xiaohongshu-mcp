package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
)

type authMode string

const (
	authModeOff     authMode = "off"
	authModeWarn    authMode = "warn"
	authModeEnforce authMode = "enforce"
)

type accessScope string

const (
	scopeRead  accessScope = "read"
	scopeWrite accessScope = "write"
	scopeAdmin accessScope = "admin"
)

type scopedCredential struct {
	token  string
	actor  string
	scopes map[accessScope]struct{}
}

type requestPrincipal struct {
	actor  string
	scopes map[accessScope]struct{}
}

type principalContextKey struct{}

const (
	internalActorHeader = "X-XHS-Authenticated-Actor"
	internalScopeHeader = "X-XHS-Authenticated-Scopes"
)

type backendSecurityConfig struct {
	mode           authMode
	token          string // 兼容迁移期的旧单 token。
	credentials    []scopedCredential
	allowedOrigins map[string]struct{}
	allowInsecure  bool
	tokenFileError bool
}

func loadBackendSecurityConfig() backendSecurityConfig {
	mode := parseAuthMode(os.Getenv("XHS_AUTH_MODE"))
	tokenFile, tokenFileSet := os.LookupEnv("XHS_API_TOKEN_FILE")
	var token string
	var err error
	if tokenFileSet && strings.TrimSpace(tokenFile) != "" {
		token, err = readSecretFile(tokenFile)
	}
	fileError := err != nil
	credentials := make([]scopedCredential, 0, 6)
	for _, item := range []struct {
		scope             accessScope
		current, previous string
	}{
		{scopeRead, "XHS_READ_TOKEN_FILE", "XHS_READ_TOKEN_PREVIOUS_FILE"},
		{scopeWrite, "XHS_WRITE_TOKEN_FILE", "XHS_WRITE_TOKEN_PREVIOUS_FILE"},
		{scopeAdmin, "XHS_ADMIN_TOKEN_FILE", "XHS_ADMIN_TOKEN_PREVIOUS_FILE"},
	} {
		for _, name := range []string{item.current, item.previous} {
			path, set := os.LookupEnv(name)
			if !set || strings.TrimSpace(path) == "" {
				continue
			}
			value, readErr := readSecretFile(path)
			if readErr != nil {
				fileError = true
				continue
			}
			credentials = append(credentials, newScopedCredential(value, item.scope))
		}
	}
	if err != nil && mode != authModeOff && len(credentials) == 0 {
		logrus.Warn("后端 API token 未配置或不可用")
	}
	return backendSecurityConfig{
		mode:           mode,
		token:          token,
		credentials:    credentials,
		allowedOrigins: parseOriginAllowlist(os.Getenv("XHS_CORS_ALLOWED_ORIGINS")),
		allowInsecure:  parseInsecureTestMode(os.Getenv("XHS_ALLOW_INSECURE_TEST_MODE")),
		tokenFileError: fileError,
	}
}

func newScopedCredential(token string, scopes ...accessScope) scopedCredential {
	set := make(map[accessScope]struct{}, len(scopes))
	for _, scope := range scopes {
		set[scope] = struct{}{}
	}
	return scopedCredential{token: token, actor: hashAuditValue(token), scopes: set}
}

func (config backendSecurityConfig) configuredCredentials() []scopedCredential {
	credentials := append([]scopedCredential(nil), config.credentials...)
	if validBearerTokenSyntax(config.token) {
		// 旧单 token 在迁移窗口保留原能力；独立 admin token 不隐含 read/write。
		credentials = append(credentials, newScopedCredential(config.token, scopeRead, scopeWrite, scopeAdmin))
	}
	return credentials
}

func parseInsecureTestMode(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func parseAuthMode(value string) authMode {
	switch authMode(strings.ToLower(strings.TrimSpace(value))) {
	case authModeOff:
		return authModeOff
	case authModeWarn:
		return authModeWarn
	default:
		return authModeEnforce
	}
}

func readSecretFile(path string) (string, error) {
	return readSecretFileFrom(path, openSecretFileNoFollow)
}

func readSecretFileFrom(path string, openFile func(string) (*os.File, error)) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("secret file path is required")
	}
	file, err := openFile(path)
	if err != nil {
		return "", errors.New("secret file is unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", errors.New("secret file is unavailable")
	}
	permissions := info.Mode().Perm()
	if !info.Mode().IsRegular() || permissions != 0o400 && permissions != 0o600 {
		return "", errors.New("secret file must be a private regular file")
	}
	value, err := io.ReadAll(file)
	if err != nil {
		return "", errors.New("secret file is unavailable")
	}
	secret := strings.TrimSuffix(string(value), "\n")
	secret = strings.TrimSuffix(secret, "\r")
	if secret == "" {
		return "", errors.New("secret file is empty")
	}
	if !validBearerTokenSyntax(secret) {
		return "", errors.New("secret file contains an invalid bearer token")
	}
	return secret, nil
}

func parseOriginAllowlist(value string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if isExactOrigin(origin) {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

func isExactOrigin(origin string) bool {
	if origin == "" || strings.Contains(origin, "*") {
		return false
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.Path == ""
}

func backendAuthMiddleware(config backendSecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 内部身份头只允许由鉴权中间件生成。
		c.Request.Header.Del(internalActorHeader)
		c.Request.Header.Del(internalScopeHeader)
		path := c.Request.URL.Path
		protected := path == "/mcp" || strings.HasPrefix(path, "/mcp/") || path == "/api/v1" || strings.HasPrefix(path, "/api/v1/")
		if !protected {
			c.Next()
			return
		}
		headers := c.Request.Header.Values("Authorization")
		if len(headers) > 0 && !validBearerAuthorizationSyntax(headers) {
			abortUnauthorized(c)
			return
		}
		credentials := config.configuredCredentials()
		if config.mode == authModeOff && !config.tokenFileError && (len(credentials) > 0 || config.allowInsecure) {
			setPrincipal(c, migrationPrincipal())
			c.Next()
			return
		}
		if len(credentials) == 0 || config.tokenFileError {
			abortUnauthorized(c)
			return
		}
		principal, valid := authenticateBearer(headers, credentials)
		if valid {
			setPrincipal(c, principal)
			c.Next()
			return
		}
		if config.mode == authModeWarn {
			logrus.WithField("event", "authentication_warning").Warn("未认证的后端请求")
			setPrincipal(c, migrationPrincipal())
			c.Next()
			return
		}
		abortUnauthorized(c)
	}
}

func setPrincipal(c *gin.Context, principal requestPrincipal) {
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), principalContextKey{}, principal))
	c.Request.Header.Set(internalActorHeader, principal.actor)
	scopes := make([]string, 0, len(principal.scopes))
	for _, scope := range []accessScope{scopeRead, scopeWrite, scopeAdmin} {
		if principal.allows(scope) {
			scopes = append(scopes, string(scope))
		}
	}
	c.Request.Header.Set(internalScopeHeader, strings.Join(scopes, ","))
}

func principalFromContext(ctx context.Context) (requestPrincipal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(requestPrincipal)
	return principal, ok
}

func migrationPrincipal() requestPrincipal {
	return requestPrincipal{actor: "migration-anonymous", scopes: map[accessScope]struct{}{scopeRead: {}, scopeWrite: {}, scopeAdmin: {}}}
}

func (p requestPrincipal) allows(scope accessScope) bool {
	_, ok := p.scopes[scope]
	return ok
}

func principalFromMCPRequest(ctx context.Context, req *mcp.CallToolRequest) (requestPrincipal, bool) {
	if req != nil && req.Extra != nil {
		actor := req.Extra.Header.Get(internalActorHeader)
		scopeHeader := req.Extra.Header.Get(internalScopeHeader)
		if actor == "" || scopeHeader == "" {
			return requestPrincipal{}, false
		}
		scopes := make(map[accessScope]struct{})
		for _, value := range strings.Split(scopeHeader, ",") {
			scopes[accessScope(value)] = struct{}{}
		}
		return requestPrincipal{actor: actor, scopes: scopes}, true
	}
	return principalFromContext(ctx)
}

func authenticateBearer(headers []string, credentials []scopedCredential) (requestPrincipal, bool) {
	if !validBearerAuthorizationSyntax(headers) {
		return requestPrincipal{}, false
	}
	providedHash := sha256.Sum256([]byte(headers[0][len("Bearer "):]))
	for _, credential := range credentials {
		expectedHash := sha256.Sum256([]byte(credential.token))
		if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1 {
			return requestPrincipal{actor: credential.actor, scopes: credential.scopes}, true
		}
	}
	return requestPrincipal{}, false
}

func requireRESTScope(scope accessScope, operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("operation", operation)
		c.Set("scope", string(scope))
		principal, ok := principalFromContext(c.Request.Context())
		if !ok {
			abortUnauthorized(c)
			return
		}
		if !principal.allows(scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "code": "FORBIDDEN"})
			return
		}
		c.Next()
	}
}

func requestAuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = randomRequestID()
		}
		c.Header("X-Request-ID", requestID)
		started := time.Now()
		c.Next()
		logRESTAudit(c, requestID, started)
	}
}

func logRESTAudit(c *gin.Context, requestID string, started time.Time) {
	operationValue, exists := c.Get("operation")
	operation, ok := operationValue.(string)
	if !exists || !ok {
		return
	}
	principal, _ := principalFromContext(c.Request.Context())
	scope, _ := c.Get("scope")
	accountID := c.Param("id")
	if selected, selectedOK := c.Get("account"); selectedOK {
		accountID, _ = selected.(string)
	}
	outcome := "success"
	if c.Writer.Status() >= 400 {
		outcome = "failure"
	}
	if (c.Writer.Status() == http.StatusRequestTimeout || c.Writer.Status() == http.StatusGatewayTimeout) && isUncertainWriteOperation(operation) {
		outcome = "UNKNOWN"
	}
	logrus.WithFields(logrus.Fields{
		"event": "security_audit", "request_id": requestID, "actor": principal.actor,
		"scope": scope, "operation": operation, "account_id_hash": hashAuditValue(accountID),
		"target_hash": hashAuditValue(c.FullPath()), "outcome": outcome,
		"duration_ms": time.Since(started).Milliseconds(),
	}).Info("安全审计")
}

func isUncertainWriteOperation(operation string) bool {
	return strings.Contains(operation, "publish") || strings.Contains(operation, "comment") || strings.Contains(operation, "reply")
}

func isUncertainError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func uncertainHTTPStatus(err error) (int, bool) {
	if errors.Is(err, context.Canceled) {
		return http.StatusRequestTimeout, true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, true
	}
	return 0, false
}

func hashAuditValue(value string) string {
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func randomRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(value[:])
}

func abortUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer realm="xiaohongshu-mcp"`)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "code": "UNAUTHORIZED"})
}

func validBearerToken(headers []string, expected string) bool {
	_, ok := authenticateBearer(headers, []scopedCredential{newScopedCredential(expected, scopeRead)})
	return ok
}

func validBearerAuthorizationSyntax(headers []string) bool {
	if len(headers) != 1 {
		return false
	}
	header := headers[0]
	return len(header) > len("Bearer ") && strings.EqualFold(header[:len("Bearer")], "Bearer") && header[len("Bearer")] == ' ' && validBearerTokenSyntax(header[len("Bearer "):])
}

// validBearerTokenSyntax 校验 RFC 6750 b64token 语法。
func validBearerTokenSyntax(token string) bool {
	if token == "" {
		return false
	}
	body := false
	padding := false
	for _, char := range []byte(token) {
		if char == '=' {
			if !body {
				return false
			}
			padding = true
			continue
		}
		if padding || !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("-._~+/", rune(char))) {
			return false
		}
		body = true
	}
	return body
}

func corsMiddleware(allowedOrigins map[string]struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
		origin := c.GetHeader("Origin")
		if _, allowed := allowedOrigins[origin]; origin != "" && allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Idempotency-Key")
		}
		preflight := c.Request.Method == http.MethodOptions && origin != "" && c.GetHeader("Access-Control-Request-Method") != ""
		if preflight {
			if _, allowed := allowedOrigins[origin]; !allowed {
				c.Next()
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
