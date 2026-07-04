package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func armOAuthRelay(t *testing.T, paths oauthRelayPaths, state string) {
	t.Helper()
	if err := os.MkdirAll(paths.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeOAuthRelayFile(paths.pending, []byte(state+"\n")); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleOAuthRelayCallbackIsOneShot(t *testing.T) {
	a := &app{home: t.TempDir()}
	paths := newOAuthRelayPaths(a.oauthRelayDir())
	armOAuthRelay(t, paths, "good-state")

	request := httptest.NewRequest(http.MethodGet, "/oauth/google/callback?state=good-state&code=auth-code", nil)
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("callback status=%d, want 200", response.Code)
	}
	result, err := os.ReadFile(paths.result)
	if err != nil || strings.TrimSpace(string(result)) != "auth-code" {
		t.Fatalf("callback result=%q err=%v", result, err)
	}
	if _, err := os.Stat(paths.pending); !os.IsNotExist(err) {
		t.Fatalf("pending relay survived success: %v", err)
	}

	replay := httptest.NewRecorder()
	a.handle(replay, request)
	if replay.Code != http.StatusNotFound {
		t.Fatalf("replayed callback status=%d, want 404", replay.Code)
	}
}

func TestGoogleOAuthDisplayPollDoesNotConsumeCallbackResult(t *testing.T) {
	a := &app{home: t.TempDir()}
	paths := newOAuthRelayPaths(a.oauthRelayDir())
	armOAuthRelay(t, paths, "good-state")
	display := oauthRelayDisplay{Connection: "family", URL: "https://accounts.google.com/o/oauth2/v2/auth?state=good-state", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	body, err := json.Marshal(display)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeOAuthRelayFile(paths.display, body); err != nil {
		t.Fatal(err)
	}
	if err := writeOAuthRelayFile(paths.qr, []byte("png")); err != nil {
		t.Fatal(err)
	}

	callback := httptest.NewRecorder()
	a.handle(callback, httptest.NewRequest(http.MethodGet, "/oauth/google/callback?state=good-state&code=auth-code", nil))
	if callback.Code != http.StatusOK {
		t.Fatalf("callback status=%d, want 200", callback.Code)
	}

	// A dashboard poll commonly happens immediately after success. It must
	// hide the display without deleting the code before the CLI reads it.
	poll := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth-display", nil)
	request.RemoteAddr = "127.0.0.1:10003"
	a.handle(poll, request)
	if poll.Code != http.StatusNotFound {
		t.Fatalf("post-callback display poll=%d, want 404", poll.Code)
	}
	result, err := os.ReadFile(paths.result)
	if err != nil || strings.TrimSpace(string(result)) != "auth-code" {
		t.Fatalf("post-poll callback result=%q err=%v", result, err)
	}
}

func TestGoogleOAuthRelayRejectsUnarmedStaleAndWrongState(t *testing.T) {
	a := &app{home: t.TempDir()}
	paths := newOAuthRelayPaths(a.oauthRelayDir())
	call := func(target string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.handle(w, httptest.NewRequest(http.MethodGet, target, nil))
		return w
	}
	if got := call("/oauth/google/callback?state=x&code=y").Code; got != http.StatusNotFound {
		t.Fatalf("unarmed status=%d, want 404", got)
	}
	armOAuthRelay(t, paths, "expected")
	if got := call("/oauth/google/callback?state=other&code=y").Code; got != http.StatusNotFound {
		t.Fatalf("wrong-state status=%d, want 404", got)
	}
	if _, err := os.Stat(paths.pending); err != nil {
		t.Fatalf("wrong state disarmed relay: %v", err)
	}
	old := time.Now().Add(-oauthRelayTTL - time.Second)
	if err := os.Chtimes(paths.pending, old, old); err != nil {
		t.Fatal(err)
	}
	if got := call("/oauth/google/callback?state=expected&code=y").Code; got != http.StatusNotFound {
		t.Fatalf("stale status=%d, want 404", got)
	}
	if _, err := os.Stat(paths.pending); !os.IsNotExist(err) {
		t.Fatalf("stale pending relay survived: %v", err)
	}
}

func TestGoogleOAuthDisplayRoutesAreEphemeralAndNoStore(t *testing.T) {
	a := &app{home: t.TempDir()}
	paths := newOAuthRelayPaths(a.oauthRelayDir())
	armOAuthRelay(t, paths, "display-state")
	display := oauthRelayDisplay{Connection: "family", URL: "https://accounts.google.com/o/oauth2/v2/auth?state=display-state", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	body, err := json.Marshal(display)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeOAuthRelayFile(paths.display, body); err != nil {
		t.Fatal(err)
	}
	if err := writeOAuthRelayFile(paths.qr, []byte("png")); err != nil {
		t.Fatal(err)
	}

	metadata := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth-display", nil)
	request.RemoteAddr = "127.0.0.1:10001"
	a.handle(metadata, request)
	if metadata.Code != http.StatusOK || metadata.Header().Get("Cache-Control") != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("metadata status/cache=%d/%q", metadata.Code, metadata.Header().Get("Cache-Control"))
	}
	var got oauthRelayDisplay
	if err := json.Unmarshal(metadata.Body.Bytes(), &got); err != nil || got.Connection != "family" {
		t.Fatalf("metadata body=%q err=%v", metadata.Body.String(), err)
	}

	qr := httptest.NewRecorder()
	qrRequest := httptest.NewRequest(http.MethodGet, "/api/oauth-display/qr.png", nil)
	qrRequest.RemoteAddr = "127.0.0.1:10002"
	a.handle(qr, qrRequest)
	if qr.Code != http.StatusOK || qr.Header().Get("Content-Type") != "image/png" || qr.Header().Get("Cache-Control") != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("QR status/type/cache=%d/%q/%q", qr.Code, qr.Header().Get("Content-Type"), qr.Header().Get("Cache-Control"))
	}

	if err := os.Remove(paths.pending); err != nil {
		t.Fatal(err)
	}
	gone := httptest.NewRecorder()
	a.handle(gone, request)
	if gone.Code != http.StatusNotFound {
		t.Fatalf("unarmed display status=%d, want 404", gone.Code)
	}
	for _, path := range []string{paths.display, paths.qr} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("display artifact remains at %s: %v", path, err)
		}
	}
}

