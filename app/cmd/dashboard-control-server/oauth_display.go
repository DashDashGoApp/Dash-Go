package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// The dashboard-only QR presentation is short lived. It never creates a
	// public OAuth callback: phone setup always remains the explicit Desktop
	// client paste-back flow.
	oauthDisplayTTL       = 10 * time.Minute
	oauthDisplayWaitLimit = 5 * time.Minute
)

var errGoogleOAuthInterrupted = errors.New("Google authorization cancelled")

type oauthDisplayPaths struct {
	dir     string
	pending string
	display string
	qr      string
}

type oauthDisplay struct {
	Connection string `json:"connection"`
	URL        string `json:"url"`
	ExpiresAt  int64  `json:"expires_at"`
}

func newOAuthDisplayPaths(dir string) oauthDisplayPaths {
	return oauthDisplayPaths{
		dir:     filepath.Clean(dir),
		pending: filepath.Join(dir, "pending"),
		display: filepath.Join(dir, "display.json"),
		qr:      filepath.Join(dir, "qr.png"),
	}
}

func (a *app) oauthDisplayDir() string {
	if home := strings.TrimSpace(os.Getenv("DASH_VDIR_HOME")); home != "" {
		return filepath.Join(home, "oauth-display")
	}
	return filepath.Join(a.home, ".dashboard-vdirsyncer", "oauth-display")
}

func writeOAuthDisplayFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".oauth-display-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func cleanupOAuthDisplay(paths oauthDisplayPaths) {
	for _, path := range []string{paths.pending, paths.display, paths.qr} {
		_ = os.Remove(path)
	}
}

func oauthDisplayPending(paths oauthDisplayPaths, now time.Time) bool {
	info, err := os.Stat(paths.pending)
	if err != nil || info.IsDir() || now.After(info.ModTime().Add(oauthDisplayTTL)) {
		cleanupOAuthDisplay(paths)
		return false
	}
	return true
}

func oauthDisplayData(paths oauthDisplayPaths, now time.Time) (oauthDisplay, bool) {
	if !oauthDisplayPending(paths, now) {
		return oauthDisplay{}, false
	}
	contents, err := os.ReadFile(paths.display)
	if err != nil {
		return oauthDisplay{}, false
	}
	var display oauthDisplay
	if err := json.Unmarshal(contents, &display); err != nil || display.Connection == "" || display.URL == "" || display.ExpiresAt <= now.Unix() {
		cleanupOAuthDisplay(paths)
		return oauthDisplay{}, false
	}
	if _, err := os.Stat(paths.qr); err != nil {
		cleanupOAuthDisplay(paths)
		return oauthDisplay{}, false
	}
	return display, true
}

func (a *app) handleOAuthDisplayMetadata(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	display, active := oauthDisplayData(newOAuthDisplayPaths(a.oauthDisplayDir()), time.Now())
	if !active {
		http.NotFound(w, r)
		return
	}
	a.json(w, display)
}

func (a *app) handleOAuthDisplayQR(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	paths := newOAuthDisplayPaths(a.oauthDisplayDir())
	if _, active := oauthDisplayData(paths, time.Now()); !active {
		http.NotFound(w, r)
		return
	}
	contents, err := os.ReadFile(paths.qr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(contents)
}

// googleOAuthDisplayOnly arms an owner-local kiosk QR presentation for the
// phone paste-back route. The token exchange stays in the CLI; no HTTP OAuth
// callback is exposed by the running dashboard server.
func googleOAuthDisplayOnly(paths oauthDisplayPaths, state, connection, authorizationURL string, now time.Time) bool {
	if strings.TrimSpace(paths.dir) == "" || strings.TrimSpace(state) == "" {
		return false
	}
	if err := os.MkdirAll(paths.dir, 0o700); err != nil {
		return false
	}
	if err := os.Chmod(paths.dir, 0o700); err != nil {
		return false
	}
	cleanupOAuthDisplay(paths)
	if err := writeOAuthDisplayFile(paths.pending, []byte(strings.TrimSpace(state)+"\n")); err != nil {
		return false
	}
	if !googleOAuthCreateDisplay(paths, connection, authorizationURL, now) {
		cleanupOAuthDisplay(paths)
		return false
	}
	return true
}

func googleOAuthCreateDisplay(paths oauthDisplayPaths, connection, authorizationURL string, now time.Time) bool {
	if strings.TrimSpace(connection) == "" || strings.TrimSpace(authorizationURL) == "" {
		return false
	}
	qrencode, err := execLookPath("qrencode")
	if err != nil {
		return false
	}
	temporary, err := os.CreateTemp(paths.dir, ".oauth-qr-*.png")
	if err != nil {
		return false
	}
	temporaryName := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryName)
		return false
	}
	defer os.Remove(temporaryName)
	if err := runQRCode(qrencode, temporaryName, authorizationURL); err != nil {
		return false
	}
	if err := os.Chmod(temporaryName, 0o600); err != nil {
		return false
	}
	if err := os.Rename(temporaryName, paths.qr); err != nil {
		return false
	}
	display, err := json.Marshal(oauthDisplay{Connection: connection, URL: authorizationURL, ExpiresAt: now.Add(oauthDisplayWaitLimit).Unix()})
	if err != nil {
		_ = os.Remove(paths.qr)
		return false
	}
	if err := writeOAuthDisplayFile(paths.display, append(display, '\n')); err != nil {
		_ = os.Remove(paths.qr)
		return false
	}
	return true
}

// Small seams keep the QR and spool behavior directly testable without a
// system qrencode package.
var execLookPath = exec.LookPath
var runQRCode = func(binary, output, authorizationURL string) error {
	return exec.Command(binary, "-o", output, "-s", "8", "-m", "4", "-l", "L", authorizationURL).Run()
}
