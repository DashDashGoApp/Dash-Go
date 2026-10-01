package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

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
