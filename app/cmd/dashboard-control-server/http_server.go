package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
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
