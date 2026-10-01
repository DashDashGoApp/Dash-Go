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
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
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
		if !a.sameOriginAPIRequest(r) {
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
func (a *app) sameOriginAPIRequest(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		if len(a.requestSecurity.allowedOrigins) > 0 {
			if !a.requestSecurity.validOrigin(origin) {
				return false
			}
		} else {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				return false
			}
		}
	}
	site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	if r.Method == http.MethodPost {
		return site == "" || site == "same-origin"
	}
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

// mutableStaticRevision uses file size and nanosecond mtime for the two
// mutable browser resources that support conditional checks: live theme
// configuration and the generated event cache.
func mutableStaticRevision(root *os.Root, rel string) (string, bool) {
	st, err := root.Stat(rel)
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

func setMutableStaticRevision(w http.ResponseWriter, r *http.Request, root *os.Root, rel string) bool {
	conditionalGet := rel == "cache/events.cache.json"
	conditionalHead := conditionalGet || rel == "config/config.local.js"
	if !conditionalGet && !conditionalHead {
		return false
	}
	tag, ok := mutableStaticRevision(root, rel)
	if !ok {
		return false
	}
	w.Header().Set("ETag", tag)
	if ((r.Method == http.MethodGet && conditionalGet) || (r.Method == http.MethodHead && conditionalHead)) && requestHasETag(r.Header.Get("If-None-Match"), tag) {
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

func (a *app) dashboardContentSecurityPolicy() string {
	imageSources := []string{"'self'", "data:", "blob:", "https://tile.openstreetmap.org", "https://opengeo.ncep.noaa.gov", "https://rainviewer.com", "https://*.rainviewer.com"}
	settings := a.loadSettings()
	for _, key := range []string{"radarCustomTiles", "radarCustomWms"} {
		raw := strings.TrimSpace(jsonutil.StringValue(settings[key]))
		u, err := url.Parse(raw)
		if err != nil || !strings.EqualFold(u.Scheme, "https") || u.User != nil || u.Host == "" {
			continue
		}
		source := "https://" + strings.ToLower(u.Host)
		if !slices.Contains(imageSources, source) {
			imageSources = append(imageSources, source)
		}
	}
	slices.Sort(imageSources[3:])
	return "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src " + strings.Join(imageSources, " ") + "; connect-src 'self' https://api.rainviewer.com; frame-src https://maps.google.com; font-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"
}

func (a *app) dashboardSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Content-Security-Policy", a.dashboardContentSecurityPolicy())
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
	for _, segment := range strings.Split(requestPath, "/") {
		if segment == ".." || strings.ContainsRune(segment, '\x00') {
			return "", false
		}
	}
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
	if !valid || rel == "" || staticPrivatePath(rel) {
		setNoStore(w)
		http.NotFound(w, r)
		return
	}
	rootPath := a.dash
	rootRel := rel
	if _, ok := a.showcaseStaticDataPath(rel); ok {
		rootPath = a.showcase.dataRoot
		rootRel = rel
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		setNoStore(w)
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	file, err := root.Open(rootRel)
	if err != nil {
		setNoStore(w)
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil || st.IsDir() {
		setNoStore(w)
		http.NotFound(w, r)
		return
	}
	if a.currentVersionedUIAsset(r, rel, aliased) {
		setImmutableAssetCache(w)
	} else {
		setNoStore(w)
	}
	if setMutableStaticRevision(w, r, root, rootRel) {
		return
	}
	http.ServeContent(w, r, filepath.Base(rootRel), st.ModTime(), file)
}

func (a *app) httpServer(addr string) *http.Server {
	a.requestSecurity = newRequestSecurityPolicy(addr)
	base := a.httpRoutes()
	hostChecked := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.requestSecurity.validHost(r.Host) {
			http.Error(w, "misdirected request", http.StatusMisdirectedRequest)
			return
		}
		base.ServeHTTP(w, r)
	})
	handler := a.dashboardSecurityHeaders(hostChecked)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderLimit,
		ReadTimeout:       serverReadLimit,
		WriteTimeout:      serverWriteLimit,
		IdleTimeout:       serverIdleLimit,
		MaxHeaderBytes:    1 << 20,
	}
	// Release long-lived streams so a planned stop does not wait out the grace period.
	srv.RegisterOnShutdown(a.beginShutdown)
	return srv
}
