package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxJSONRequestBodyBytes = 64 * 1024
	immutableAssetMaxAge    = 31536000
	serverReadHeaderLimit   = 5 * time.Second
	serverReadLimit         = 15 * time.Second
	serverWriteLimit        = 45 * time.Second
	serverIdleLimit         = 60 * time.Second
)

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
}

func setImmutableAssetCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Del("Pragma")
}

// handle remains a small compatibility entry point for focused unit tests and
// internal callers. Real serving uses the same native ServeMux route table via
// httpServer; there is no second manual method-dispatch implementation.
func (a *app) handle(w http.ResponseWriter, r *http.Request) {
	a.httpRoutes().ServeHTTP(w, r)
}

func (a *app) requireLoopback(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		if !isLoopback(r) {
			a.err(w, "loopback only", http.StatusForbidden)
			return
		}
		if !sameOriginAPIRequest(r) {
			a.err(w, "same-origin API requests only", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (a *app) handleAPIGet(w http.ResponseWriter, r *http.Request) {
	a.handleGet(w, r, r.URL.Path)
}

func (a *app) handleAPIPost(w http.ResponseWriter, r *http.Request) {
	a.handlePost(w, r, r.URL.Path)
}

func (a *app) handleAPIRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIGet(w, r)
	case http.MethodPost:
		a.handleAPIPost(w, r)
	default:
		a.handleAPIMethodNotAllowed(w, r)
	}
}

func (a *app) handleAPIMethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	a.err(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (a *app) handleRuntimeFontRoute(w http.ResponseWriter, r *http.Request) {
	a.handleRuntimeFont(w, r, r.URL.Path)
}

func (a *app) handleStaticRoute(w http.ResponseWriter, r *http.Request) {
	a.static(w, r, r.URL.Path)
}

// httpRoutes uses Go 1.22 method-aware ServeMux patterns. The generic /api/
// handler intentionally preserves Dash-Go's JSON 405 and loopback-only policy
// for unsupported methods, rather than falling back to ServeMux's text 405.
func (a *app) httpRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	// The phone QR is presentation-only. Dash-Go does not publish an OAuth
	// callback endpoint; a phone user explicitly pastes the final address into
	// the owner-local setup terminal. The kiosk display remains loopback-only.
	mux.HandleFunc("GET /api/oauth-display", a.requireLoopback(a.handleOAuthDisplayMetadata))
	mux.HandleFunc("GET /api/oauth-display/qr.png", a.requireLoopback(a.handleOAuthDisplayQR))
	// ServeMux otherwise maps HEAD to GET patterns. Keep the API's explicit
	// JSON 405 contract for display endpoints as well.
	mux.HandleFunc("HEAD /api/oauth-display", a.requireLoopback(a.handleAPIMethodNotAllowed))
	mux.HandleFunc("HEAD /api/oauth-display/qr.png", a.requireLoopback(a.handleAPIMethodNotAllowed))
	// The generic API path keeps Dash-Go's JSON 405 behavior without a method
	// pattern that conflicts with the two narrower GET-only display endpoints.
	mux.HandleFunc("/api/", a.requireLoopback(a.handleAPIRoute))
	// The old prefix check treated bare /api as a static path. Keep that exact
	// boundary instead of allowing ServeMux's subtree redirect to change it.
	mux.HandleFunc("/api", a.handleStaticRoute)
	mux.HandleFunc("/fonts/", a.handleRuntimeFontRoute)
	mux.HandleFunc("/", a.handleStaticRoute)
	return mux
}

// sameOriginAPIRequest blocks browser cross-origin requests to the local
// control API. Headerless local tools remain supported; a supplied Origin or
// Sec-Fetch-Site header must describe the same dashboard origin.
func sameOriginAPIRequest(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return false
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if !strings.EqualFold(u.Scheme, scheme) || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	return site == "" || site == "same-origin" || site == "none"
}

func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *app) json(w http.ResponseWriter, v any, code ...int) {
	c := 200
	if len(code) > 0 {
		c = code[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *app) err(w http.ResponseWriter, msg string, code int) {
	a.json(w, map[string]any{"error": msg}, code)
}

func (a *app) readBody(r *http.Request) (map[string]any, error) {
	// Reject an announced oversized body before allocating or reading it. A
	// chunked/unknown-length body still uses the bounded reader below.
	if r != nil && r.ContentLength > maxJSONRequestBodyBytes {
		return nil, errRequestBodyTooLarge
	}
	if r == nil || r.Body == nil {
		return map[string]any{}, nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxJSONRequestBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxJSONRequestBodyBytes {
		return nil, errRequestBodyTooLarge
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if err := validateJSONRequestFields(m); err != nil {
		return nil, err
	}
	return m, nil
}

// configLocalRevision is intentionally limited to the one mutable file the
// browser polls for live theme changes. It avoids a body download on unchanged
// minute checks without making general user configuration cacheable.
func configLocalRevision(path string) (string, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return "", false
	}
	return fmt.Sprintf(`W/"%x-%x"`, st.Size(), st.ModTime().UnixNano()), true
}

func requestHasETag(header, want string) bool {
	for value := range strings.SplitSeq(header, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || value == want {
			return true
		}
	}
	return false
}

func setConfigLocalRevision(w http.ResponseWriter, r *http.Request, rel, full string) bool {
	if rel != "config/config.local.js" {
		return false
	}
	tag, ok := configLocalRevision(full)
	if !ok {
		return false
	}
	w.Header().Set("ETag", tag)
	if r.Method == http.MethodHead && requestHasETag(r.Header.Get("If-None-Match"), tag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

// currentVersionedUIAsset only caches app-owned CSS/JS whose URL contains the
// exact installed release version. HTML, configuration, legacy aliases, and
// APIs remain no-store so browser relaunches cannot retain mutable state.
func (a *app) currentVersionedUIAsset(r *http.Request, rel string, aliased bool) bool {
	if aliased || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	if !strings.HasPrefix(rel, "ui/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(rel))
	if ext != ".css" && ext != ".js" {
		return false
	}
	version := strings.TrimSpace(a.releaseVersion)
	return version != "" && r.URL.Query().Get("v") == version
}

// staticPrivatePath keeps legacy mutable Family Message Board data out of the
// browser file surface during and after migration to the home-side private
// store. Other expected browser bootstrap/config paths remain unchanged.
func staticPrivatePath(rel string) bool {
	return rel == "config/family-board.json" ||
		rel == "config/notification-preferences.json" ||
		// Older builds wrote support bundles under cache/. Keep that exact legacy
		// location private even when an update preserves the old file on disk.
		rel == "cache/dashboard-diagnostics.zip"
}

func dashboardSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}

// staticURLRelativePath normalizes an HTTP request path independently of the
// host filesystem. URLs always use forward slashes; filepath.Clean converts
// those slashes to backslashes in a Windows build, which can make an
// allowlisted Showcase data route look like a different path. Normalizing here
// keeps the static allowlist, cache policy, and immutable app-root join on one
// stable URL-path representation.
func staticURLRelativePath(requestPath string) (string, bool) {
	requestPath = strings.ReplaceAll(requestPath, `\`, "/")
	clean := path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	rel := strings.TrimPrefix(clean, "/")
	// Windows cannot represent a colon in a file name. Refuse a drive-qualified
	// or otherwise ambiguous URL segment before converting it to a filesystem
	// path, even though normal browser requests never need one.
	if strings.Contains(rel, ":") {
		return "", false
	}
	return rel, true
}

func (a *app) static(w http.ResponseWriter, r *http.Request, requestPath string) {
	if requestPath == "/" || requestPath == "" {
		requestPath = "/index.html"
	}
	aliased := true
	switch requestPath {
	case "/dashboard.css":
		requestPath = "/ui/dashboard.css"
	case "/control-layout.css":
		requestPath = "/ui/control-layout.css"
	case "/dashboard.js":
		requestPath = "/ui/js/app.bundle.js"
	default:
		aliased = false
	}
	rel, valid := staticURLRelativePath(requestPath)
	if !valid {
		setNoStore(w)
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(a.dash, filepath.FromSlash(rel))
	dataPath, fromShowcaseData := a.showcaseStaticDataPath(rel)
	if fromShowcaseData {
		full = dataPath
	}
	if (!fromShowcaseData && !strings.HasPrefix(full, a.dash)) || staticPrivatePath(rel) {
		setNoStore(w)
		http.NotFound(w, r)
		return
	}
	if a.currentVersionedUIAsset(r, rel, aliased) {
		setImmutableAssetCache(w)
	} else {
		setNoStore(w)
	}
	if setConfigLocalRevision(w, r, rel, full) {
		return
	}
	// ServeFile performs the file/dir check itself.  Avoid a second stat on
	// every static request; repeated Surf relaunches commonly request these
	// same small versioned assets together.
	http.ServeFile(w, r, full)
}

func (a *app) httpServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           dashboardSecurityHeaders(a.httpRoutes()),
		ReadHeaderTimeout: serverReadHeaderLimit,
		ReadTimeout:       serverReadLimit,
		WriteTimeout:      serverWriteLimit,
		IdleTimeout:       serverIdleLimit,
		MaxHeaderBytes:    1 << 20,
	}
}
