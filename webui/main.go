package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddress = ":18080"
	defaultUpstreamURL   = "http://localhost:18060"
	maxProxyRequestBody  = 1 << 20
)

//go:embed static/*
var staticFiles embed.FS

type handlerConfig struct {
	upstreamURL    string
	externalScheme string
	authMode       authMode
	username       string
	password       string
	upstreamToken  string
	readToken      string
	writeToken     string
	adminToken     string
	insecureTest   bool
	securityError  bool
}

type routeRule struct {
	methods map[string]struct{}
	scope   accessScope
	mapPath func([]string) (string, bool)
}

type accessScope string

const (
	scopeRead  accessScope = "read"
	scopeWrite accessScope = "write"
	scopeAdmin accessScope = "admin"
)

var proxyRoutes = []routeRule{
	newRoute([]string{http.MethodGet}, scopeRead, exactMap("/api/v1/accounts", "accounts")),
	newRoute([]string{http.MethodPost}, scopeAdmin, exactMap("/api/v1/accounts", "accounts")),
	newRoute([]string{http.MethodPost}, scopeAdmin, exactMap("/api/v1/accounts/quick_add", "accounts", "quick_add")),
	newRoute([]string{http.MethodDelete}, scopeAdmin, accountRoute()),
	newRoute([]string{http.MethodPut}, scopeAdmin, accountAction("default")),
	newRoute([]string{http.MethodPost}, scopeAdmin, accountAction("login", "qrcode")),
	newRoute([]string{http.MethodPost}, scopeRead, accountAction("login", "status")),
	newRoute([]string{http.MethodPost}, scopeAdmin, accountAction("sync_profile")),
	newRoute([]string{http.MethodDelete}, scopeAdmin, accountAction("login")),
	newRoute([]string{http.MethodGet}, scopeRead, exactMap("/api/v1/login/status", "login", "status")),
	newRoute([]string{http.MethodGet}, scopeAdmin, exactMap("/api/v1/login/qrcode", "login", "qrcode")),
	newRoute([]string{http.MethodDelete}, scopeAdmin, exactMap("/api/v1/login/cookies", "login", "cookies")),
	newRoute([]string{http.MethodPost}, scopeWrite, exactMap("/api/v1/publish", "publish")),
	newRoute([]string{http.MethodPost}, scopeWrite, exactMap("/api/v1/publish_video", "publish_video")),
	newRoute([]string{http.MethodGet}, scopeRead, exactMap("/api/v1/feeds/list", "feeds", "list")),
	newRoute([]string{http.MethodGet, http.MethodPost}, scopeRead, exactMap("/api/v1/feeds/search", "feeds", "search")),
	newRoute([]string{http.MethodPost}, scopeRead, exactMap("/api/v1/feeds/detail", "feeds", "detail")),
	newRoute([]string{http.MethodPost}, scopeWrite, exactMap("/api/v1/feeds/comment", "feeds", "comment")),
	newRoute([]string{http.MethodPost}, scopeWrite, exactMap("/api/v1/feeds/comment/reply", "feeds", "comment", "reply")),
	newRoute([]string{http.MethodPost}, scopeWrite, exactMap("/api/v1/feeds/like", "feeds", "like")),
	newRoute([]string{http.MethodPost}, scopeWrite, exactMap("/api/v1/feeds/favorite", "feeds", "favorite")),
	newRoute([]string{http.MethodPost}, scopeRead, exactMap("/api/v1/user/profile", "user", "profile")),
	newRoute([]string{http.MethodGet}, scopeRead, exactMap("/api/v1/user/me", "user", "me")),
}

