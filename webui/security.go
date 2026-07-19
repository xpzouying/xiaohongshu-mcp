package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type authMode string

const (
	authModeOff     authMode = "off"
	authModeWarn    authMode = "warn"
	authModeEnforce authMode = "enforce"
)

func loadHandlerConfig() handlerConfig {
	mode := parseAuthMode(os.Getenv("WEBUI_AUTH_MODE"))
	insecureTest := parseInsecureTestMode(os.Getenv("WEBUI_INSECURE_TEST_MODE"))
	password, passwordErr := readSecretFile(os.Getenv("WEBUI_PASSWORD_FILE"))
	legacyPath := os.Getenv("XHS_API_TOKEN_FILE")
	upstreamToken, tokenErr := readOptionalSecretFile(legacyPath)
	readPath := os.Getenv("XHS_READ_TOKEN_FILE")
	readToken, readErr := readOptionalSecretFile(readPath)
	writePath := os.Getenv("XHS_WRITE_TOKEN_FILE")
	writeToken, writeErr := readOptionalSecretFile(writePath)
	adminPath := os.Getenv("XHS_ADMIN_TOKEN_FILE")
	adminToken, adminErr := readOptionalSecretFile(adminPath)
	legacyConfigured := tokenErr == nil && upstreamToken != ""
	scopedConfigured := readErr == nil && writeErr == nil && adminErr == nil && readToken != "" && writeToken != "" && adminToken != ""
	securityError := strings.TrimSpace(legacyPath) != "" && tokenErr != nil ||
		strings.TrimSpace(readPath) != "" && readErr != nil ||
		strings.TrimSpace(writePath) != "" && writeErr != nil ||
		strings.TrimSpace(adminPath) != "" && adminErr != nil
	if !insecureTest && (passwordErr != nil || securityError || (!legacyConfigured && !scopedConfigured)) {
		log.Print("Web UI 鉴权 secret 未配置或不可用")
	}
	return handlerConfig{
		upstreamURL:    envOrDefault("XHS_MCP_URL", defaultUpstreamURL),
		externalScheme: os.Getenv("WEBUI_EXTERNAL_SCHEME"),
		authMode:       mode,
		username:       os.Getenv("WEBUI_USERNAME"),
		password:       password,
		upstreamToken:  upstreamToken,
		readToken:      readToken,
		writeToken:     writeToken,
		adminToken:     adminToken,
		insecureTest:   insecureTest,
		securityError:  securityError,
	}
}

func readOptionalSecretFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	return readSecretFile(path)
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
	secret := strings.TrimSpace(string(value))
	if secret == "" {
		return "", errors.New("secret file is empty")
	}
	return secret, nil
}

func webAuthentication(config handlerConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthRequest := r.URL.Path == "/api/web/health" && (r.Method == http.MethodGet || r.Method == http.MethodHead)
		if healthRequest || config.insecureTest {
			next.ServeHTTP(w, r)
			return
		}
		tokensConfigured := config.upstreamToken != "" || (config.readToken != "" && config.writeToken != "" && config.adminToken != "")
		configured := config.username != "" && config.password != "" && tokensConfigured && !config.securityError
		if !configured {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "服务安全配置不可用", "code": "SECURITY_CONFIG_UNAVAILABLE"})
			return
		}
		if config.authMode == authModeOff {
			next.ServeHTTP(w, r)
			return
		}
		username, password, ok := r.BasicAuth()
		valid := ok && constantTimeEqual(username, config.username) && constantTimeEqual(password, config.password)
		if !valid && config.authMode == authModeWarn {
			log.Print("Web UI 未认证请求")
			next.ServeHTTP(w, r)
			return
		}
		if !valid {
			w.Header().Set("WWW-Authenticate", `Basic realm="xiaohongshu-webui", charset="UTF-8"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "code": "UNAUTHORIZED"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func constantTimeEqual(provided, expected string) bool {
	providedHash := sha256.Sum256([]byte(provided))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1
}

func csrfProtection(externalScheme string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/web/") || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		fetchSite := r.Header.Get("Sec-Fetch-Site")
		// 状态请求必须携带同源 Origin；Fetch Metadata 存在时也必须明确为同源。
		valid := origin != "" && sameRequestOrigin(r, externalScheme)
		if fetchSite != "" && fetchSite != "same-origin" {
			valid = false
		}
		if !valid {
			http.Error(w, "forbidden request source", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameRequestOrigin(r *http.Request, externalScheme string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	originURL, err := url.Parse(origin)
	if err != nil || originURL.User != nil || originURL.RawQuery != "" || originURL.Fragment != "" || (originURL.Path != "" && originURL.Path != "/") {
		return false
	}
	scheme := externalScheme
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return originURL.Scheme == scheme && strings.EqualFold(originURL.Host, r.Host)
}