func TestGoogleOAuthRelayWaitReturnsCodeAndCleansSpool(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "oauth-relay")
	paths := newOAuthRelayPaths(dir)
	done := make(chan struct {
		code string
		err  error
	}, 1)
	go func() {
		code, err := googleOAuthRelayWaitWith(dir, "state", "family", "https://accounts.example/authorize", false, time.Second, time.Millisecond, make(chan os.Signal))
		done <- struct {
			code string
			err  error
		}{code, err}
	}()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(paths.pending); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("relay never armed")
		}
		time.Sleep(time.Millisecond)
	}
	if err := writeOAuthRelayFile(paths.result, []byte("returned-code\n")); err != nil {
		t.Fatal(err)
	}
	out := <-done
	if out.err != nil || out.code != "returned-code" {
		t.Fatalf("wait result=%q err=%v", out.code, out.err)
	}
	for _, path := range []string{paths.pending, paths.result, paths.display, paths.qr} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("relay artifact remains at %s: %v", path, err)
		}
	}
}

func TestGoogleOAuthRelayWaitTimeoutCleansSpool(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "oauth-relay")
	paths := newOAuthRelayPaths(dir)
	_, err := googleOAuthRelayWaitWith(dir, "state", "family", "https://accounts.example/authorize", false, 10*time.Millisecond, time.Millisecond, make(chan os.Signal))
	if err == nil || !strings.Contains(err.Error(), "no web sign-in") {
		t.Fatalf("timeout err=%v", err)
	}
	for _, path := range []string{paths.pending, paths.result, paths.display, paths.qr} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("relay artifact remains at %s: %v", path, statErr)
		}
	}
}
