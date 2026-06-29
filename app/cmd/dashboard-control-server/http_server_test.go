package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeStaticTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestStaticCachePolicy(t *testing.T) {
	root := t.TempDir()
	writeStaticTestFile(t, root, "index.html", "<!doctype html>")
	writeStaticTestFile(t, root, "config/config.local.js", "window.DASHBOARD_LOCAL={};")
	writeStaticTestFile(t, root, "config/family-board.json", `{"notes":[{"text":"private"}]}`)
	writeStaticTestFile(t, root, "ui/dashboard.css", "body{}")
	writeStaticTestFile(t, root, "ui/control-layout.css", ".ctrl{}")
	writeStaticTestFile(t, root, "ui/js/app.bundle.js", "window.app=true;")
	a := &app{dash: root, releaseVersion: "1.4.1-beta.11"}
	cases := []struct {
		name, target, wantCache string
		wantPragma              string
	}{
		{"versioned dashboard css", "/ui/dashboard.css?v=1.4.1-beta.11", "public, max-age=31536000, immutable", ""},
		{"versioned control css", "/ui/control-layout.css?v=1.4.1-beta.11", "public, max-age=31536000, immutable", ""},
		{"versioned bundle", "/ui/js/app.bundle.js?v=1.4.1-beta.11", "public, max-age=31536000, immutable", ""},
		{"wrong version", "/ui/dashboard.css?v=1.4.1-beta.3", "no-store, no-cache, must-revalidate, max-age=0", "no-cache"},
		{"unversioned css", "/ui/dashboard.css", "no-store, no-cache, must-revalidate, max-age=0", "no-cache"},
		{"legacy alias", "/dashboard.css?v=1.4.1-beta.11", "no-store, no-cache, must-revalidate, max-age=0", "no-cache"},
		{"html", "/", "no-store, no-cache, must-revalidate, max-age=0", "no-cache"},
		{"config", "/config/config.local.js?v=1.4.1-beta.11", "no-store, no-cache, must-revalidate, max-age=0", "no-cache"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			a.handle(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d", w.Code)
			}
			if got := w.Header().Get("Cache-Control"); got != tc.wantCache {
				t.Fatalf("Cache-Control=%q want %q", got, tc.wantCache)
			}
			if got := w.Header().Get("Pragma"); got != tc.wantPragma {
				t.Fatalf("Pragma=%q want %q", got, tc.wantPragma)
			}
		})
	}

	private := httptest.NewRecorder()
	a.handle(private, httptest.NewRequest(http.MethodGet, "/config/family-board.json", nil))
	if private.Code != http.StatusNotFound {
		t.Fatalf("private legacy board status=%d want 404", private.Code)
	}
}

func TestHTTPServerTimeouts(t *testing.T) {
	a := &app{}
	s := a.httpServer("127.0.0.1:8090")
	if s.ReadHeaderTimeout != serverReadHeaderLimit || s.ReadTimeout != serverReadLimit || s.WriteTimeout != serverWriteLimit || s.IdleTimeout != serverIdleLimit {
		t.Fatalf("unexpected timeouts: %#v", s)
	}
	if s.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes=%d", s.MaxHeaderBytes)
	}
}

func TestConfigLocalRevisionSupportsConditionalHead(t *testing.T) {
	root := t.TempDir()
	writeStaticTestFile(t, root, "config/config.local.js", `window.DASHBOARD_LOCAL={theme:"paper"};`)
	a := &app{dash: root, releaseVersion: "1.4.1-beta.11"}

	first := httptest.NewRecorder()
	a.handle(first, httptest.NewRequest(http.MethodGet, "/config/config.local.js", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial config status=%d", first.Code)
	}
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("config response did not include a revision ETag")
	}
	if got := first.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("config cache policy=%q", got)
	}

	conditional := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/config/config.local.js", nil)
	req.Header.Set("If-None-Match", tag)
	a.handle(conditional, req)
	if conditional.Code != http.StatusNotModified {
		t.Fatalf("conditional HEAD status=%d want 304", conditional.Code)
	}
	if got := conditional.Header().Get("ETag"); got != tag {
		t.Fatalf("conditional ETag=%q want %q", got, tag)
	}
}

func TestHTTPServerAppliesBaselineSecurityHeadersToAllResponses(t *testing.T) {
	root := t.TempDir()
	writeStaticTestFile(t, root, "index.html", "<!doctype html>")
	writeStaticTestFile(t, root, "ui/js/app.js", "window.app=true;")
	a := &app{dash: root, releaseVersion: "1.4.3-beta.93"}
	s := a.httpServer("127.0.0.1:8090")
	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
	}
	for _, target := range []string{"/", "/ui/js/app.js", "/api/not-found", "/missing"} {
		t.Run(target, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, target, nil)
			r.RemoteAddr = "127.0.0.1:12345"
			w := httptest.NewRecorder()
			s.Handler.ServeHTTP(w, r)
			for header, want := range expected {
				if got := w.Header().Get(header); got != want {
					t.Fatalf("%s=%q want %q", header, got, want)
				}
			}
		})
	}
}