func main() {
	listenAddress := envOrDefault("WEBUI_ADDR", defaultListenAddress)
	server := &http.Server{
		Addr:              listenAddress,
		Handler:           newHandler(loadHandlerConfig()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	log.Printf("Web UI 服务监听 %s", listenAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newHandler(config handlerConfig) http.Handler {
	if config.authMode == "" {
		config.authMode = authModeOff
	}
	if config.externalScheme != "" && config.externalScheme != "http" && config.externalScheme != "https" {
		panic("WEBUI_EXTERNAL_SCHEME 仅支持 http 或 https")
	}
	upstream := config.upstreamURL
	if upstream == "" {
		upstream = defaultUpstreamURL
	}
	upstreamURL, err := url.Parse(upstream)
	if err != nil || upstreamURL.Scheme == "" || upstreamURL.Host == "" {
		panic("无效的上游地址: " + upstream)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": "上游服务不可用",
			"code":  "UPSTREAM_UNAVAILABLE",
		})
		log.Printf("Web UI 代理失败: %v", err)
	}
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = upstreamURL.Host
		r.Header.Del("Authorization")
		r.Header.Del("Proxy-Authorization")
		r.Header.Del("X-XHS-Authenticated-Actor")
		r.Header.Del("X-XHS-Authenticated-Scopes")
		if token := config.tokenForPath(r.Method, r.URL.Path); token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		response.Header.Del("Authorization")
		response.Header.Del("Proxy-Authenticate")
		return nil
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
	pageHandler := func(file string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, staticFS, file)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/web/health", healthHandler)
	mux.HandleFunc("/api/web/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		path, allowed := proxyPath(r.Method, r.URL.EscapedPath())
		if !allowed {
			http.NotFound(w, r)
			return
		}
		if !limitProxyRequestBody(w, r) {
			return
		}
		r.URL.Path = path
		r.URL.RawPath = ""
		proxy.ServeHTTP(w, r)
	})
	mux.Handle("GET /static/", staticHandler)
	mux.HandleFunc("GET /{$}", pageHandler("index.html"))
	mux.HandleFunc("GET /search.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/discover.html", http.StatusTemporaryRedirect)
	})
	for _, page := range []string{"accounts.html", "discover.html", "publish.html", "detail.html", "profile.html"} {
		mux.HandleFunc("GET /"+page, pageHandler(page))
	}

	secured := securityHeaders(mux)
	if !config.insecureTest {
		secured = csrfProtection(config.externalScheme, secured)
	}
	secured = sameOriginOnly(config.externalScheme, secured)
	secured = webAuthentication(config, secured)
	return rejectUnsafePaths(secured)
}

func limitProxyRequestBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxProxyRequestBody+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "读取请求体失败", "code": "INVALID_REQUEST"})
		return false
	}
	if len(body) > maxProxyRequestBody {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "请求体超过 1 MiB 限制", "code": "REQUEST_TOO_LARGE"})
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return true
}

func rejectUnsafePaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath := strings.ToLower(r.URL.EscapedPath())
		if strings.Contains(escapedPath, "..") || strings.Contains(escapedPath, "%2f") || strings.ContainsAny(escapedPath, "\\\x00") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "xiaohongshu-mcp-webui",
	})
}

func proxyPath(method, escapedPath string) (string, bool) {
	const prefix = "/api/web/"
	if !strings.HasPrefix(escapedPath, prefix) || strings.Contains(strings.ToLower(escapedPath), "%2f") {
		return "", false
	}
	relative := strings.TrimPrefix(escapedPath, prefix)
	if relative == "" || strings.Contains(relative, "..") || strings.ContainsAny(relative, "\\\x00") {
		return "", false
	}
	segments := strings.Split(relative, "/")
	for _, segment := range segments {
		if segment == "" {
			return "", false
		}
	}
	for _, rule := range proxyRoutes {
		if _, ok := rule.methods[method]; ok {
			if path, matched := rule.mapPath(segments); matched {
				return path, true
			}
		}
	}
	return "", false
}

func newRoute(methods []string, scope accessScope, mapPath func([]string) (string, bool)) routeRule {
	allowed := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		allowed[method] = struct{}{}
	}
	return routeRule{methods: allowed, scope: scope, mapPath: mapPath}
}

func (config handlerConfig) tokenForPath(method, path string) string {
	for _, rule := range proxyRoutes {
		if _, ok := rule.methods[method]; !ok {
			continue
		}
		if mapped, matched := rule.mapPath(strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")); matched && mapped == path {
			switch rule.scope {
			case scopeRead:
				return firstNonEmpty(config.readToken, config.upstreamToken)
			case scopeWrite:
				return firstNonEmpty(config.writeToken, config.upstreamToken)
			case scopeAdmin:
				return firstNonEmpty(config.adminToken, config.upstreamToken)
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func exactMap(upstreamPath string, want ...string) func([]string) (string, bool) {
	return func(got []string) (string, bool) {
		if len(got) != len(want) {
			return "", false
		}
		for i := range want {
			if got[i] != want[i] {
				return "", false
			}
		}
		return upstreamPath, true
	}
}

func accountRoute() func([]string) (string, bool) {
	return func(segments []string) (string, bool) {
		if len(segments) != 2 || segments[0] != "accounts" || !validAccountID(segments[1]) {
			return "", false
		}
		return "/api/v1/accounts/" + segments[1], true
	}
}

func accountAction(action ...string) func([]string) (string, bool) {
	return func(segments []string) (string, bool) {
		if len(segments) != len(action)+2 || segments[0] != "accounts" || !validAccountID(segments[1]) {
			return "", false
		}
		for i := range action {
			if segments[i+2] != action[i] {
				return "", false
			}
		}
		return "/api/v1/" + strings.Join(segments, "/"), true
	}
}

func validAccountID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func sameOriginOnly(externalScheme string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !sameRequestOrigin(r, externalScheme) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https:; media-src 'self' https:; object-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("响应序列化失败: %v", err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
