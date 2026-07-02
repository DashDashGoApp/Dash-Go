package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	settingspkg "github.com/DashDashGoApp/Dash-Go/app/internal/settings"
)

// Runtime font metadata and verification now live in internal/settings. These
// aliases preserve focused main-package tests while keeping production state
// and download behavior in the extracted bounded context.
const runtimeFontSourceCommit = settingspkg.RuntimeFontSourceCommit

type runtimeFontAsset = settingspkg.RuntimeFontAsset
type runtimeFontSpec = settingspkg.RuntimeFontSpec

var runtimeFontSpecs = settingspkg.RuntimeFontSpecs()

func (a *app) fontStatus() map[string]any         { return a.settingsService().FontStatus() }
func (a *app) runtimeFontState(key string) string { return a.settingsService().RuntimeFontState(key) }
func (a *app) fontStatusPayload() map[string]any  { return a.settingsService().FontStatusPayload() }

func (a *app) handleFontGet(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/fonts/status":
		a.json(w, map[string]any{"fonts": a.fontStatusPayload()})
		return true
	case "/api/fonts/face.css":
		// This stylesheet is render-blocking in index.html and its contents
		// change only when the household changes fonts. no-store forced every
		// kiosk boot and Surf relaunch to refetch it before first paint;
		// revalidation keeps font changes instant (each request still asks the
		// server) while letting an unchanged answer be one 304 round trip.
		css := a.settingsService().FontFaceCSS()
		sum := sha256.Sum256([]byte(css))
		tag := `W/"fontcss-` + hex.EncodeToString(sum[:8]) + `"`
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", tag)
		if requestHasETag(r.Header.Get("If-None-Match"), tag) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
		_, _ = w.Write([]byte(css))
		return true
	}
	return false
}

func (a *app) handleRuntimeFont(w http.ResponseWriter, r *http.Request, path string) bool {
	const prefix = "/fonts/"
	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return true
	}
	name := strings.TrimPrefix(path, prefix)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.NotFound(w, r)
		return true
	}
	font, info, publicName, ok := a.settingsService().OpenRuntimeFont(name)
	if !ok {
		http.NotFound(w, r)
		return true
	}
	defer font.Close()
	// no-cache (revalidate every use), not no-store: ServeContent already
	// sends Last-Modified and honors If-Modified-Since, so an unchanged font
	// file becomes a 304 instead of a full TTF download and re-parse on every
	// kiosk boot. A replaced font still shows up immediately.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, publicName, info.ModTime(), font)
	return true
}

func (a *app) handleFontPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if path != "/api/fonts/download" {
		return false
	}
	if !a.tokenOK(r.Header.Get("X-Dashboard-Token")) {
		a.err(w, "locked", http.StatusLocked)
		return true
	}
	key := jsonutil.BodyString(body, "key")
	spec, ok := runtimeFontSpecs[key]
	if !ok {
		a.err(w, "font is not downloadable", http.StatusBadRequest)
		return true
	}
	if err := a.downloadRuntimeFont(spec); err != nil {
		a.err(w, "font download failed: "+err.Error(), http.StatusBadGateway)
		return true
	}
	a.invalidateSystemStatus()
	a.json(w, map[string]any{"fonts": a.fontStatusPayload()})
	return true
}

func fontLooksValid(path string) bool          { return settingspkg.FontLooksValid(path) }
func validRuntimeFontSHA256(value string) bool { return settingspkg.ValidRuntimeFontSHA256(value) }

func runtimeFontAssetValid(path string, asset runtimeFontAsset) bool {
	return settingspkg.RuntimeFontAssetValid(path, asset)
}

func (a *app) downloadRuntimeFont(spec runtimeFontSpec) error {
	return a.downloadRuntimeFontWithClient(spec, &http.Client{Timeout: 22 * time.Second})
}
func (a *app) downloadRuntimeFontWithClient(spec runtimeFontSpec, client *http.Client) error {
	return a.settingsService().DownloadRuntimeFontWithClient(spec, client)
}
