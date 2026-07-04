package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func withFixtureQRCode(t *testing.T) {
	t.Helper()
	oldLookup, oldRun := execLookPath, runQRCode
	execLookPath = func(string) (string, error) { return "qrencode", nil }
	runQRCode = func(_ string, output, _ string) error { return os.WriteFile(output, []byte("png"), 0o600) }
	t.Cleanup(func() { execLookPath, runQRCode = oldLookup, oldRun })
}

func TestGoogleOAuthDisplayRoutesAreEphemeralAndNoStore(t *testing.T) {
	withFixtureQRCode(t)
	a := &app{home: t.TempDir()}
	paths := newOAuthDisplayPaths(a.oauthDisplayDir())
	if !googleOAuthDisplayOnly(paths, "display-state", "family", "https://accounts.example/authorize", time.Now()) {
		t.Fatal("display presentation was not armed")
	}

	metadata := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth-display", nil)
	request.RemoteAddr = "127.0.0.1:10001"
	a.handle(metadata, request)
	if metadata.Code != http.StatusOK || metadata.Header().Get("Cache-Control") != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("metadata status/cache=%d/%q", metadata.Code, metadata.Header().Get("Cache-Control"))
	}
	var got oauthDisplay
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

func TestGoogleOAuthDisplayExpiresAndNeverPublishesCallback(t *testing.T) {
	withFixtureQRCode(t)
	a := &app{home: t.TempDir()}
	paths := newOAuthDisplayPaths(a.oauthDisplayDir())
	if !googleOAuthDisplayOnly(paths, "display-state", "family", "https://accounts.example/authorize", time.Now()) {
		t.Fatal("display presentation was not armed")
	}
	old := time.Now().Add(-oauthDisplayTTL - time.Second)
	if err := os.Chtimes(paths.pending, old, old); err != nil {
		t.Fatal(err)
	}
	metadata := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth-display", nil)
	request.RemoteAddr = "127.0.0.1:10003"
	a.handle(metadata, request)
	if metadata.Code != http.StatusNotFound {
		t.Fatalf("expired display status=%d, want 404", metadata.Code)
	}
	for _, path := range []string{paths.pending, paths.display, paths.qr} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expired display artifact remains at %s: %v", path, err)
		}
	}

	callback := httptest.NewRecorder()
	a.handle(callback, httptest.NewRequest(http.MethodGet, "/oauth/google/callback?state=display-state&code=code", nil))
	if callback.Code != http.StatusNotFound {
		t.Fatalf("public callback status=%d, want 404", callback.Code)
	}
}
