package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	oauthRelayTTL       = 10 * time.Minute
	oauthRelayWaitLimit = 5 * time.Minute
)

var errGoogleOAuthInterrupted = errors.New("Google authorization cancelled")

type oauthRelayPaths struct {
	dir     string
	pending string
	result  string
	display string
	qr      string
}

type oauthRelayDisplay struct {
	Connection string `json:"connection"`
	URL        string `json:"url"`
	ExpiresAt  int64  `json:"expires_at"`
}

func newOAuthRelayPaths(dir string) oauthRelayPaths {
	return oauthRelayPaths{
		dir:     filepath.Clean(dir),
		pending: filepath.Join(dir, "pending"),
		result:  filepath.Join(dir, "result"),
		display: filepath.Join(dir, "display.json"),
		qr:      filepath.Join(dir, "qr.png"),
	}
}

func (a *app) oauthRelayDir() string {
	if home := strings.TrimSpace(os.Getenv("DASH_VDIR_HOME")); home != "" {
		return filepath.Join(home, "oauth-relay")
	}
	return filepath.Join(a.home, ".dashboard-vdirsyncer", "oauth-relay")
}

func writeOAuthRelayFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".oauth-relay-*")
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

func cleanupOAuthRelayPresentation(paths oauthRelayPaths) {
	// Do not remove result here. A successful callback removes pending before
	// the CLI's next poll, and the display poll may race that read. The CLI
	// owns result cleanup after it consumes (or times out waiting for) the code.
	for _, path := range []string{paths.pending, paths.display, paths.qr} {
		_ = os.Remove(path)
	}
}

func cleanupOAuthRelay(paths oauthRelayPaths) {
	cleanupOAuthRelayPresentation(paths)
	_ = os.Remove(paths.result)
}

func oauthRelayPendingState(paths oauthRelayPaths, now time.Time) (string, bool) {
	info, err := os.Stat(paths.pending)
	if err != nil || info.IsDir() || now.After(info.ModTime().Add(oauthRelayTTL)) {
		cleanupOAuthRelayPresentation(paths)
		return "", false
	}
	contents, err := os.ReadFile(paths.pending)
	if err != nil {
		return "", false
	}
	state := strings.TrimSpace(string(contents))
	return state, state != ""
}

func oauthRelayDisplayData(paths oauthRelayPaths, now time.Time) (oauthRelayDisplay, bool) {
	if _, active := oauthRelayPendingState(paths, now); !active {
		return oauthRelayDisplay{}, false
	}
	contents, err := os.ReadFile(paths.display)
	if err != nil {
		return oauthRelayDisplay{}, false
	}
	var display oauthRelayDisplay
	if err := json.Unmarshal(contents, &display); err != nil || display.Connection == "" || display.URL == "" || display.ExpiresAt <= now.Unix() {
		return oauthRelayDisplay{}, false
	}
	if _, err := os.Stat(paths.qr); err != nil {
		return oauthRelayDisplay{}, false
	}
	return display, true
}

// handleGoogleOAuthCallback is intentionally unauthenticated. The only route
// that reaches it is armed by the owner-local CLI with a random state, expires
// quickly, and accepts one matching code before returning to 404.
func (a *app) handleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	paths := newOAuthRelayPaths(a.oauthRelayDir())
	state, active := oauthRelayPendingState(paths, time.Now())
	if !active {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	receivedState := query.Get("state")
	code := strings.TrimSpace(query.Get("code"))
	if code == "" || receivedState == "" || subtle.ConstantTimeCompare([]byte(state), []byte(receivedState)) != 1 {
		http.NotFound(w, r)
		return
	}
	if err := writeOAuthRelayFile(paths.result, []byte(code+"\n")); err != nil {
		http.Error(w, "could not record authorization", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(paths.pending)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!doctype html><meta name=\"viewport\" content=\"width=device-width\"><title>Dash-Go connected</title><h2>Dash-Go: Google Calendar connected.</h2><p>You can close this tab and return to the terminal.</p>"))
}

func (a *app) handleOAuthDisplayMetadata(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	display, active := oauthRelayDisplayData(newOAuthRelayPaths(a.oauthRelayDir()), time.Now())
	if !active {
		http.NotFound(w, r)
		return
	}
	a.json(w, display)
}

func (a *app) handleOAuthDisplayQR(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	paths := newOAuthRelayPaths(a.oauthRelayDir())
	if _, active := oauthRelayDisplayData(paths, time.Now()); !active {
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

func googleOAuthCreateDisplay(paths oauthRelayPaths, connection, authorizationURL string, now time.Time) bool {
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
	display, err := json.Marshal(oauthRelayDisplay{Connection: connection, URL: authorizationURL, ExpiresAt: now.Add(oauthRelayWaitLimit).Unix()})
	if err != nil {
		_ = os.Remove(paths.qr)
		return false
	}
	if err := writeOAuthRelayFile(paths.display, append(display, '\n')); err != nil {
		_ = os.Remove(paths.qr)
		return false
	}
	return true
}

// Small seams keep the QR and spool behavior directly testable without a
// system qrencode package or an actual signal in the focused Go tests.
var execLookPath = exec.LookPath
var runQRCode = func(binary, output, authorizationURL string) error {
	return exec.Command(binary, "-o", output, "-s", "8", "-m", "4", "-l", "L", authorizationURL).Run()
}

func googleOAuthRelayWait(relayDir, state, connection, authorizationURL string, display bool) (string, error) {
	return googleOAuthRelayWaitWith(relayDir, state, connection, authorizationURL, display, oauthRelayWaitLimit, time.Second, nil)
}

func googleOAuthRelayWaitWith(relayDir, state, connection, authorizationURL string, display bool, waitLimit, pollEvery time.Duration, interrupt <-chan os.Signal) (string, error) {
	paths := newOAuthRelayPaths(relayDir)
	if err := os.MkdirAll(paths.dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(paths.dir, 0o700); err != nil {
		return "", err
	}
	cleanupOAuthRelay(paths)
	if err := writeOAuthRelayFile(paths.pending, []byte(strings.TrimSpace(state)+"\n")); err != nil {
		return "", err
	}
	defer cleanupOAuthRelay(paths)
	if display {
		_ = googleOAuthCreateDisplay(paths, connection, authorizationURL, time.Now())
	}
	if pollEvery <= 0 {
		pollEvery = time.Second
	}
	if waitLimit <= 0 || waitLimit > oauthRelayTTL {
		waitLimit = oauthRelayWaitLimit
	}
	if interrupt == nil {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signalChannel)
		interrupt = signalChannel
	}
	deadline := time.NewTimer(waitLimit)
	defer deadline.Stop()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		if result, err := os.ReadFile(paths.result); err == nil {
			code := strings.TrimSpace(string(result))
			if code != "" {
				return code, nil
			}
		}
		select {
		case <-interrupt:
			return "", errGoogleOAuthInterrupted
		case <-deadline.C:
			return "", fmt.Errorf("no web sign-in was received within %s", waitLimit.Round(time.Second))
		case <-ticker.C:
		}
	}
}
